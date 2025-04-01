package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"test_hack/internal/storage"

	"github.com/gin-gonic/gin"
)

type ScheduleEvent struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Room []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Building    string `json:"building"`
		BuildingURL string `json:"building_url"`
		Direction   string `json:"direction"`
	} `json:"room"`
	Group []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Number string `json:"number"`
	} `json:"group"`
	Lecturer []struct {
		ID          int    `json:"id"`
		FirstName   string `json:"first_name"`
		MiddleName  string `json:"middle_name"`
		LastName    string `json:"last_name"`
		AvatarID    int    `json:"avatar_id"`
		AvatarLink  string `json:"avatar_link"`
		Description string `json:"description"`
	} `json:"lecturer"`
	StartTS string `json:"start_ts"`
	EndTS   string `json:"end_ts"`
}

type ScheduleResponse struct {
	Items  []ScheduleEvent `json:"items"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
	Total  int             `json:"total"`
}

var scheduleCtx = context.Background()

// GetScheduleHandler получает расписание с внешнего API
// @Summary		Получение расписания
// @Description	Получает расписание по заданным параметрам (start, end, group_id), кэширует результат в Redis
// @Tags			schedule
// @Accept			json
// @Produce		json
// @Param			start	query		string	true	"Дата начала"
// @Param			end	query		string	true	"Дата окончания"
// @Param			group_id	query		string	true	"ID группы"
// @Success		200		{object}	ScheduleResponse	"Успешный ответ с данными расписания"
// @Failure		400		{object}	response.ErrorResponse	"Ошибка валидации данных"
// @Failure		500		{object}	response.ErrorResponse	"Ошибка сервера"
// @Router			/schedule [get]
func GetScheduleHandler(c *gin.Context) {
	start := c.Query("start")
	end := c.Query("end")
	groupID := c.Query("group_id")
	if start == "" || end == "" || groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать start, end и group_id"})
		return
	}

	cacheKey := "schedule_" + start + "_" + end + "_" + groupID
	redisClient := storage.RedisClient

	// Проверка кэша
	cached, err := redisClient.Get(scheduleCtx, cacheKey).Result()
	if err == nil && cached != "" {
		var schedule ScheduleResponse
		if err := json.Unmarshal([]byte(cached), &schedule); err == nil {
			c.JSON(http.StatusOK, schedule)
			return
		}
	}

	// Формирование URL запроса к внешнему API
	apiURL := "https://api.profcomff.com/timetable/event/?start=" + start + "&end=" + end + "&group_id=" + groupID
	resp, err := http.Get(apiURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить данные расписания"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа внешнего API"})
		return
	}

	var schedule ScheduleResponse
	if err := json.Unmarshal(body, &schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка декодирования данных расписания"})
		return
	}

	// Кэширование результата на 1 час
	redisClient.Set(scheduleCtx, cacheKey, string(body), time.Hour)

	c.JSON(http.StatusOK, schedule)
}
