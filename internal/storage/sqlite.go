package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"Eldarius_bot/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

// SQLite реализует интерфейс Repository для SQLite
type SQLite struct {
	db *sql.DB
}

// NewSQLite создает новое подключение к SQLite
func NewSQLite(dbPath string) (*SQLite, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия базы данных: %w", err)
	}

	// Создаем таблицы, если они не существуют
	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLite{db: db}, nil
}

// createTables создает необходимые таблицы в базе данных
func createTables(db *sql.DB) error {
	// Таблица групп
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("ошибка создания таблицы групп: %w", err)
	}

	// Таблица дней рождения
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS birthdays (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			birthday DATE NOT NULL,
			group_id INTEGER NOT NULL,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("ошибка создания таблицы дней рождения: %w", err)
	}

	// Таблица настроек
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			group_id INTEGER PRIMARY KEY,
			notify_time TEXT NOT NULL,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("ошибка создания таблицы настроек: %w", err)
	}

	return nil
}

// AddBirthday добавляет запись о дне рождения
func (s *SQLite) AddBirthday(ctx context.Context, birthday *models.Birthday) error {
	// Проверяем валидность записи
	if err := birthday.Validate(); err != nil {
		return fmt.Errorf("невалидная запись о дне рождения: %w", err)
	}

	// Проверяем существование группы
	group := &models.Group{
		ID:    birthday.GroupID,
		Title: "", // Название группы будет обновлено позже
	}

	// Добавляем группу, если она не существует
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO groups (id, title)
		VALUES (?, ?)
	`, group.ID, group.Title)
	if err != nil {
		return fmt.Errorf("ошибка добавления группы: %w", err)
	}

	// Проверяем количество дней рождения в группе
	var count int
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM birthdays WHERE group_id = ?
	`, birthday.GroupID).Scan(&count)
	if err != nil {
		return fmt.Errorf("ошибка подсчета дней рождения: %w", err)
	}

	if count >= 100 {
		return fmt.Errorf("превышен лимит дней рождения в группе (100)")
	}

	// Добавляем день рождения
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO birthdays (name, birthday, group_id)
		VALUES (?, ?, ?)
	`, birthday.Name, birthday.Birthday, birthday.GroupID)
	if err != nil {
		return fmt.Errorf("ошибка добавления дня рождения: %w", err)
	}

	return nil
}

// GetBirthdays возвращает список дней рождения для группы
func (s *SQLite) GetBirthdays(ctx context.Context, groupID int64) ([]*models.Birthday, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, birthday, group_id
		FROM birthdays
		WHERE group_id = ?
		ORDER BY birthday
	`, groupID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения дней рождения: %w", err)
	}
	defer rows.Close()

	var birthdays []*models.Birthday
	for rows.Next() {
		b := &models.Birthday{}
		err := rows.Scan(&b.ID, &b.Name, &b.Birthday, &b.GroupID)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования дня рождения: %w", err)
		}
		birthdays = append(birthdays, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении дней рождения: %w", err)
	}

	return birthdays, nil
}

// DeleteBirthday удаляет запись о дне рождения
func (s *SQLite) DeleteBirthday(ctx context.Context, groupID int64, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM birthdays
		WHERE id = ? AND group_id = ?
	`, id, groupID)
	if err != nil {
		return fmt.Errorf("ошибка удаления дня рождения: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("ошибка получения количества удаленных строк: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("день рождения не найден")
	}

	return nil
}

// GetUpcomingBirthdays возвращает список предстоящих дней рождения
func (s *SQLite) GetUpcomingBirthdays(ctx context.Context, groupID int64, days int) ([]*models.Birthday, error) {
	if days <= 0 {
		return nil, fmt.Errorf("количество дней должно быть положительным")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, birthday, group_id
		FROM birthdays
		WHERE group_id = ?
		AND strftime('%m-%d', birthday) BETWEEN strftime('%m-%d', 'now')
		AND strftime('%m-%d', 'now', '+' || ? || ' days')
		ORDER BY strftime('%m-%d', birthday)
	`, groupID, days)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения предстоящих дней рождения: %w", err)
	}
	defer rows.Close()

	var birthdays []*models.Birthday
	for rows.Next() {
		b := &models.Birthday{}
		err := rows.Scan(&b.ID, &b.Name, &b.Birthday, &b.GroupID)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования дня рождения: %w", err)
		}
		birthdays = append(birthdays, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении дней рождения: %w", err)
	}

	return birthdays, nil
}

// AddGroup добавляет группу
func (s *SQLite) AddGroup(ctx context.Context, group *models.Group) error {
	if err := group.Validate(); err != nil {
		return fmt.Errorf("невалидная запись о группе: %w", err)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO groups (id, title)
		VALUES (?, ?)
	`, group.ID, group.Title)
	if err != nil {
		return fmt.Errorf("ошибка добавления группы: %w", err)
	}

	return nil
}

// GetGroup возвращает информацию о группе
func (s *SQLite) GetGroup(ctx context.Context, id int64) (*models.Group, error) {
	group := &models.Group{ID: id}
	err := s.db.QueryRowContext(ctx, `
		SELECT title FROM groups WHERE id = ?
	`, id).Scan(&group.Title)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("группа не найдена")
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка получения группы: %w", err)
	}

	return group, nil
}

// GetAllGroups возвращает список всех групп
func (s *SQLite) GetAllGroups(ctx context.Context) ([]*models.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title FROM groups
	`)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения групп: %w", err)
	}
	defer rows.Close()

	var groups []*models.Group
	for rows.Next() {
		g := &models.Group{}
		err := rows.Scan(&g.ID, &g.Title)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования группы: %w", err)
		}
		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при чтении групп: %w", err)
	}

	return groups, nil
}

// GetNotifyTime возвращает время уведомления для группы
func (s *SQLite) GetNotifyTime(ctx context.Context, groupID int64) (time.Time, error) {
	var timeStr string
	err := s.db.QueryRowContext(ctx, `
		SELECT notify_time FROM settings WHERE group_id = ?
	`, groupID).Scan(&timeStr)
	if err == sql.ErrNoRows {
		// Возвращаем время по умолчанию (9:00)
		return time.Date(0, 0, 0, 9, 0, 0, 0, time.Local), nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("ошибка получения времени уведомления: %w", err)
	}

	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("ошибка парсинга времени: %w", err)
	}

	return t, nil
}

// SetNotifyTime устанавливает время уведомления для группы
func (s *SQLite) SetNotifyTime(ctx context.Context, groupID int64, t time.Time) error {
	timeStr := t.Format("15:04")
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO settings (group_id, notify_time)
		VALUES (?, ?)
	`, groupID, timeStr)
	if err != nil {
		return fmt.Errorf("ошибка установки времени уведомления: %w", err)
	}

	return nil
}

// Close закрывает соединение с базой данных
func (s *SQLite) Close() error {
	return s.db.Close()
}
