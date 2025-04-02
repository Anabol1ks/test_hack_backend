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

const customTimeLayout = "2006-01-02T15:04:05"

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
	groupIDStr := c.Query("group_id")
	if groupIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать group_id"})
		return
	}
	// Опционально, можно передавать start и end через query, но мы зададим их по умолчанию: сегодня - сегодня+7 дней.
	now := time.Now()
	startTime := now
	endTime := now.AddDate(0, 0, 7)

	// Поиск в БД по расписанию для заданного периода, где GroupIDs содержит искомый group_id.
	var schedules []models.Schedule
	// Используем LIKE, например, если groupIDStr="67", то ищем "%67%"
	pattern := "%" + groupIDStr + "%"
	if err := storage.DB.
		Where("start_time BETWEEN ? AND ? AND group_ids LIKE ?", startTime, endTime, pattern).
		Find(&schedules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка поиска расписания в БД"})
		return
	}

	if len(schedules) > 0 {
		// Возвращаем найденные данные
		c.JSON(http.StatusOK, schedules)
		return
	}

	// Если данных нет, вызываем внешний API для загрузки расписания для этой группы.
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

	// Для каждого события внешнего API:
	// 1. Преобразуем строки с датами в time.Time.
	// 2. Собираем список GroupIDs (из массива group, через запятую).
	// 3. Если событие с таким ExternalID еще не существует в БД, сохраняем его.
	var inserted []models.Schedule
	for _, event := range externalResp.Items {
		// Преобразуем время (предполагается формат ISO8601)
		startT, err1 := time.Parse(customTimeLayout, event.StartTS)
		endT, err2 := time.Parse(customTimeLayout, event.EndTS)
		if err1 != nil || err2 != nil {
			// Если ошибка парсинга, пропускаем событие
			continue
		}

		// Собираем список GroupIDs из event.Group
		var groupIDs []string
		for _, grp := range event.Group {
			// Преобразуем int в строку
			groupIDs = append(groupIDs, strconv.Itoa(grp.ID))
		}
		groupsJoined := strings.Join(groupIDs, ",")

		// Проверяем наличие события с таким ExternalID
		var existing models.Schedule
		err := storage.DB.Where("external_id = ?", strconv.Itoa(event.ID)).First(&existing).Error
		if err == nil {
			// Событие уже есть, пропускаем его
			continue
		}

		// Создаем новое событие
		newEvent := models.Schedule{
			ExternalID: strconv.Itoa(event.ID),
			Name:       event.Name,
			StartTime:  startT,
			EndTime:    endT,
			GroupIDs:   groupsJoined,
		}

		if err := storage.DB.Create(&newEvent).Error; err != nil {
			// Если ошибка при сохранении, можно залогировать и продолжить
			continue
		}
		inserted = append(inserted, newEvent)
	}

	// Если после загрузки ничего не вставлено, возвращаем ошибку или пустой список
	if len(inserted) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Нет новых событий", "data": schedules})
		return
	}

	// После вставки возвращаем данные (можно комбинировать старые и новые, если они есть)
	schedules = append(schedules, inserted...)
	c.JSON(http.StatusOK, schedules)
}
