package app

import (
	"embed"
	"log"
	"net/http"
	"os"

	"example.com/m/automation"
	"example.com/m/internal/config"
	"example.com/m/internal/controller"
	"example.com/m/internal/middleware"
	"example.com/m/internal/repository"
	"example.com/m/internal/service"
	"example.com/m/telegram"
	"example.com/m/web"
	"github.com/gin-gonic/gin"
)

type App struct {
	config      *config.Config
	router      *gin.Engine
	webFiles    embed.FS
	controllers *Controllers
	services    *Services
}

type Controllers struct {
	App      controller.AppController
	Log      controller.LogController
	Telegram controller.TelegramController
}

type Services struct {
	App      service.AppService
	Log      service.LogService
	Telegram service.TelegramService
}

func NewApp(webFiles embed.FS) *App {
	cfg := config.Load()

	app := &App{
		config:   cfg,
		webFiles: webFiles,
	}

	app.initServices()
	app.initControllers()
	app.initRouter()

	return app
}

func (a *App) initServices() {
	// Repositories
	logRepo := repository.NewLogRepository(a.config.GetLogFilePath())

	// External dependencies
	keyboardManager := automation.NewKeyboardManager()
	telegramBot := telegram.NewTelegramBot(a.config.Telegram.Token, a.config.Telegram.ChatID)

	// Services
	a.services = &Services{
		App:      service.NewAppService(keyboardManager, a.config),
		Log:      service.NewLogService(logRepo),
		Telegram: service.NewTelegramService(telegramBot, a.config),
	}
}

func (a *App) initControllers() {
	a.controllers = &Controllers{
		App:      controller.NewAppController(a.services.App),
		Log:      controller.NewLogController(a.services.Log),
		Telegram: controller.NewTelegramController(a.services.Telegram),
	}
}

func (a *App) initRouter() {
	if a.config.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	a.router = gin.New()
	a.router.Use(gin.Logger())
	a.router.Use(gin.Recovery())
	a.router.Use(middleware.CORS())

	// 직접 라우트 설정 (순환 참조 방지)
	a.setupRoutes()
}

func (a *App) setupRoutes() {
	// API 라우트 먼저 등록 (우선순위)
	api := a.router.Group("/api")
	{
		// App routes
		api.POST("/start", a.controllers.App.Start)
		api.POST("/stop", a.controllers.App.Stop)
		api.POST("/reset", a.controllers.App.Reset)
		api.POST("/exit", a.controllers.App.Exit)
		api.GET("/status", a.controllers.App.Status)
		api.POST("/settings", a.controllers.App.Settings)
		api.GET("/settings/load", a.controllers.App.LoadSettings)

		// Log routes
		api.GET("/logs", a.controllers.Log.GetLogs)
		api.POST("/logs/clear", a.controllers.Log.ClearLogs)

		// Telegram routes
		api.POST("/telegram/config", a.controllers.Telegram.Config)
		api.POST("/telegram/test", a.controllers.Telegram.Test)
		api.GET("/telegram/config", a.controllers.Telegram.GetConfig)
	}

	// 정적 파일 서빙 (마지막에 등록)
	web.SetupStaticRoutes(a.router, a.webFiles)
}

func (a *App) Run() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	return http.ListenAndServe(":"+port, a.router)
}
