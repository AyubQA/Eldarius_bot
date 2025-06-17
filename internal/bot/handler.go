package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"Eldarius_bot/internal/models"
	"Eldarius_bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Handler обрабатывает команды и сообщения от пользователей
type Handler struct {
	store storage.Repository
	bot   *tgbotapi.BotAPI
}

// NewHandler создает новый обработчик команд
func NewHandler(store storage.Repository, bot *tgbotapi.BotAPI) *Handler {
	return &Handler{
		store: store,
		bot:   bot,
	}
}

// HandleUpdate обрабатывает обновление от Telegram
func (h *Handler) HandleUpdate(update *tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}

	// Проверяем, упомянут ли бот
	if !h.isBotMentioned(update.Message) {
		return nil
	}

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Отправляем меню
	return h.sendMainMenu(ctx, update.Message.Chat.ID)
}

// isBotMentioned проверяет, упомянут ли бот в сообщении
func (h *Handler) isBotMentioned(message *tgbotapi.Message) bool {
	if message.Entities == nil {
		return false
	}

	for _, entity := range message.Entities {
		if entity.Type == "mention" {
			mention := message.Text[entity.Offset : entity.Offset+entity.Length]
			botUsername := "@" + h.bot.Self.UserName
			return mention == botUsername
		}
	}
	return false
}

// sendMainMenu отправляет главное меню
func (h *Handler) sendMainMenu(ctx context.Context, chatID int64) error {
	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Показать дни рождения", "show_birthdays"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ Добавить день рождения", "add_birthday"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Удалить день рождения", "delete_birthday"),
		),
	)
	_, err := h.bot.Send(msg)
	return err
}

// HandleCallback обрабатывает нажатия на кнопки меню
func (h *Handler) HandleCallback(callback *tgbotapi.CallbackQuery) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Обрабатываем нажатие на кнопку удаления
	if strings.HasPrefix(callback.Data, "delete_name_") {
		return h.handleDeleteBirthdayCallback(ctx, callback)
	}

	switch callback.Data {
	case "show_birthdays":
		return h.handleShowBirthdays(ctx, callback.Message.Chat.ID)
	case "add_birthday":
		return h.handleAddBirthday(ctx, callback.Message.Chat.ID)
	case "delete_birthday":
		return h.handleDeleteBirthday(ctx, callback.Message.Chat.ID)
	default:
		return fmt.Errorf("неизвестный callback: %s", callback.Data)
	}
}

// handleShowBirthdays показывает список дней рождения
func (h *Handler) handleShowBirthdays(ctx context.Context, chatID int64) error {
	birthdays, err := h.store.GetBirthdays(ctx, chatID)
	if err != nil {
		return fmt.Errorf("ошибка при получении дней рождения: %w", err)
	}

	if len(birthdays) == 0 {
		msg := tgbotapi.NewMessage(chatID, "В этой группе пока нет дней рождения.")
		_, err := h.bot.Send(msg)
		return err
	}

	// Сортируем дни рождения по ближайшей дате
	sort.Slice(birthdays, func(i, j int) bool {
		now := time.Now()
		nextBirthdayI := getNextBirthday(birthdays[i].Birthday, now)
		nextBirthdayJ := getNextBirthday(birthdays[j].Birthday, now)
		return nextBirthdayI.Before(nextBirthdayJ)
	})

	var text strings.Builder
	text.WriteString("📅 Дни рождения в группе:\n\n")
	for _, b := range birthdays {
		nextBirthday := getNextBirthday(b.Birthday, time.Now())
		daysUntil := int(nextBirthday.Sub(time.Now()).Hours() / 24)

		if daysUntil == 0 {
			text.WriteString(fmt.Sprintf("🎉 %s - СЕГОДНЯ! (%s)\n",
				b.Name,
				b.Birthday.Format("02.01.2006")))
		} else {
			text.WriteString(fmt.Sprintf("🎂 %s - %d %s (%s)\n",
				b.Name,
				daysUntil,
				getDaysWord(daysUntil),
				b.Birthday.Format("02.01.2006")))
		}
	}

	msg := tgbotapi.NewMessage(chatID, text.String())
	_, err = h.bot.Send(msg)
	return err
}

// getNextBirthday вычисляет дату следующего дня рождения
func getNextBirthday(birthday time.Time, now time.Time) time.Time {
	nextBirthday := time.Date(now.Year(), birthday.Month(), birthday.Day(), 0, 0, 0, 0, time.Local)
	if nextBirthday.Before(now) {
		nextBirthday = nextBirthday.AddDate(1, 0, 0)
	}
	return nextBirthday
}

// getDaysWord возвращает правильное склонение слова "день"
func getDaysWord(days int) string {
	if days%10 == 1 && days%100 != 11 {
		return "день"
	}
	if days%10 >= 2 && days%10 <= 4 && (days%100 < 10 || days%100 >= 20) {
		return "дня"
	}
	return "дней"
}

// handleAddBirthday показывает меню добавления дня рождения
func (h *Handler) handleAddBirthday(ctx context.Context, chatID int64) error {
	msg := tgbotapi.NewMessage(chatID, "Введите имя, фамилию и дату рождения в формате:\nИмя Фамилия ДД.ММ.ГГГГ")
	_, err := h.bot.Send(msg)
	return err
}

// handleDeleteBirthday показывает меню удаления дня рождения
func (h *Handler) handleDeleteBirthday(ctx context.Context, chatID int64) error {
	// Получаем список дней рождения
	birthdays, err := h.store.GetBirthdays(ctx, chatID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка при получении дней рождения: %v", err))
		_, err := h.bot.Send(msg)
		return err
	}

	if len(birthdays) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📝 В этой группе пока нет дней рождения.")
		_, err := h.bot.Send(msg)
		return err
	}

	// Создаем клавиатуру со списком дней рождения
	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, b := range birthdays {
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("❌ %s (%s)", b.Name, b.Birthday.Format("02.01.2006")),
				fmt.Sprintf("delete_name_%s", b.Name),
			),
		))
	}

	msg := tgbotapi.NewMessage(chatID, "🗑 Выберите день рождения для удаления из списка:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	_, err = h.bot.Send(msg)
	return err
}

// handleDeleteBirthdayCallback обрабатывает нажатие на кнопку удаления дня рождения
func (h *Handler) handleDeleteBirthdayCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	// Извлекаем имя из callback data
	name := strings.TrimPrefix(callback.Data, "delete_name_")

	// Получаем список дней рождения
	birthdays, err := h.store.GetBirthdays(ctx, callback.Message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка при получении дней рождения: %v", err))
		_, err := h.bot.Send(msg)
		return err
	}

	// Ищем день рождения по имени
	var foundBirthday *models.Birthday
	for _, b := range birthdays {
		if b.Name == name {
			foundBirthday = b
			break
		}
	}

	if foundBirthday == nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "❌ День рождения не найден")
		_, err := h.bot.Send(msg)
		return err
	}

	// Удаляем день рождения
	if err := h.store.DeleteBirthday(ctx, callback.Message.Chat.ID, foundBirthday.ID); err != nil {
		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("❌ Ошибка при удалении дня рождения: %v", err))
		_, err := h.bot.Send(msg)
		return err
	}

	// Отправляем подтверждение
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, fmt.Sprintf("✅ День рождения %s успешно удален!", foundBirthday.Name))
	_, err = h.bot.Send(msg)
	return err
}

// HandleMessage обрабатывает текстовые сообщения
func (h *Handler) HandleMessage(message *tgbotapi.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Проверяем, является ли сообщение командой
	if message.IsCommand() {
		switch message.Command() {
		case "start":
			return h.sendMainMenu(ctx, message.Chat.ID)
		case "help":
			helpText := `Доступные команды:
/start - Показать главное меню
/help - Показать это сообщение
/remind - Напомнить о днях рождения

Также вы можете упомянуть бота (@username) для вызова меню.`
			msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
			_, err := h.bot.Send(msg)
			return err
		case "remind":
			return h.handleShowBirthdays(ctx, message.Chat.ID)
		}
		return nil
	}

	// Проверяем, упомянут ли бот
	if h.isBotMentioned(message) {
		return h.sendMainMenu(ctx, message.Chat.ID)
	}

	// Проверяем, является ли сообщение ответом на запрос добавления дня рождения
	if message.ReplyToMessage != nil && message.ReplyToMessage.Text == "Введите имя, фамилию и дату рождения в формате:\nИмя Фамилия ДД.ММ.ГГГГ" {
		return h.processAddBirthday(ctx, message)
	}

	// Проверяем, является ли сообщение ответом на запрос удаления дня рождения
	if message.ReplyToMessage != nil && strings.Contains(message.ReplyToMessage.Text, "Введите имя и фамилию человека, чей день рождения нужно удалить") {
		return h.processDeleteBirthdayByName(ctx, message)
	}

	return nil
}

// processAddBirthday обрабатывает добавление дня рождения
func (h *Handler) processAddBirthday(ctx context.Context, message *tgbotapi.Message) error {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат. Используйте: Имя Фамилия ДД.ММ.ГГГГ")
		_, err := h.bot.Send(msg)
		return err
	}

	// Извлекаем дату из последней части
	dateStr := parts[len(parts)-1]
	birthday, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неверный формат даты. Используйте: ДД.ММ.ГГГГ")
		_, err := h.bot.Send(msg)
		return err
	}

	// Объединяем все части кроме последней как имя
	name := strings.Join(parts[:len(parts)-1], " ")

	// Создаем запись о дне рождения
	b := &models.Birthday{
		Name:     name,
		Birthday: birthday,
		GroupID:  message.Chat.ID,
	}

	// Сохраняем в базу данных
	if err := h.store.AddBirthday(ctx, b); err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка при добавлении дня рождения: %v", err))
		_, err := h.bot.Send(msg)
		return err
	}

	// Отправляем подтверждение
	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ День рождения %s успешно добавлен!", name))
	_, err = h.bot.Send(msg)
	return err
}

// processDeleteBirthdayByName обрабатывает удаление дня рождения по имени
func (h *Handler) processDeleteBirthdayByName(ctx context.Context, message *tgbotapi.Message) error {
	// Получаем список дней рождения
	birthdays, err := h.store.GetBirthdays(ctx, message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Ошибка при получении дней рождения: %v", err))
		_, err := h.bot.Send(msg)
		return err
	}

	// Ищем день рождения по имени
	var foundBirthday *models.Birthday
	for _, b := range birthdays {
		if strings.EqualFold(b.Name, message.Text) {
			foundBirthday = b
			break
		}
	}

	if foundBirthday == nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ День рождения не найден. Проверьте правильность имени и фамилии.")
		_, err := h.bot.Send(msg)
		return err
	}

	// Удаляем день рождения
	if err := h.store.DeleteBirthday(ctx, message.Chat.ID, foundBirthday.ID); err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("❌ Ошибка при удалении дня рождения: %v", err))
		_, err := h.bot.Send(msg)
		return err
	}

	// Отправляем подтверждение
	msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ День рождения %s успешно удален!", foundBirthday.Name))
	_, err = h.bot.Send(msg)
	return err
}
