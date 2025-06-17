package bot

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"Eldarius_bot/internal/config"
	"Eldarius_bot/internal/scheduler"
	"Eldarius_bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Service представляет сервис бота
type Service struct {
	bot       *tgbotapi.BotAPI
	handler   *Handler
	store     storage.Repository
	scheduler *scheduler.Scheduler
	config    *config.Config
}

// NewService создает новый сервис
func NewService(cfg *config.Config, store storage.Repository) (*Service, error) {
	// Создаем экземпляр бота
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания бота: %w", err)
	}

	// Создаем обработчик
	handler := NewHandler(store, bot)

	// Создаем планировщик
	scheduler := scheduler.NewScheduler(store, bot)

	return &Service{
		config:    cfg,
		store:     store,
		bot:       bot,
		handler:   handler,
		scheduler: scheduler,
	}, nil
}

// Start запускает сервис
func (s *Service) Start() error {
	// Настраиваем бота
	s.bot.Debug = s.config.Debug

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем планировщик
	go func() {
		if err := s.scheduler.Start(ctx); err != nil {
			fmt.Printf("Ошибка планировщика: %v\n", err)
		}
	}()

	// Настраиваем получение обновлений
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := s.bot.GetUpdatesChan(updateConfig)

	// Обрабатываем сигналы завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Основной цикл обработки
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sigChan:
			return nil
		case update := <-updates:
			if update.Message != nil {
				if err := s.handler.HandleMessage(update.Message); err != nil {
					fmt.Printf("Ошибка обработки сообщения: %v\n", err)
				}
			} else if update.CallbackQuery != nil {
				if err := s.handler.HandleCallback(update.CallbackQuery); err != nil {
					fmt.Printf("Ошибка обработки callback: %v\n", err)
				}
			}
		}
	}
}

// Stop останавливает сервис
func (s *Service) Stop() error {
	// Закрываем соединение с базой данных
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("ошибка закрытия хранилища: %w", err)
	}

	return nil
}
