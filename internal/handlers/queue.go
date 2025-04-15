package handlers

import (
	"net/http"
	"strconv"
	"test_hack/internal/models"
	"test_hack/internal/response"
	"test_hack/internal/storage"
	"time"

	"github.com/gin-gonic/gin"
)

// JoinQueueHandler обрабатывает запрос на вступление в очередь
// @Summary		Вступление в очередь
// @Description	Добавляет пользователя в очередь и уведомляет других участников
// @Tags			queue
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"ID очереди"
// @Security		BearerAuth
// @Success		200	{object}	response.MessageResponse	"Успешное вступление в очередь с указанием позиции"
// @Failure		400	{object}	response.ErrorResponse	"Ошибка валидации (INVALID_QUEUE_ID, ALREADY_IN_QUEUE, QUEUE_INACTIVE)"
// @Failure		404	{object}	response.ErrorResponse	"Очередь не найдена (QUEUE_NOT_FOUND)"
// @Failure		500	{object}	response.ErrorResponse	"Ошибка сервера (DB_ERROR)"
// @Router			/api/queues/{id}/join [post]
func JoinQueueHandler(c *gin.Context) {
	queueIDStr := c.Param("id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "INVALID_QUEUE_ID",
			Message: "Неверный идентификатор очереди",
		})
		return
	}

	userID := c.GetUint("userID")
	var existingEntry models.QueueEntry
	if err := storage.DB.Where("user_id = ? AND queue_id = ? AND exited_at IS NULL", userID, queueID).First(&existingEntry).Error; err == nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "ALREADY_IN_QUEUE",
			Message: "Пользователь уже состоит в этой очереди",
		})
		return
	}

	var queue models.Queue
	if err := storage.DB.First(&queue, queueID).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{
			Code:    "QUEUE_NOT_FOUND",
			Message: "Очередь не найдена",
		})
		return
	}

	now := time.Now()
	// Проверяем, что очередь активна: открыта и время не вышло (между OpensAt и ClosesAt)
	if now.Before(queue.OpensAt) || now.After(queue.ClosesAt) || !queue.IsActive {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "QUEUE_INACTIVE",
			Message: "Очередь не активна",
		})
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
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Ошибка добаления в очередь",
			Details: err.Error(),
		})
		return
	}

	HubInstance.BroadcastWSMessage(WSMessage{
		EventType: "user_joined",
		QueueID:   queueIDStr,
		Data: map[string]interface{}{
			"user_id":  userID,
			"position": newPosition,
		},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Вступление в очередь прошла успешно", "position": newPosition})
}

// LeaveQueueHandler обрабатывает запрос на выход из очереди
// @Summary		Выход из очереди
// @Description	Удаляет пользователя из очереди и уведомляет других участников
// @Tags			queue
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"ID очереди"
// @Security		BearerAuth
// @Success		200	{object}	response.SuccessResponse	"Успешный выход из очереди"
// @Failure		400	{object}	response.ErrorResponse	"Ошибка валидации (INVALID_QUEUE_ID, NOT_IN_QUEUE)"
// @Failure		500	{object}	response.ErrorResponse	"Ошибка сервера (DB_ERROR)"
// @Router			/api/queues/{id}/leave [post]
func LeaveQueueHandler(c *gin.Context) {
	queueIDStr := c.Param("id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "INVALID_QUEUE_ID",
			Message: "Неверный идентификатор очереди",
		})
		return
	}

	userID := c.GetUint("userID")

	var entry models.QueueEntry
	if err := storage.DB.
		Where("user_id = ? AND queue_id = ? AND exited_at IS NULL", userID, queueID).
		First(&entry).Error; err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "NOT_IN_QUEUE",
			Message: "Активная запись в очереди не найдена",
		})
		return
	}
	now := time.Now()
	entry.ExitedAt = &now
	if err := storage.DB.Save(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Ошибка при выходе из очереди",
			Details: err.Error(),
		})
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
	HubInstance.BroadcastWSMessage(WSMessage{
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

// GetQueueStatusHandler обрабатывает запрос на получение статуса очереди
// @Summary		Получение статуса очереди
// @Description	Возвращает информацию о состоянии очереди и списке участников
// @Tags			queue
// @Accept			json
// @Produce		json
// @Param			id	path		string	true	"ID очереди"
// @Security		BearerAuth
// @Success		200	{object}	response.SwaggerQueueStatusResponse	"Успешное получение статуса очереди"
// @Failure		400	{object}	response.ErrorResponse	"Ошибка валидации (INVALID_QUEUE_ID)"
// @Failure		404	{object}	response.ErrorResponse	"Очередь не найдена (QUEUE_NOT_FOUND)"
// @Failure		500	{object}	response.ErrorResponse	"Ошибка сервера (DB_ERROR)"
// @Router			/api/queues/{id}/status [get]
func GetQueueStatusHandler(c *gin.Context) {
	// Извлекаем queueID из параметров URL
	queueIDStr := c.Param("id")
	queueID, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "INVALID_QUEUE_ID",
			Message: "Неверный идентификатор очереди",
		})
		return
	}

	// Загружаем очередь по ID
	var queue models.Queue
	if err := storage.DB.First(&queue, queueID).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{
			Code:    "QUEUE_NOT_FOUND",
			Message: "Очередь не найдена",
		})
		return
	}

	// Загружаем записи участников очереди, где exited_at is null, упорядоченные по position
	var entries []models.QueueEntry
	if err := storage.DB.
		Preload("User").
		Where("queue_id = ? AND exited_at IS NULL", queueID).
		Order("position ASC").
		Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Ошибка загрузки записей очереди",
			Details: err.Error(),
		})
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
