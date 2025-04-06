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
	"test_hack/internal/ws"

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

	go ws.HubInstance.Run()

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
		authGroup.POST("/login", auth.Login)
		authGroup.POST("/register", auth.Register)
		authGroup.POST("/refresh", auth.RefreshToken)
	}

	apiGroup := r.Group("")
	{
		apiGroup.GET("/groups", handlers.GetGroupsHandler)
		apiGroup.GET("/schedule", handlers.GetScheduleHandler)
	}

	queues := r.Group("/api/queues")
	{
		queues.GET("/:id/ws", ws.QueueWebSocketHandler)
	}

	if err := r.Run(":8080"); err != nil {
		log.Fatal("Ошибка запуска сервера...", err.Error())
	}
}
