package models

import (
	"time"

	"gorm.io/gorm"
)

type Schedule struct {
	gorm.Model
	ExternalID string    `gorm:"uniqueIndex"`    // Идентификатор из внешнего API
	Name       string    `gorm:"not null"`       // Название события
	StartTime  time.Time `gorm:"index;not null"` // Начало события
	EndTime    time.Time `gorm:"not null"`       // Окончание события
	GroupIDs   string    `gorm:"not null"`       // Список ID групп, например "67,203,111"
}
