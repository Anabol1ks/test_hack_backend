package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"test_hack/internal/response"
	"test_hack/internal/storage"

	"github.com/gin-gonic/gin"
)

// Структуры для декодирования ответа API
type Group struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

type GroupResponse struct {
	Items  []Group `json:"items"`
	Limit  int     `json:"limit"`
	Offset int     `json:"offset"`
	Total  int     `json:"total"`
}

var ctx = context.Background()

// GetGroupsHandler обрабатывает запрос на получение списка групп
// @Summary		Получение списка групп
// @Description	Получает список всех групп, кэширует результат в Redis
// @Tags			groups
// @Accept			json
// @Produce		json
// @Success		200		{object}	GroupResponse	"Успешный ответ с данными групп"
// @Failure		500		{object}	response.ErrorResponse	"Ошибка сервера (API_ERROR, CACHE_ERROR, DECODE_ERROR)"
// @Router			/groups [get]
func GetGroupsHandler(c *gin.Context) {
	cacheKey := "groups_all"
	redisClient := storage.RedisClient // предполагается, что клиент Redis инициализирован в storage

	// Проверка кэша
	cached, err := redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cached != "" {
		var groups GroupResponse
		if err := json.Unmarshal([]byte(cached), &groups); err == nil {
			c.JSON(http.StatusOK, groups)
			return
		}
	}

	// Запрос к внешнему API
	apiURL := "https://api.profcomff.com/timetable/group/?limit=1000"
	resp, err := http.Get(apiURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "API_ERROR",
			Message: "Не удалось получить данные групп",
			Details: err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "API_ERROR",
			Message: "Ошибка чтения ответа внешнего API",
			Details: err.Error(),
		})
		return
	}

	var groups GroupResponse
	if err := json.Unmarshal(body, &groups); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DECODE_ERROR",
			Message: "Ошибка декодирования данных групп",
			Details: err.Error(),
		})
		return
	}

	// Кэширование результата на 6 часов
	redisClient.Set(ctx, cacheKey, string(body), time.Hour*6)

	c.JSON(http.StatusOK, groups)
}
