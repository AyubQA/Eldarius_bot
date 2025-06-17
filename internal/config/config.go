package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config содержит конфигурацию приложения
type Config struct {
	Token string // Токен Telegram бота
	Debug bool   // Режим отладки
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// Загружаем .env файл, если он существует
	_ = godotenv.Load()

	// Получаем токен бота
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("не указан токен бота (TELEGRAM_BOT_TOKEN)")
	}

	// Получаем режим отладки
	debug := os.Getenv("DEBUG") == "true"

	return &Config{
		Token: token,
		Debug: debug,
	}, nil
}
