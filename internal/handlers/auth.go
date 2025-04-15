package handlers

import (
	"net/http"
	"os"
	"test_hack/internal/models"
	"test_hack/internal/response"
	"test_hack/internal/storage"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	AccessSecret  = []byte(os.Getenv("JWT_ACCESS_SECRET"))
	refreshSecret = []byte(os.Getenv("JWT_REFRESH_SECRET"))
)

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Surname  string `json:"surname" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// @Summary		Регистрация пользователя
// @Description	Регистрация нового пользователя
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			user	body		RegisterRequest				true	"Данные пользователя"
// @Success		201		{object}	response.SuccessResponse	"Успешная регистрация"
// @Failure		400		{object}	response.ErrorResponse		"Ошибка валидации (VALIDATION_ERROR) или пользователь уже существует (EMAIL_EXISTS)"
// @Failure		500		{object}	response.ErrorResponse		"Ошибка сервера (PASSWORD_HASH_ERROR, DB_ERROR)"
// @Router			/auth/register [post]
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "VALIDATION_ERROR",
			Message: "Ошибка валидации данных",
			Details: err.Error(),
		})
		return
	}

	var existingUser models.User
	if err := storage.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "EMAIL_EXISTS",
			Message: "Пользователь с таким email уже существует",
		})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "PASSWORD_HASH_ERROR",
			Message: "Ошибка при хешировании пароля",
		})
		return
	}

	user := models.User{
		Name:         req.Name,
		Surname:      req.Surname,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	if err := storage.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "DB_ERROR",
			Message: "Ошибка при создании пользователя",
		})
		return
	}

	c.JSON(http.StatusCreated, response.SuccessResponse{
		Message: "Пользователь успешно зарегистрирован",
	})
}

// @Summary		Авторизация пользователя
// @Description	Авторизация пользователя и получение токенов
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			user	body		LoginRequest			true	"Данные для авторизации"
// @Success		200		{object}	response.TokenResponse	"Успешная авторизация"
// @Failure		400		{object}	response.ErrorResponse	"Ошибка валидации данных (VALIDATION_ERROR)"
// @Failure		401		{object}	response.ErrorResponse	"Неверные учетные данные (INVALID_CREDENTIALS)"
// @Failure		500		{object}	response.ErrorResponse	"Ошибка сервера (TOKEN_GENERATION_ERROR)"
// @Router			/auth/login [post]
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "VALIDATION_ERROR",
			Message: "Ошибка валидации данных",
			Details: err.Error(),
		})
		return
	}

	var user models.User
	if err := storage.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{
			Code:    "INVALID_CREDENTIALS",
			Message: "Неверный email или пароль",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{
			Code:    "INVALID_CREDENTIALS",
			Message: "Неверный email или пароль",
		})
		return
	}

	accessToken, err := generateToken(user.ID, time.Minute*15, AccessSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "TOKEN_GENERATION_ERROR",
			Message: "Ошибка при генерации access токена",
		})
		return
	}

	refreshToken, err := generateToken(user.ID, time.Hour*24*7, refreshSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "TOKEN_GENERATION_ERROR",
			Message: "Ошибка при генерации refresh токена",
		})
		return
	}

	c.JSON(http.StatusOK, response.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func generateToken(userID uint, duration time.Duration, secret []byte) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(duration).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// @Summary		Обновление access токена
// @Description	Обновление access токена с помощью refresh токена
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			refresh_token	body		RefreshTokenRequest		true	"Refresh токен"
// @Success		200				{object}	response.TokenResponse	"Успешное обновление access токена"
// @Failure		400				{object}	response.ErrorResponse	"Ошибка валидации данных (VALIDATION_ERROR)"
// @Failure		401				{object}	response.ErrorResponse	"Неверный или просроченный refresh токен (INVALID_REFRESH_TOKEN) или пользователь не найден (USER_NOT_FOUND)"
// @Failure		500				{object}	response.ErrorResponse	"Ошибка сервера (TOKEN_GENERATION_ERROR)"
// @Router			/auth/refresh [post]
func RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{
			Code:    "VALIDATION_ERROR",
			Message: "Ошибка валидации данных",
			Details: err.Error(),
		})
		return
	}

	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return refreshSecret, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{
			Code:    "INVALID_REFRESH_TOKEN",
			Message: "Неверный или просроченный refresh токен",
		})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{
			Code:    "INVALID_REFRESH_TOKEN",
			Message: "Неверный или просроченный refresh токен",
		})
		return
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{
			Code:    "INVALID_REFRESH_TOKEN",
			Message: "Неверный или просроченный refresh токен",
		})
		return
	}

	userID := uint(userIDFloat)

	var user models.User
	if err := storage.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{
			Code:    "USER_NOT_FOUND",
			Message: "Пользователь не найден",
		})
		return
	}

	newAccessToken, err := generateToken(user.ID, time.Minute*15, AccessSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "TOKEN_GENERATION_ERROR",
			Message: "Ошибка при генерации access токена",
		})
		return
	}

	newRefreshToken, err := generateToken(userID, time.Hour*24*7, refreshSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{
			Code:    "TOKEN_GENERATION_ERROR",
			Message: "Ошибка при генерации нового refresh токена",
		})
		return
	}

	c.JSON(http.StatusOK, response.TokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	})
}
