package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

// Bot представляет основную структуру бота
type Bot struct {
	api *tgbotapi.BotAPI
	db  *sql.DB
}

// NewBot создает новый экземпляр бота
func NewBot(token string, dbPath string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания бота: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия базы данных: %v", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("ошибка соединения с базой данных: %v", err)
	}

	return &Bot{
		api: api,
		db:  db,
	}, nil
}

// Close закрывает соединения
func (b *Bot) Close() {
	if b.db != nil {
		b.db.Close()
	}
}

// getBirthdaysForToday возвращает список дней рождения на сегодня
func (b *Bot) getBirthdaysForToday() ([]string, error) {
	today := time.Now().Format("01-02")
	rows, err := b.db.Query(`
		SELECT first_name, last_name 
		FROM birthdays 
		WHERE strftime('%m-%d', birthday) = ?
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []string
	for rows.Next() {
		var firstName, lastName string
		if err := rows.Scan(&firstName, &lastName); err != nil {
			return nil, err
		}
		birthdays = append(birthdays, fmt.Sprintf("%s %s", firstName, lastName))
	}
	return birthdays, nil
}

// sendBirthdayNotifications отправляет уведомления о днях рождения
func (b *Bot) sendBirthdayNotifications() {
	birthdays, err := b.getBirthdaysForToday()
	if err != nil {
		log.Printf("Ошибка получения дней рождения: %v", err)
		return
	}

	if len(birthdays) == 0 {
		return
	}

	message := "Сегодня день рождения у:\n"
	for _, name := range birthdays {
		message += fmt.Sprintf("- %s\n", name)
	}

	rows, err := b.db.Query("SELECT id FROM groups")
	if err != nil {
		log.Printf("Ошибка получения списка групп: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var groupID int64
		if err := rows.Scan(&groupID); err != nil {
			log.Printf("Ошибка сканирования ID группы: %v", err)
			continue
		}

		msg := tgbotapi.NewMessage(groupID, message)
		if _, err := b.api.Send(msg); err != nil {
			log.Printf("Ошибка отправки сообщения в группу %d: %v", groupID, err)
		}
	}
}

// handleCommand обрабатывает команды бота
func (b *Bot) handleCommand(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

	switch update.Message.Command() {
	case "start":
		msg.Text = "Привет! Я бот для отслеживания дней рождения. Упомяните меня в группе, чтобы начать работу."
	case "help":
		msg.Text = "Доступные команды:\n" +
			"/add - Добавить день рождения\n" +
			"/list - Показать список дней рождения\n" +
			"/delete - Удалить день рождения\n" +
			"/help - Показать это сообщение"
	case "add":
		msg.Text = "Пожалуйста, введите имя и дату рождения в формате:\nИмя Фамилия ДД.ММ.ГГГГ"
	case "list":
		rows, err := b.db.Query(`
			SELECT first_name, last_name, strftime('%d.%m.%Y', birthday) as birthday
			FROM birthdays
			WHERE group_id = ?
			ORDER BY strftime('%m-%d', birthday)
		`, update.Message.Chat.ID)
		if err != nil {
			msg.Text = "Ошибка при получении списка дней рождения"
			break
		}
		defer rows.Close()

		var birthdays []string
		for rows.Next() {
			var firstName, lastName, birthday string
			if err := rows.Scan(&firstName, &lastName, &birthday); err != nil {
				continue
			}
			birthdays = append(birthdays, fmt.Sprintf("%s %s - %s", firstName, lastName, birthday))
		}

		if len(birthdays) == 0 {
			msg.Text = "Список дней рождения пуст"
		} else {
			msg.Text = "Список дней рождения:\n" + strings.Join(birthdays, "\n")
		}
	case "delete":
		rows, err := b.db.Query(`
			SELECT id, first_name, last_name, strftime('%d.%m.%Y', birthday) as birthday
			FROM birthdays
			WHERE group_id = ?
			ORDER BY strftime('%m-%d', birthday)
		`, update.Message.Chat.ID)
		if err != nil {
			msg.Text = "Ошибка при получении списка дней рождения"
			break
		}
		defer rows.Close()

		var buttons [][]tgbotapi.InlineKeyboardButton
		for rows.Next() {
			var id int
			var firstName, lastName, birthday string
			if err := rows.Scan(&id, &firstName, &lastName, &birthday); err != nil {
				continue
			}
			button := tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%s %s - %s", firstName, lastName, birthday),
				fmt.Sprintf("delete_%d", id),
			)
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(button))
		}

		if len(buttons) == 0 {
			msg.Text = "Список дней рождения пуст"
		} else {
			msg.Text = "Выберите день рождения для удаления:"
			keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
			msg.ReplyMarkup = keyboard
		}
	}

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
}

// handleBirthdayAddition обрабатывает добавление дня рождения
func (b *Bot) handleBirthdayAddition(update tgbotapi.Update) {
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 3 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пожалуйста, введите имя, фамилию и дату рождения в формате:\nИмя Фамилия ДД.ММ.ГГГГ")
		b.api.Send(msg)
		return
	}

	// Получаем имя и фамилию
	firstName := parts[0]
	lastName := parts[1]

	// Получаем дату рождения
	birthdayStr := parts[2]
	birthday, err := time.Parse("02.01.2006", birthdayStr)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный формат даты. Используйте формат ДД.ММ.ГГГГ")
		b.api.Send(msg)
		return
	}

	// Добавляем запись в базу данных
	_, err = b.db.Exec(`
		INSERT INTO birthdays (first_name, last_name, birthday, group_id)
		VALUES (?, ?, ?, ?)
	`, firstName, lastName, birthday.Format("2006-01-02"), update.Message.Chat.ID)

	if err != nil {
		log.Printf("Ошибка добавления дня рождения: %v", err)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Произошла ошибка при добавлении дня рождения")
		b.api.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("День рождения %s %s успешно добавлен!", firstName, lastName))
	b.api.Send(msg)
}

// handleMention обрабатывает упоминание бота
func (b *Bot) handleMention(update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🎯 Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
		),
	)
	b.api.Send(msg)
}

// handleCallback обрабатывает нажатия на кнопки
func (b *Bot) handleCallback(update tgbotapi.Update) {
	callback := update.CallbackQuery
	if callback == nil {
		return
	}

	// Отвечаем на callback query, чтобы убрать часики
	b.api.Send(tgbotapi.NewCallback(callback.ID, ""))

	data := callback.Data
	switch {
	case data == "show":
		// Получаем список дней рождения
		rows, err := b.db.Query(`
			SELECT first_name, last_name, birthday 
			FROM birthdays 
			WHERE group_id = ? 
			ORDER BY strftime('%m-%d', birthday)
		`, callback.Message.Chat.ID)
		if err != nil {
			msg := tgbotapi.NewEditMessageTextAndMarkup(
				callback.Message.Chat.ID,
				callback.Message.MessageID,
				"❌ Ошибка при получении списка дней рождения.",
				tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
					),
				),
			)
			b.api.Send(msg)
			return
		}
		defer rows.Close()

		var birthdays []string
		for rows.Next() {
			var firstName, lastName string
			var birthday time.Time
			if err := rows.Scan(&firstName, &lastName, &birthday); err != nil {
				continue
			}
			birthdays = append(birthdays, fmt.Sprintf("%d %s - %s %s",
				birthday.Day(),
				getMonthName(birthday.Month()),
				firstName,
				lastName))
		}

		var messageText string
		if len(birthdays) > 0 {
			messageText = "📅 Список дней рождения:\n" + strings.Join(birthdays, "\n")
		} else {
			messageText = "📅 Список дней рождения пуст"
		}

		msg := tgbotapi.NewEditMessageTextAndMarkup(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			messageText,
			tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
				),
			),
		)
		b.api.Send(msg)

	case data == "add":
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "📝 Пожалуйста, введите имя, фамилию и дату рождения в формате:\nИмя Фамилия ДД.ММ.ГГГГ")
		var buttons [][]tgbotapi.InlineKeyboardButton
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
		))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
		))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
		))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
		b.api.Send(msg)

	case data == "delete":
		rows, err := b.db.Query(`
			SELECT id, first_name, last_name, strftime('%d.%m', birthday) as birthday
			FROM birthdays 
			WHERE group_id = ?
			ORDER BY strftime('%m-%d', birthday)
		`, callback.Message.Chat.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "❌ Ошибка при получении списка дней рождения")
			var buttons [][]tgbotapi.InlineKeyboardButton
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
			))
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
			))
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
			))
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
			b.api.Send(msg)
			return
		}
		defer rows.Close()

		var buttons [][]tgbotapi.InlineKeyboardButton
		for rows.Next() {
			var id int
			var firstName, lastName, birthday string
			if err := rows.Scan(&id, &firstName, &lastName, &birthday); err != nil {
				continue
			}
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("❌ %s %s - %s", firstName, lastName, birthday),
					fmt.Sprintf("delete_%d", id),
				),
			))
		}

		if len(buttons) == 0 {
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "📅 Список дней рождения пуст")
			var menuButtons [][]tgbotapi.InlineKeyboardButton
			menuButtons = append(menuButtons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
			))
			menuButtons = append(menuButtons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
			))
			menuButtons = append(menuButtons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
			))
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(menuButtons...)
			b.api.Send(msg)
			return
		}

		// Добавляем кнопки меню в конец
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
		))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
		))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
		))

		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "Выберите день рождения для удаления:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
		b.api.Send(msg)

	case strings.HasPrefix(data, "delete_"):
		id := strings.TrimPrefix(data, "delete_")
		_, err := b.db.Exec("DELETE FROM birthdays WHERE id = ?", id)
		if err != nil {
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "❌ Ошибка при удалении дня рождения")
			var buttons [][]tgbotapi.InlineKeyboardButton
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
			))
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
			))
			buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
			))
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
			b.api.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "✅ День рождения успешно удален")
		var buttons [][]tgbotapi.InlineKeyboardButton
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Показать дни рождения", "show"),
		))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add"),
		))
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete"),
		))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(buttons...)
		b.api.Send(msg)
	}
}

// getMonthName возвращает название месяца в родительном падеже
func getMonthName(month time.Month) string {
	months := map[time.Month]string{
		time.January:   "января",
		time.February:  "февраля",
		time.March:     "марта",
		time.April:     "апреля",
		time.May:       "мая",
		time.June:      "июня",
		time.July:      "июля",
		time.August:    "августа",
		time.September: "сентября",
		time.October:   "октября",
		time.November:  "ноября",
		time.December:  "декабря",
	}
	return months[month]
}

// ensureGroupExists проверяет существование группы и создает её при необходимости
func ensureGroupExists(db *sql.DB, chatID int64, chatTitle string) error {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM groups WHERE id = ?)", chatID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		_, err = db.Exec("INSERT INTO groups (id, title) VALUES (?, ?)", chatID, chatTitle)
		if err != nil {
			return err
		}
		// Создаем настройки по умолчанию для новой группы
		_, err = db.Exec("INSERT INTO settings (group_id, notify_time) VALUES (?, ?)", chatID, "09:00")
		if err != nil {
			return err
		}
	}
	return nil
}

// checkBirthdays проверяет предстоящие дни рождения и отправляет уведомления
func (b *Bot) checkBirthdays() {
	// Получаем все группы
	rows, err := b.db.Query("SELECT id, notify_time FROM settings")
	if err != nil {
		log.Printf("Error getting groups: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var groupID int64
		var notifyTime string
		if err := rows.Scan(&groupID, &notifyTime); err != nil {
			continue
		}

		// Получаем текущее время
		now := time.Now()
		currentTime := now.Format("15:04")

		// Проверяем, пора ли отправлять уведомления
		if currentTime != notifyTime {
			continue
		}

		// 1. Проверяем дни рождения в ближайшие 7 дней
		weekBirthdays, err := b.getUpcomingBirthdays(groupID, 7)
		if err != nil {
			log.Printf("Error getting week birthdays: %v", err)
			continue
		}

		if len(weekBirthdays) > 0 {
			message := "🎂 В ближайшие 7 дней дни рождения у:\n" + strings.Join(weekBirthdays, "\n")
			msg := tgbotapi.NewMessage(groupID, message)
			b.api.Send(msg)
		}

		// 2. Проверяем дни рождения завтра
		tomorrowBirthdays, err := b.getUpcomingBirthdays(groupID, 1)
		if err != nil {
			log.Printf("Error getting tomorrow birthdays: %v", err)
			continue
		}

		if len(tomorrowBirthdays) > 0 {
			message := "📅 Завтра день рождения у:\n" + strings.Join(tomorrowBirthdays, "\n")
			msg := tgbotapi.NewMessage(groupID, message)
			b.api.Send(msg)
		}

		// 3. Проверяем дни рождения сегодня
		todayBirthdays, err := b.getTodayBirthdays(groupID)
		if err != nil {
			log.Printf("Error getting today birthdays: %v", err)
			continue
		}

		for _, birthday := range todayBirthdays {
			message := fmt.Sprintf("🎉 С Днем Рождения, %s! 🎉\n\n"+
				"Пусть этот день будет особенным и запомнится только радостными моментами! "+
				"Желаем тебе счастья, успехов во всех начинаниях и исполнения всех желаний! "+
				"Пусть каждый день приносит радость и улыбку! 🌟", birthday)
			msg := tgbotapi.NewMessage(groupID, message)
			b.api.Send(msg)
		}
	}
}

// getUpcomingBirthdays получает список дней рождения в ближайшие дни
func (b *Bot) getUpcomingBirthdays(groupID int64, days int) ([]string, error) {
	rows, err := b.db.Query(`
		SELECT first_name, last_name, birthday
		FROM birthdays
		WHERE group_id = ?
		AND strftime('%m-%d', birthday) = strftime('%m-%d', date('now', '+' || ? || ' days'))
		ORDER BY strftime('%m-%d', birthday)
	`, groupID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []string
	for rows.Next() {
		var firstName, lastName string
		var birthday time.Time
		if err := rows.Scan(&firstName, &lastName, &birthday); err != nil {
			continue
		}
		birthdays = append(birthdays, fmt.Sprintf("%d %s - %s %s",
			birthday.Day(),
			getMonthName(birthday.Month()),
			firstName,
			lastName))
	}
	return birthdays, nil
}

// getTodayBirthdays получает список дней рождения на сегодня
func (b *Bot) getTodayBirthdays(groupID int64) ([]string, error) {
	rows, err := b.db.Query(`
		SELECT first_name, last_name
		FROM birthdays
		WHERE group_id = ?
		AND strftime('%m-%d', birthday) = strftime('%m-%d', 'now')
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var birthdays []string
	for rows.Next() {
		var firstName, lastName string
		if err := rows.Scan(&firstName, &lastName); err != nil {
			continue
		}
		birthdays = append(birthdays, fmt.Sprintf("%s %s", firstName, lastName))
	}
	return birthdays, nil
}

// Start запускает бота
func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	// Запускаем проверку дней рождения каждый час
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			b.checkBirthdays()
		}
	}()

	for update := range updates {
		if update.Message == nil {
			if update.CallbackQuery != nil {
				b.handleCallback(update)
			}
			continue
		}

		// Получаем информацию о чате
		chat := update.Message.Chat
		if chat == nil {
			continue
		}

		// Проверяем и создаем группу при необходимости
		if err := ensureGroupExists(b.db, chat.ID, chat.Title); err != nil {
			log.Printf("Error ensuring group exists: %v", err)
			continue
		}

		// Обработка команд и упоминаний
		if update.Message.IsCommand() {
			b.handleCommand(update)
		} else if update.Message.Entities != nil {
			for _, entity := range update.Message.Entities {
				if entity.Type == "mention" {
					mention := update.Message.Text[entity.Offset : entity.Offset+entity.Length]
					if mention == "@"+b.api.Self.UserName {
						b.handleMention(update)
						break
					}
				}
			}
		} else if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From.ID == b.api.Self.ID {
			b.handleBirthdayAddition(update)
		}
	}
}

func main() {
	// Получаем токен бота из переменных окружения
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не установлен")
	}

	// Получаем путь к базе данных из переменных окружения
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		log.Fatal("DATABASE_PATH не установлен")
	}

	// Создаем экземпляр бота
	bot, err := NewBot(token, dbPath)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}
	defer bot.Close()

	// Запускаем бота
	bot.Start()
}
