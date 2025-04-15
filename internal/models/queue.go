package models

import (
	"time"

	"gorm.io/gorm"
)

type Queue struct {
	gorm.Model
	ScheduleID      uint      `gorm:"index;not null"` // Ссылка на событие из расписания (или может быть null, если очередь создаётся отдельно)
	OpensAt         time.Time `gorm:"index"`          // Время открытия очереди (обычно за 24 часа до начала события)
	ClosesAt        time.Time `gorm:"index"`          // Время закрытия очереди (время начала события)
	IsActive        bool      `gorm:"default:false"`  // Флаг активности очереди
	MaxParticipants int       // Опциональный лимит участников очереди
}
