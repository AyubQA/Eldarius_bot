package storage

import (
	"context"
	"time"

	"Eldarius_bot/internal/models"
)

// Repository определяет интерфейс для работы с хранилищем данных
type Repository interface {
	// Методы для работы с днями рождения
	AddBirthday(ctx context.Context, birthday *models.Birthday) error
	GetBirthdays(ctx context.Context, groupID int64) ([]*models.Birthday, error)
	DeleteBirthday(ctx context.Context, groupID int64, id int64) error
	GetUpcomingBirthdays(ctx context.Context, groupID int64, days int) ([]*models.Birthday, error)

	// Методы для работы с группами
	AddGroup(ctx context.Context, group *models.Group) error
	GetGroup(ctx context.Context, id int64) (*models.Group, error)
	GetAllGroups(ctx context.Context) ([]*models.Group, error)

	// Методы для работы с настройками
	GetNotifyTime(ctx context.Context, groupID int64) (time.Time, error)
	SetNotifyTime(ctx context.Context, groupID int64, t time.Time) error

	// Методы управления соединением
	Close() error
}
