package tasks

import (
	"log"
	"test_hack/internal/models"
	"test_hack/internal/storage"
	"time"

	"github.com/robfig/cron/v3"
)

// CreateQueueForUpcomingEvents ищет события, для которых наступает время открытия очереди, и создаёт очередь
func CreateQueueForUpcomingEvents() {
	now := time.Now()
	// Пример: ищем события, у которых StartTime через 24 часа ± небольшой интервал
	startWindow := now.Add(24 * time.Hour).Add(-5 * time.Minute)
	endWindow := now.Add(24 * time.Hour).Add(5 * time.Minute)

	var schedules []models.Schedule
	if err := storage.DB.Where("start_time BETWEEN ? AND ?", startWindow, endWindow).Find(&schedules).Error; err != nil {
		log.Println("Ошибка при поиске событий для очереди:", err)
		return
	}

	for _, sched := range schedules {
		// Проверка, существует ли уже очередь для этого события
		var queue models.Queue
		if err := storage.DB.Where("schedule_id = ?", sched.ID).First(&queue).Error; err == nil {
			// Очередь уже создана
			continue
		}

		// Создание очереди
		newQueue := models.Queue{
			ScheduleID: sched.ID,
			OpensAt:    sched.StartTime.Add(-24 * time.Hour),
			ClosesAt:   sched.StartTime,
			IsActive:   true,
		}
		if err := storage.DB.Create(&newQueue).Error; err != nil {
			log.Println("Ошибка создания очереди:", err)
		} else {
			log.Printf("Очередь для события %s создана\n", sched.Name)
		}
	}
}

// InitScheduler инициализирует планировщик cron задач
func InitScheduler() *cron.Cron {
	c := cron.New(cron.WithSeconds())
	// Запускаем задачу каждые 5 минут
	_, err := c.AddFunc("*/300 * * * * *", CreateQueueForUpcomingEvents)
	if err != nil {
		log.Println("Ошибка запуска cron-задачи:", err)
	}
	c.Start()
	return c
}
