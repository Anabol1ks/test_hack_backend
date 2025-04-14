package response

import "time"

// SuccessResponse представляет успешный ответ API
type SuccessResponse struct {
	Message string `json:"message" example:"Операция успешно выполнена"`
}

type MessageResponse struct {
	Message string `json:"message" example:"Операция успешно выполнена"`
}

// ErrorResponse представляет ответ с ошибкой API
type ErrorResponse struct {
	// Код ошибки для программной обработки
	// example: VALIDATION_ERROR
	Code string `json:"code"`

	// Человекочитаемое сообщение об ошибке
	// example: Ошибка валидации данных
	Message string `json:"message"`

	// Дополнительные детали об ошибке (опционально)
	// example: поле email должно быть валидным email адресом
	Details string `json:"details,omitempty"`
}

// TokenResponse представляет ответ с токенами авторизации
type TokenResponse struct {
	// JWT токен для доступа к защищенным эндпоинтам
	// example: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
	AccessToken string `json:"access_token"`

	// JWT токен для обновления access токена
	// example: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
	RefreshToken string `json:"refresh_token"`
}

// Swagger-friendly model definitions
// These are simplified versions of the models for Swagger documentation

// SwaggerUser представляет модель пользователя для Swagger
type SwaggerUser struct {
	ID        uint      `json:"id" example:"1"`
	Name      string    `json:"name" example:"Иван"`
	Surname   string    `json:"surname" example:"Иванов"`
	Email     string    `json:"email" example:"ivan@example.com"`
	CreatedAt time.Time `json:"created_at" example:"2023-01-01T12:00:00Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2023-01-01T12:00:00Z"`
}

// SwaggerSchedule представляет модель расписания для Swagger
type SwaggerSchedule struct {
	ID         uint      `json:"id" example:"1"`
	ExternalID string    `json:"external_id" example:"123456"`
	Name       string    `json:"name" example:"Практика по программированию"`
	StartTime  time.Time `json:"start_time" example:"2023-01-01T10:00:00Z"`
	EndTime    time.Time `json:"end_time" example:"2023-01-01T12:00:00Z"`
	GroupIDs   string    `json:"group_ids" example:"67,203,111"`
	CreatedAt  time.Time `json:"created_at" example:"2023-01-01T09:00:00Z"`
	UpdatedAt  time.Time `json:"updated_at" example:"2023-01-01T09:00:00Z"`
}

// SwaggerQueue представляет модель очереди для Swagger
type SwaggerQueue struct {
	ID              uint      `json:"id" example:"1"`
	ScheduleID      uint      `json:"schedule_id" example:"1"`
	OpensAt         time.Time `json:"opens_at" example:"2023-01-01T09:00:00Z"`
	ClosesAt        time.Time `json:"closes_at" example:"2023-01-01T10:00:00Z"`
	IsActive        bool      `json:"is_active" example:"true"`
	MaxParticipants int       `json:"max_participants,omitempty" example:"30"`
	CreatedAt       time.Time `json:"created_at" example:"2023-01-01T08:00:00Z"`
	UpdatedAt       time.Time `json:"updated_at" example:"2023-01-01T08:00:00Z"`
}

// SwaggerScheduleWithQueue представляет модель расписания с очередью для Swagger
type SwaggerScheduleWithQueue struct {
	Schedule SwaggerSchedule `json:"schedule"`
	Queue    *SwaggerQueue   `json:"queue,omitempty"`
}

// SwaggerParticipant представляет участника очереди для Swagger
type SwaggerParticipant struct {
	UserID   uint   `json:"user_id" example:"1"`
	Name     string `json:"name" example:"Иван"`
	Surname  string `json:"surname" example:"Иванов"`
	Position int    `json:"position" example:"1"`
}

// SwaggerQueueStatusResponse представляет статус очереди для Swagger
type SwaggerQueueStatusResponse struct {
	QueueID      uint                 `json:"queue_id" example:"1"`
	ScheduleID   uint                 `json:"schedule_id" example:"1"`
	IsActive     bool                 `json:"is_active" example:"true"`
	OpensAt      time.Time            `json:"opens_at" example:"2023-01-01T09:00:00Z"`
	ClosesAt     time.Time            `json:"closes_at" example:"2023-01-01T10:00:00Z"`
	Participants []SwaggerParticipant `json:"participants"`
}
