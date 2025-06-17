package models

import (
	"fmt"
	"time"
)

// Birthday представляет запись о дне рождения
type Birthday struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Birthday time.Time `json:"birthday"`
	GroupID  int64     `json:"group_id"`
}

// Group представляет группу в Telegram
type Group struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// Validate проверяет валидность записи о дне рождения
func (b *Birthday) Validate() error {
	if b.Name == "" {
		return fmt.Errorf("имя не может быть пустым")
	}

	if b.Birthday.IsZero() {
		return fmt.Errorf("дата рождения не может быть пустой")
	}

	// Проверяем, что дата рождения не в будущем
	if b.Birthday.After(time.Now()) {
		return fmt.Errorf("дата рождения не может быть в будущем")
	}

	// Проверяем, что дата рождения не слишком старая (например, не старше 150 лет)
	if time.Now().Sub(b.Birthday) > 150*365*24*time.Hour {
		return fmt.Errorf("дата рождения слишком старая")
	}

	return nil
}

// Validate проверяет валидность записи о группе
func (g *Group) Validate() error {
	if g.ID == 0 {
		return fmt.Errorf("ID группы не может быть пустым")
	}

	if g.Title == "" {
		return fmt.Errorf("название группы не может быть пустым")
	}

	return nil
}
