package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"Eldarius_bot/internal/models"
	"Eldarius_bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// Number of days to notify in advance
	advanceNotificationDays = 3
)

// Scheduler планирует и отправляет уведомления о днях рождения
type Scheduler struct {
	store storage.Repository
	bot   *tgbotapi.BotAPI
}

// NewScheduler создает новый планировщик уведомлений
func NewScheduler(store storage.Repository, bot *tgbotapi.BotAPI) *Scheduler {
	return &Scheduler{
		store: store,
		bot:   bot,
	}
}

// Start запускает планировщик уведомлений
func (s *Scheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.checkBirthdays(ctx); err != nil {
				// Логируем ошибку, но продолжаем работу
				fmt.Printf("Ошибка при проверке дней рождения: %v\n", err)
			}
		}
	}
}

// checkBirthdays проверяет дни рождения и отправляет уведомления
func (s *Scheduler) checkBirthdays(ctx context.Context) error {
	// Получаем все группы
	groups, err := s.store.GetAllGroups(ctx)
	if err != nil {
		return fmt.Errorf("ошибка получения групп: %w", err)
	}

	// Проверяем дни рождения для каждой группы
	for _, group := range groups {
		// Получаем время уведомления для группы
		notifyTime, err := s.store.GetNotifyTime(ctx, group.ID)
		if err != nil {
			fmt.Printf("Ошибка получения времени уведомления для группы %d: %v\n", group.ID, err)
			continue
		}

		// Проверяем, нужно ли отправлять уведомление
		if !s.shouldNotify(notifyTime) {
			continue
		}

		// Получаем предстоящие дни рождения
		birthdays, err := s.store.GetUpcomingBirthdays(ctx, group.ID, 7)
		if err != nil {
			fmt.Printf("Ошибка получения предстоящих дней рождения для группы %d: %v\n", group.ID, err)
			continue
		}

		if len(birthdays) == 0 {
			continue
		}

		// Отправляем уведомление
		if err := s.sendGroupNotification(ctx, group.ID, birthdays); err != nil {
			fmt.Printf("Ошибка отправки уведомления в группу %d: %v\n", group.ID, err)
		}
	}

	return nil
}

// shouldNotify проверяет, нужно ли отправлять уведомление
func (s *Scheduler) shouldNotify(notifyTime time.Time) bool {
	now := time.Now()
	targetTime := time.Date(now.Year(), now.Month(), now.Day(),
		notifyTime.Hour(), notifyTime.Minute(), 0, 0, time.Local)

	// Проверяем, что текущее время находится в пределах одной минуты от целевого времени
	return now.Sub(targetTime).Abs() <= time.Minute
}

// sendGroupNotification отправляет уведомление в группу
func (s *Scheduler) sendGroupNotification(ctx context.Context, groupID int64, birthdays []*models.Birthday) error {
	var text strings.Builder
	text.WriteString("🎂 Предстоящие дни рождения:\n\n")

	for _, b := range birthdays {
		nextBirthday := getNextBirthday(b.Birthday, time.Now())
		daysUntil := int(nextBirthday.Sub(time.Now()).Hours() / 24)

		if daysUntil == 0 {
			text.WriteString(fmt.Sprintf("🎉 Сегодня день рождения у %s!\n", b.Name))
		} else {
			text.WriteString(fmt.Sprintf("📅 Через %d %s день рождения у %s\n",
				daysUntil,
				getDaysWord(daysUntil),
				b.Name))
		}
	}

	msg := tgbotapi.NewMessage(groupID, text.String())
	_, err := s.bot.Send(msg)
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
