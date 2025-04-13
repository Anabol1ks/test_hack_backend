package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name         string `gorm:"not null"`
	Surname      string `gorm:"not null"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
}

type Schedule struct {
	gorm.Model
	ExternalID string    `gorm:"uniqueIndex"`    // Идентификатор из внешнего API
	Name       string    `gorm:"not null"`       // Название события
	StartTime  time.Time `gorm:"index;not null"` // Начало события
	EndTime    time.Time `gorm:"not null"`       // Окончание события
	GroupIDs   string    `gorm:"not null"`       // Список ID групп, например "67,203,111"
}

type Queue struct {
	gorm.Model
	ScheduleID      uint      `gorm:"index;not null"` // Ссылка на событие из расписания (или может быть null, если очередь создаётся отдельно)
	OpensAt         time.Time `gorm:"index"`          // Время открытия очереди (обычно за 24 часа до начала события)
	ClosesAt        time.Time `gorm:"index"`          // Время закрытия очереди (время начала события)
	IsActive        bool      `gorm:"default:false"`  // Флаг активности очереди
	MaxParticipants int       // Опциональный лимит участников очереди
}

type QueueEntry struct {
	gorm.Model
	UserID   uint       `gorm:"index;not null"`
	User     User       `gorm:"foreignKey:UserID"`
	QueueID  uint       `gorm:"index;not null"`
	Position int        `gorm:"index;not null"` // Текущая позиция в очереди
	ExitedAt *time.Time // Время выхода из очереди, если пользователь покинул очередь (nil — активный участник)
}
