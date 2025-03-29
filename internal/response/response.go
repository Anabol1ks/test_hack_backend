package response

// SuccessResponse представляет успешный ответ API
type SuccessResponse struct {
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
