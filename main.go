package main

import (
	"fmt"
	"log"
	"os"
	_ "test_hack/docs"
	"test_hack/internal/auth"
	"test_hack/internal/handlers"
	"test_hack/internal/models"
	"test_hack/internal/storage"
	"test_hack/internal/tasks"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @Title						Онлайн очередь для сдачи практики
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
func main() {
	key := os.Getenv("ENV_CHEK")
	if key == "" {
		fmt.Println("Подключение к .env")
		err := godotenv.Load()
		if err != nil {
			log.Fatal("Ошибка получения .env")
		}
	}

	storage.ConnectDatabase()

	if err := storage.DB.AutoMigrate(&models.User{}, &models.Schedule{}, &models.Queue{}, &models.QueueEntry{}); err != nil {
		log.Fatal("Ошибка при миграции... ", err.Error())
	}

	storage.InitRedis()
	tasks.InitScheduler()

	go handlers.HubInstance.Run()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", handlers.Login)
		authGroup.POST("/register", handlers.Register)
		authGroup.POST("/refresh", handlers.RefreshToken)
	}

	profileGroup := r.Group("/profile", auth.AuthMiddleware())
	{
		profileGroup.GET("/", handlers.GetMyProfileHandler)
		profileGroup.GET("/queues", handlers.GetUserQueuesHandler)
	}

	apiGroup := r.Group("")
	{
		apiGroup.GET("/groups", handlers.GetGroupsHandler)
		apiGroup.GET("/schedule", handlers.GetFullScheduleHandler)
	}

	r.GET("/api/queues/:id/status", handlers.GetQueueStatusHandler)
	r.GET("/api/queues/:id/ws", handlers.QueueWebSocketHandler)
	queues := r.Group("/api/queues", auth.AuthMiddleware())
	{
		queues.POST("/:id/join", handlers.JoinQueueHandler)
		queues.POST("/:id/leave", handlers.LeaveQueueHandler)
	}

	if err := r.Run(":8080"); err != nil {
		log.Fatal("Ошибка запуска сервера...", err.Error())
	}
}
