package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"test_hack/internal/models"
	"test_hack/internal/storage"

	"github.com/gin-gonic/gin"
)

const customTimeLayout = "2006-01-02T15:04:05"

// ScheduleEvent и ScheduleResponse – структуры для внешнего API (оставляем их без изменений).
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

// ScheduleWithQueue – агрегированная структура для ответа
type ScheduleWithQueue struct {
	Schedule models.Schedule `json:"schedule"`
	Queue    *models.Queue   `json:"queue,omitempty"`
}

var scheduleCtx = context.Background()

// GetScheduleHandler получает расписание с внешнего API
// @Summary		Получение расписания
// @Description	Получает расписание по заданным параметрам (start, end, group_id), кэширует результат в Redis
// @Tags			schedule
// @Accept			json
// @Produce		json
// // @Param			start	query		string	false	"Дата начала"
// // @Param			end	query		string	false	"Дата окончания"
// @Param			group_id	query		string	true	"ID группы"
// @Success		200		{object}	ScheduleResponse	"Успешный ответ с данными расписания"
// @Failure		400		{object}	response.ErrorResponse	"Ошибка валидации данных"
// @Failure		500		{object}	response.ErrorResponse	"Ошибка сервера"
// @Router			/schedule [get]
func GetFullScheduleHandler(c *gin.Context) {
	groupIDStr := c.Query("group_id")
	if groupIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать group_id"})
		return
	}

	// Здесь можно позволить передавать start и end через query, но по умолчанию используем период сегодня - сегодня+7 дней.
	now := time.Now()
	startTime := now
	endTime := now.AddDate(0, 0, 7)

	// Создаем уникальный ключ для кэширования пустого результата
	cacheKeyEmpty := "no_events:" + groupIDStr + ":" + startTime.Format("2006-01-02") + ":" + endTime.Format("2006-01-02")
	redisClient := storage.RedisClient

	// Проверка кэша: если установлен маркер "нет событий", сразу возвращаем пустой массив.
	if val, err := redisClient.Get(scheduleCtx, cacheKeyEmpty).Result(); err == nil && val == "true" {
		c.JSON(http.StatusOK, gin.H{"message": "Нет событий на выбранный период", "data": []ScheduleWithQueue{}})
		return
	}

	// Попытка извлечь расписание из БД
	var schedules []models.Schedule
	pattern := "%" + groupIDStr + "%"
	if err := storage.DB.
		Where("start_time BETWEEN ? AND ? AND group_ids LIKE ?", startTime, endTime, pattern).
		Find(&schedules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка поиска расписания в БД"})
		return
	}

	// Если в БД расписание не найдено – вызываем внешний API
	if len(schedules) == 0 {
		apiURL := "https://api.profcomff.com/timetable/event/?start=" +
			startTime.Format("2006-01-02") + "&end=" + endTime.Format("2006-01-02") + "&group_id=" + groupIDStr
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

		var externalResp ScheduleResponse
		if err := json.Unmarshal(body, &externalResp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка декодирования данных расписания"})
			return
		}

		// Если внешний API возвращает пустой список, запоминаем это в Redis, чтобы не дергать его повторно.
		if externalResp.Total == 0 || len(externalResp.Items) == 0 {
			redisClient.Set(scheduleCtx, cacheKeyEmpty, "true", 15*time.Minute)
			c.JSON(http.StatusOK, gin.H{"message": "Нет событий на выбранный период", "data": []ScheduleWithQueue{}})
			return
		}

		// Обрабатываем каждый элемент из внешнего API:
		for _, event := range externalResp.Items {
			startT, err1 := time.Parse(customTimeLayout, event.StartTS)
			endT, err2 := time.Parse(customTimeLayout, event.EndTS)
			if err1 != nil || err2 != nil {
				continue
			}

			var groupIDs []string
			for _, grp := range event.Group {
				groupIDs = append(groupIDs, strconv.Itoa(grp.ID))
			}
			groupsJoined := strings.Join(groupIDs, ",")

			// Проверяем, существует ли событие с таким ExternalID.
			var existing models.Schedule
			if err := storage.DB.Where("external_id = ?", strconv.Itoa(event.ID)).First(&existing).Error; err == nil {
				continue
			}

			newEvent := models.Schedule{
				ExternalID: strconv.Itoa(event.ID),
				Name:       event.Name,
				StartTime:  startT,
				EndTime:    endT,
				GroupIDs:   groupsJoined,
			}

			if err := storage.DB.Create(&newEvent).Error; err != nil {
				continue
			}
			schedules = append(schedules, newEvent)
		}
	}

	// Если после загрузки все равно расписание пусто, возвращаем пустой результат
	if len(schedules) == 0 {
		redisClient.Set(scheduleCtx, cacheKeyEmpty, "true", 15*time.Minute)
		c.JSON(http.StatusOK, gin.H{"message": "Нет событий на выбранный период", "data": []ScheduleWithQueue{}})
		return
	}

	// Получаем список schedule_id из найденных расписаний
	var scheduleIDs []uint
	for _, s := range schedules {
		scheduleIDs = append(scheduleIDs, s.ID)
	}
	// Получаем очереди для этих расписаний
	var queues []models.Queue
	storage.DB.Where("schedule_id IN ?", scheduleIDs).Find(&queues)

	// Строим мапу scheduleID -> Queue
	queueMap := make(map[uint]models.Queue)
	for _, q := range queues {
		queueMap[q.ScheduleID] = q
	}

	var results []ScheduleWithQueue
	for _, s := range schedules {
		var q *models.Queue = nil
		if found, ok := queueMap[s.ID]; ok {
			q = &found
		}
		results = append(results, ScheduleWithQueue{
			Schedule: s,
			Queue:    q,
		})
	}

	c.JSON(http.StatusOK, results)
}
