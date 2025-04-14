package queue

import (
	"net/http"
	"strconv"
	"test_hack/internal/models"
	"test_hack/internal/storage"
	"test_hack/internal/ws"
	"time"

	"github.com/gin-gonic/gin"
)

func JoinQueueHandler(c *gin.Context) {
	queueIDStr := c.Param("id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный идентификатор очереди"})
		return
	}

	userID := c.GetUint("userID")
	var existingEntry models.QueueEntry
	if err := storage.DB.Where("user_id = ? AND queue_id = ? AND exited_at IS NULL", userID, queueID).First(&existingEntry).Error; err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пользователь уже состоит в этой очереди"})
		return
	}

	var queue models.Queue
	if err := storage.DB.First(&queue, queueID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Очередь не найдена"})
		return
	}

	now := time.Now()
	// Проверяем, что очередь активна: открыта и время не вышло (между OpensAt и ClosesAt)
	if now.Before(queue.OpensAt) || now.After(queue.ClosesAt) || !queue.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Очередь не активна"})
		return
	}

	var maxPosition int
	row := storage.DB.Model(&models.QueueEntry{}).Where("queue_id = ? AND exited_at IS NULL", queueID).Select("COALESCE(MAX(position),0)").Row()
	_ = row.Scan(&maxPosition)
	newPosition := maxPosition + 1

	entry := models.QueueEntry{
		UserID:   userID,
		QueueID:  uint(queueID),
		Position: newPosition,
		ExitedAt: nil,
	}

	if err := storage.DB.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка добаления в очередь"})
		return
	}

	ws.HubInstance.BroadcastWSMessage(ws.WSMessage{
		EventType: "user_joined",
		QueueID:   queueIDStr,
		Data: map[string]interface{}{
			"user_id":  userID,
			"position": newPosition,
		},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Вступление в очередь прошла успешно", "position": newPosition})
}

func LeaveQueueHandler(c *gin.Context) {
	queueIDStr := c.Param("id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный идентификатор очереди"})
		return
	}

	userID := c.GetUint("userID")

	var entry models.QueueEntry
	if err := storage.DB.
		Where("user_id = ? AND queue_id = ? AND exited_at IS NULL", userID, queueID).
		First(&entry).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Активная запись в очереди не найдена"})
		return
	}
	now := time.Now()
	entry.ExitedAt = &now
	if err := storage.DB.Save(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при выходе из очереди"})
		return
	}

	// Пересчитываем позиции для всех участников очереди, чья позиция больше, чем у вышедшего.
	var entries []models.QueueEntry
	if err := storage.DB.
		Where("queue_id = ? AND exited_at IS NULL AND position > ?", queueID, entry.Position).
		Find(&entries).Error; err == nil {
		for _, e := range entries {
			e.Position -= 1
			storage.DB.Save(&e)
		}
	}

	// Готовим сообщение для рассылки через WebSocket.
	ws.HubInstance.BroadcastWSMessage(ws.WSMessage{
		EventType: "user_left",
		QueueID:   queueIDStr,
		Data: map[string]interface{}{
			"user_id":       userID,
			"left_position": entry.Position,
		},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Вы успешно вышли из очереди"})
}

type Participant struct {
	UserID   uint   `json:"user_id"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Position int    `json:"position"`
}

// QueueStatusResponse содержит статус очереди и список участников.
type QueueStatusResponse struct {
	QueueID      uint          `json:"queue_id"`
	ScheduleID   uint          `json:"schedule_id"`
	IsActive     bool          `json:"is_active"`
	OpensAt      time.Time     `json:"opens_at"`
	ClosesAt     time.Time     `json:"closes_at"`
	Participants []Participant `json:"participants"`
}

func GetQueueStatusHandler(c *gin.Context) {
	// Извлекаем queueID из параметров URL
	queueIDStr := c.Param("id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный идентификатор очереди"})
		return
	}

	// Загружаем очередь по ID
	var queue models.Queue
	if err := storage.DB.First(&queue, queueID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Очередь не найдена"})
		return
	}

	// Загружаем записи участников очереди, где exited_at is null, упорядоченные по position
	var entries []models.QueueEntry
	if err := storage.DB.
		Preload("User").
		Where("queue_id = ? AND exited_at IS NULL", queueID).
		Order("position ASC").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки записей очереди"})
		return
	}

	// Формируем список участников с нужными полями (имя и фамилия)
	participants := make([]Participant, 0, len(entries))
	for _, entry := range entries {
		participant := Participant{
			UserID:   entry.UserID,
			Name:     entry.User.Name,
			Surname:  entry.User.Surname,
			Position: entry.Position,
		}
		participants = append(participants, participant)
	}

	response := QueueStatusResponse{
		QueueID:      queue.ID,
		ScheduleID:   queue.ScheduleID,
		IsActive:     queue.IsActive,
		OpensAt:      queue.OpensAt,
		ClosesAt:     queue.ClosesAt,
		Participants: participants,
	}

	c.JSON(http.StatusOK, response)
}
