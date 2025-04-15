package test

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"test_hack/internal/handlers"
	"test_hack/internal/models"
	"test_hack/internal/storage"
	"test_hack/internal/tasks"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func AuthMiddlewareTest() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Request.Header.Get("X-Test-UserID")
		if userIDStr == "" {
			// Значение по умолчанию
			c.Set("userID", uint(1))
		} else {
			// Попытка сконвертировать строку в число
			id, err := strconv.Atoi(userIDStr)
			if err != nil {
				c.Set("userID", uint(1))
			} else {
				c.Set("userID", uint(id))
			}
		}
		c.Next()
	}
}

func setupTestServer() *httptest.Server {
	key := os.Getenv("ENV_CHEK")
	if key == "" {
		fmt.Println("Подключение к .env")
		err := godotenv.Load("../.env")
		if err != nil {
			log.Fatal("Ошибка получения .env")
		}
	}

	storage.ConnectTestingDatabase()
	storage.DB.Exec("TRUNCATE TABLE users, schedules, queues, queue_entries RESTART IDENTITY CASCADE;")

	if err := storage.DB.AutoMigrate(&models.User{}, &models.Schedule{}, &models.Queue{}, &models.QueueEntry{}); err != nil {
		log.Fatal("Ошибка при миграции... ", err.Error())
	}

	storage.InitRedis()
	tasks.InitScheduler()

	go handlers.HubInstance.Run()

	r := gin.Default()

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", handlers.Login)
		authGroup.POST("/register", handlers.Register)
		authGroup.POST("/refresh", handlers.RefreshToken)
	}

	apiGroup := r.Group("")
	{
		apiGroup.GET("/groups", handlers.GetGroupsHandler)
		apiGroup.GET("/schedule", handlers.GetFullScheduleHandler)
	}

	r.GET("/api/queues/:id/status", handlers.GetQueueStatusHandler)
	queues := r.Group("/api/queues", AuthMiddlewareTest())
	{
		queues.POST("/:id/join", handlers.JoinQueueHandler)
		queues.POST("/:id/leave", handlers.LeaveQueueHandler)
		queues.GET("/:id/ws", handlers.QueueWebSocketHandler)
	}

	return httptest.NewServer(r)
}

func TestQueueFlow(t *testing.T) {
	// Настройка сервера
	ts := setupTestServer()
	defer ts.Close()

	log.Println("Тест: очистка базы перед запуском теста")
	// (База очищается в setupTestServer())

	// 1. Создаем тестовое расписание и очередь вручную.
	now := time.Now()
	testSchedule := models.Schedule{
		ExternalID: "9999",
		Name:       "Тестовая пара",
		StartTime:  now.Add(2 * time.Minute), // через 2 минуты от текущего времени
		EndTime:    now.Add(3 * time.Minute),
		GroupIDs:   "1,2",
	}
	err := storage.DB.Create(&testSchedule).Error
	assert.NoError(t, err, "Ошибка создания тестового расписания")
	log.Println("Тестовое расписание создано, ID:", testSchedule.ID)

	testQueue := models.Queue{
		ScheduleID: testSchedule.ID,
		OpensAt:    now, // очередь открыта
		ClosesAt:   testSchedule.StartTime,
		IsActive:   true,
	}
	err = storage.DB.Create(&testQueue).Error
	assert.NoError(t, err, "Ошибка создания тестовой очереди")
	log.Println("Тестовая очередь создана, ID:", testQueue.ID)

	// 2. Регистрируем двух тестовых пользователей с уникальными email.
	user1Email := fmt.Sprintf("ivan_%d@example.com", time.Now().UnixNano())
	user2Email := fmt.Sprintf("petr_%d@example.com", time.Now().UnixNano())
	user1 := models.User{Name: "Иван", Surname: "Иванов", Email: user1Email, PasswordHash: "hashed123"}
	user2 := models.User{Name: "Петр", Surname: "Петров", Email: user2Email, PasswordHash: "hashed456"}
	err = storage.DB.Create(&user1).Error
	assert.NoError(t, err, "Ошибка создания пользователя 1")
	err = storage.DB.Create(&user2).Error
	assert.NoError(t, err, "Ошибка создания пользователя 2")
	log.Println("Тестовые пользователи созданы, ID1:", user1.ID, "ID2:", user2.ID)

	// 3. Симулируем вход пользователей в очередь через HTTP запрос join.
	joinURL := ts.URL + "/api/queues/" + strconv.Itoa(int(testQueue.ID)) + "/join"

	log.Println("Отправка запроса join для пользователя 1")
	req1, _ := http.NewRequest("POST", joinURL, nil)
	req1.Header.Set("X-Test-UserID", fmt.Sprintf("%d", user1.ID))
	res1, err := http.DefaultClient.Do(req1)
	assert.NoError(t, err, "Ошибка запроса join для пользователя 1")
	defer res1.Body.Close()
	assert.Equal(t, http.StatusOK, res1.StatusCode, "Пользователь 1 не смог присоединиться к очереди")

	log.Println("Отправка запроса join для пользователя 2")
	req2, _ := http.NewRequest("POST", joinURL, nil)
	req2.Header.Set("X-Test-UserID", fmt.Sprintf("%d", user2.ID))
	res2, err := http.DefaultClient.Do(req2)
	assert.NoError(t, err, "Ошибка запроса join для пользователя 2")
	defer res2.Body.Close()
	assert.Equal(t, http.StatusOK, res2.StatusCode, "Пользователь 2 не смог присоединиться к очереди")

	// 4. Проверка состояния очереди через HTTP GET /api/queues/:id/status
	statusURL := ts.URL + "/api/queues/" + strconv.Itoa(int(testQueue.ID)) + "/status"
	statusReq, _ := http.NewRequest("GET", statusURL, nil)
	// Для тестовой авторизации можно установить X-Test-UserID, если требуется
	statusReq.Header.Set("X-Test-UserID", fmt.Sprintf("%d", user1.ID))
	statusRes, err := http.DefaultClient.Do(statusReq)
	assert.NoError(t, err, "Ошибка запроса статуса очереди")
	defer statusRes.Body.Close()
	assert.Equal(t, http.StatusOK, statusRes.StatusCode, "Ошибка получения статуса очереди")

	var statusResponse map[string]interface{}
	json.NewDecoder(statusRes.Body).Decode(&statusResponse)
	log.Println("Статус очереди получен:", statusResponse)
	// Если возвращается агрегированная структура с участниками, можно проверить наличие двух участников.
	participantsData, exists := statusResponse["participants"]
	assert.True(t, exists, "В ответе статуса очереди отсутствует поле participants")
	participants := participantsData.([]interface{})
	assert.Equal(t, 2, len(participants), "Количество участников в очереди неверное")

	// 5. Тестируем WS-соединение для очереди.
	wsURL := "ws" + ts.URL[4:] + "/api/queues/" + strconv.Itoa(int(testQueue.ID)) + "/ws"
	dialer := websocket.Dialer{}
	wsHeaders := http.Header{}
	wsHeaders.Set("X-Test-UserID", fmt.Sprintf("%d", user1.ID))
	wsConn, _, err := dialer.Dial(wsURL, wsHeaders)
	assert.NoError(t, err, "Ошибка подключения к WS")
	defer wsConn.Close()

	// 6. Читаем одно или несколько WS-сообщений, связанных с join/обновлением очереди.
	_, wsMessage, err := wsConn.ReadMessage()
	assert.NoError(t, err, "Ошибка чтения WS сообщения")
	var wsMsg map[string]interface{}
	err = json.Unmarshal(wsMessage, &wsMsg)
	assert.NoError(t, err, "Ошибка разбора WS сообщения")
	log.Println("Получено WS сообщение:", wsMsg)
	assert.Contains(t, wsMsg, "event_type", "WS сообщение не содержит поле event_type")

	// 7. Симулируем автоматическое закрытие очереди:
	log.Println("Симуляция закрытия очереди: обновляем closes_at на прошлое время")
	storage.DB.Model(&models.Queue{}).Where("id = ?", testQueue.ID).Update("closes_at", time.Now().Add(-1*time.Minute))
	tasks.CloseExpiredQueues()

	// WS: ожидаем сообщение с event_type "queue_closed"
	_, msgClosed, err := wsConn.ReadMessage()
	assert.NoError(t, err, "Ошибка чтения WS сообщения (queue_closed)")
	var closedMsg map[string]interface{}
	err = json.Unmarshal(msgClosed, &closedMsg)
	assert.NoError(t, err, "Ошибка разбора WS сообщения (queue_closed)")
	assert.Equal(t, "queue_closed", closedMsg["event_type"], "Неверный тип WS сообщения после закрытия очереди")
	log.Println("Получено WS сообщение о закрытии очереди:", closedMsg)
}
