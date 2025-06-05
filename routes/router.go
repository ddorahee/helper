package routes

import (
	"embed"
	"net/http"

	"example.com/m/controllers"
	"example.com/m/services"
)

// NewRouter는 새로운 HTTP 라우터를 생성합니다
func NewRouter(serviceContainer *services.ServiceContainer, webFiles embed.FS) *http.ServeMux {
	router := http.NewServeMux()

	// 컨트롤러 초기화
	appController := controllers.NewAppController(serviceContainer)
	logController := controllers.NewLogController(serviceContainer)
	telegramController := controllers.NewTelegramController(serviceContainer)
	staticController := controllers.NewStaticController(webFiles)

	// 정적 파일 라우트
	router.HandleFunc("/", staticController.ServeStatic)

	// API 라우트
	setupAPIRoutes(router, appController, logController, telegramController)

	return router
}

func setupAPIRoutes(
	router *http.ServeMux,
	appController *controllers.AppController,
	logController *controllers.LogController,
	telegramController *controllers.TelegramController,
) {
	// 앱 관련 라우트
	router.HandleFunc("/api/start", appController.Start)
	router.HandleFunc("/api/stop", appController.Stop)
	router.HandleFunc("/api/reset", appController.Reset)
	router.HandleFunc("/api/exit", appController.Exit)
	router.HandleFunc("/api/status", appController.Status)
	router.HandleFunc("/api/settings", appController.Settings)
	router.HandleFunc("/api/settings/load", appController.LoadSettings)

	// 로그 관련 라우트
	router.HandleFunc("/api/log", logController.Log)
	router.HandleFunc("/api/logs", logController.Logs)
	router.HandleFunc("/api/logs/clear", logController.ClearLogs)

	// 텔레그램 관련 라우트
	router.HandleFunc("/api/telegram/config", telegramController.Config)
	router.HandleFunc("/api/telegram/test", telegramController.Test)
}
