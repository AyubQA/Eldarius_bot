# Birthday Bot

Telegram бот для отслеживания дней рождения, написанный на Go.

## Описание

Бот помогает отслеживать дни рождения друзей и знакомых, отправляя уведомления в Telegram.

## Функциональность

- Добавление дней рождения
- Удаление дней рождения
- Просмотр списка дней рождения
- Автоматические уведомления о приближающихся днях рождения

## Технологии

- Go 1.24
- SQLite
- Docker
- Amvera Cloud

## Установка и запуск

### Локальная разработка

1. Клонируйте репозиторий:
```bash
git clone https://github.com/your-username/birthday-bot.git
cd birthday-bot
```

2. Установите зависимости:
```bash
go mod download
```

3. Создайте файл .env:
```bash
TELEGRAM_BOT_TOKEN=your_bot_token
```

4. Запустите бота:
```bash
go run main.go
```

### Docker

1. Соберите образ:
```bash
docker build -t birthday-bot .
```

2. Запустите контейнер:
```bash
docker run -d \
  -v $(pwd)/data:/app/data \
  -e TELEGRAM_BOT_TOKEN=your_token \
  birthday-bot
```

## Развертывание на Amvera

1. Создайте ZIP-архив проекта
2. Загрузите в Amvera через веб-интерфейс
3. Настройте переменные окружения в панели управления Amvera

## Структура проекта

```
.
├── cmd/            # Точка входа приложения
├── internal/       # Внутренние пакеты
├── pkg/           # Публичные пакеты
├── data/          # Данные приложения
├── Dockerfile     # Конфигурация Docker
├── amvera.yaml    # Конфигурация Amvera
├── go.mod         # Зависимости Go
└── main.go        # Основной файл
```

## Лицензия

MIT

## Автор

[Ваше имя] 