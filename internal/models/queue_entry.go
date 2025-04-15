package models

import (
	"time"

	"gorm.io/gorm"
)

type QueueEntry struct {
	gorm.Model
	UserID   uint       `gorm:"index;not null"`
	User     User       `gorm:"foreignKey:UserID"`
	QueueID  uint       `gorm:"index;not null"`
	Position int        `gorm:"index;not null"` // Текущая позиция в очереди
	ExitedAt *time.Time // Время выхода из очереди, если пользователь покинул очередь (nil — активный участник)
}
