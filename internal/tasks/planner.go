package tasks

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"test_hack/internal/models"
	"test_hack/internal/storage"
	"test_hack/internal/ws"

	"github.com/robfig/cron/v3"
)

// CreateQueueForUpcomingEvents ищет события, для которых наступает время открытия очереди, и создаёт очередь.
func CreateQueueForUpcomingEvents() {
	now := time.Now()
	// Ищем события, у которых начало происходит в период от текущего момента до (текущего времени + 24 часа + 5 минут)
	startWindow := now
	endWindow := now.Add(28 * time.Hour).Add(5 * time.Minute)

	log.Printf("Поиск событий в окне: %s - %s\n", startWindow.Format(time.RFC3339), endWindow.Format(time.RFC3339))

	var schedules []models.Schedule
	if err := storage.DB.Where("start_time BETWEEN ? AND ?", startWindow, endWindow).Find(&schedules).Error; err != nil {
		log.Println("Ошибка при поиске событий для очереди:", err)
		return
	}

	if len(schedules) == 0 {
		log.Println("Не найдено событий для создания очередей.")
		return
	}

	for _, sched := range schedules {
		// Проверяем, что событие ещё не началось
		if sched.StartTime.Before(now) {
			continue
		}

		// Проверка, существует ли уже очередь для данного события
		var queue models.Queue
		err := storage.DB.Where("schedule_id = ?", sched.ID).First(&queue).Error
		if err == nil {
			// Очередь уже создана, пропускаем событие
			log.Printf("Очередь для события '%s' уже существует.\n", sched.Name)
			continue
		}

		// Создание новой очереди
		newQueue := models.Queue{
			ScheduleID: sched.ID,
			OpensAt:    now,             // Открываем очередь сразу, если событие в пределах 24 часов
			ClosesAt:   sched.StartTime, // Закрытие очереди в момент начала события
			IsActive:   true,
		}
		if err := storage.DB.Create(&newQueue).Error; err != nil {
			log.Println("Ошибка создания очереди для события", sched.Name, ":", err)
		} else {
			log.Printf("Очередь для события '%s' создана успешно.\n", sched.Name)
		}
	}
}

// InitScheduler инициализирует планировщик cron-задач.
func InitScheduler() *cron.Cron {
	c := cron.New(cron.WithSeconds())

	// Задача создания очередей каждые 5 минут.
	_, err := c.AddFunc("0 */5 * * * *", CreateQueueForUpcomingEvents)
	if err != nil {
		log.Println("Ошибка запуска cron-задачи CreateQueueForUpcomingEvents:", err)
	}

	// Задача очистки устаревших расписаний, например, каждый день в 03:00.
	_, err = c.AddFunc("0 0 3 * * *", CleanOldSchedules)
	if err != nil {
		log.Println("Ошибка запуска cron-задачи CleanOldSchedules:", err)
	}

	_, err = c.AddFunc("0 5 3 * * *", CleanExpiredQueues)
	if err != nil {
		log.Println("Ошибка запуска cron-задачи CleanExpiredQueues:", err)
	}

	_, err = c.AddFunc("0 * * * * *", CloseExpiredQueues)
	if err != nil {
		log.Println("Ошибка запуска cron-задачи CloseExpiredQueues:", err)
	}

	c.Start()
	log.Println("Cron-планировщик запущен.")
	return c
}

func CleanOldSchedules() {
	threshold := time.Now().Add(-24 * time.Hour)
	if err := storage.DB.Where("end_time < ?", threshold).Delete(&models.Schedule{}).Error; err != nil {
		log.Println("Ошибка при удалении устаревших расписаний:", err)
	} else {
		log.Println("Устаревшие расписания успешно удалены.")
	}
}

// CleanExpiredQueues удаляет из базы устаревшие очереди, у которых время закрытия прошло.
func CleanExpiredQueues() {
	now := time.Now()
	if err := storage.DB.Where("closes_at < ?", now).Delete(&models.Queue{}).Error; err != nil {
		log.Println("Ошибка при удалении устаревших очередей:", err)
	} else {
		log.Println("Устаревшие очереди успешно удалены.")
	}
}

// CloseExpiredQueues ищет активные очереди, у которых время закрытия истекло,
// обновляет их статус (IsActive = false) и отправляет уведомление через WebSocket.
func CloseExpiredQueues() {
	now := time.Now()
	var queues []models.Queue

	// Ищем очереди, которые активны и время закрытия уже наступило.
	if err := storage.DB.Where("is_active = ? AND closes_at <= ?", true, now).Find(&queues).Error; err != nil {
		log.Println("Ошибка при поиске очередей для закрытия:", err)
		return
	}

	// Если очередей для закрытия нет, можно завершить работу функции.
	if len(queues) == 0 {
		log.Println("Нет очередей для закрытия.")
		return
	}

	for _, q := range queues {
		// Обновляем статус очереди: помечаем как неактивную.
		q.IsActive = false
		if err := storage.DB.Save(&q).Error; err != nil {
			log.Println("Ошибка при закрытии очереди для schedule_id", q.ScheduleID, ":", err)
			continue
		}

		log.Printf("Очередь для schedule_id %d (queue_id %d) закрыта.\n", q.ScheduleID, q.ID)

		// Подготавливаем сообщение о закрытии очереди для WebSocket.
		payload := map[string]interface{}{
			"event_type": "queue_closed",
			"queue_id":   q.ID,
			"timestamp":  now.Unix(),
		}
		msg, err := json.Marshal(payload)
		if err != nil {
			log.Println("Ошибка сериализации сообщения о закрытии очереди:", err)
			continue
		}

		// Отправляем сообщение всем клиентам, подключённым к этой очереди.
		// Здесь мы используем q.ID в качестве идентификатора комнаты.
		ws.HubInstance.BroadcastMessage(ws.BroadcastMessage{
			QueueID: strconv.Itoa(int(q.ID)),
			Message: msg,
		})
	}
}
