package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"test_hack/internal/models"
	"test_hack/internal/response"
	"test_hack/internal/storage"
	"time"

	"github.com/gin-gonic/gin"
)

// UserQueueItem represents a single queue entry with all required details
type UserQueueItem struct {
	QueueID      uint     `json:"queue_id"`
	Position     int      `json:"position"`
	ScheduleID   uint     `json:"schedule_id"`
	ScheduleName string   `json:"schedule_name"`
	StartTime    string   `json:"start_time"`
	EndTime      string   `json:"end_time"`
	GroupNumbers []string `json:"group_numbers"`
	OpensAt      string   `json:"opens_at"`
	ClosesAt     string   `json:"closes_at"`
	IsActive     bool     `json:"is_active"`
}

// GetUserQueuesHandler godoc
// @Summary		Получение списка своих очередей
// @Description	Получение списка очередей, в которых пользователь участвует
// @Tags			profile
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Success		200	{array}		UserQueueItem	"List of queues the user is part of"
// @Failure		500	{object}	response.ErrorResponse	"Server error (DB_ERROR)"
// @Router			/profile/queues [get]
func GetUserQueuesHandler(c *gin.Context) {
	userID := c.GetUint("userID")

	// Get all active queue entries for the user
	var queueEntries []models.QueueEntry
	if err := storage.DB.
		Where("user_id = ? AND exited_at IS NULL", userID).
		Find(&queueEntries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Error fetching user queue entries",
			Details: err.Error(),
		})
		return
	}

	if len(queueEntries) == 0 {
		c.JSON(http.StatusOK, []UserQueueItem{})
		return
	}

	// Extract queue IDs
	var queueIDs []uint
	for _, entry := range queueEntries {
		queueIDs = append(queueIDs, entry.QueueID)
	}

	// Get queue details
	var queues []models.Queue
	if err := storage.DB.
		Where("id IN ?", queueIDs).
		Find(&queues).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Error fetching queue details",
			Details: err.Error(),
		})
		return
	}

	// Create a map for quick lookup
	queueMap := make(map[uint]models.Queue)
	var scheduleIDs []uint
	for _, q := range queues {
		queueMap[q.ID] = q
		scheduleIDs = append(scheduleIDs, q.ScheduleID)
	}

	// Get schedule details
	var schedules []models.Schedule
	if err := storage.DB.
		Where("id IN ?", scheduleIDs).
		Find(&schedules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Error fetching schedule details",
			Details: err.Error(),
		})
		return
	}

	// Create a map for schedule lookup
	scheduleMap := make(map[uint]models.Schedule)
	for _, s := range schedules {
		scheduleMap[s.ID] = s
	}

	// Get group details for all schedules
	var allGroupIDs []string
	for _, s := range schedules {
		groupIDs := strings.Split(s.GroupIDs, ",")
		allGroupIDs = append(allGroupIDs, groupIDs...)
	}

	// Fetch group information from Redis or external API
	// This is a simplified approach - in a real implementation, you might want to
	// cache group information or fetch it from the external API
	groupMap := make(map[string]string) // Map group ID to group number
	cacheKey := "groups_all"
	cached, err := storage.RedisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var groupResponse GroupResponse
		if err := json.Unmarshal([]byte(cached), &groupResponse); err == nil {
			for _, group := range groupResponse.Items {
				groupMap[strconv.Itoa(group.ID)] = group.Number
			}
		}
	}

	// Build response
	var result []UserQueueItem
	for _, entry := range queueEntries {
		queue, queueExists := queueMap[entry.QueueID]
		if !queueExists {
			continue
		}

		schedule, scheduleExists := scheduleMap[queue.ScheduleID]
		if !scheduleExists {
			continue
		}

		// Extract group numbers
		var groupNumbers []string
		groupIDs := strings.Split(schedule.GroupIDs, ",")
		for _, gID := range groupIDs {
			if number, exists := groupMap[gID]; exists {
				groupNumbers = append(groupNumbers, number)
			} else {
				groupNumbers = append(groupNumbers, gID) // Fallback to ID if number not found
			}
		}

		item := UserQueueItem{
			QueueID:      queue.ID,
			Position:     entry.Position,
			ScheduleID:   schedule.ID,
			ScheduleName: schedule.Name,
			StartTime:    schedule.StartTime.Format(time.RFC3339),
			EndTime:      schedule.EndTime.Format(time.RFC3339),
			GroupNumbers: groupNumbers,
			OpensAt:      queue.OpensAt.Format(time.RFC3339),
			ClosesAt:     queue.ClosesAt.Format(time.RFC3339),
			IsActive:     queue.IsActive,
		}

		result = append(result, item)
	}

	c.JSON(http.StatusOK, result)
}
