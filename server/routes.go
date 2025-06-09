package server

import (
	"net/http"

	"example.com/m/handlers"
)

// Application 인터페이스 정의 (순환 import 방지)
type ApplicationInterface interface {
	GetConfig() interface{}
	GetTimerManager() interface{}
	GetKeyboardManager() interface{}
	GetTelegramBot() interface{}
	SetupAutoStop(mode string, hours float64)
	RunAutomation(mode string)
}

func (s *Server) SetupHandlers(app ApplicationInterface, mux *http.ServeMux) {
	// API 핸들러 생성
	apiHandler := handlers.NewAPIHandler(app)
	logHandler := handlers.NewLogHandler(s.Config)
	settingsHandler := handlers.NewSettingsHandler(s.Config)
	telegramHandler := handlers.NewTelegramHandler(s.Config, app)

	// API 엔드포인트 설정
	mux.HandleFunc("/api/start", apiHandler.HandleStart)
	mux.HandleFunc("/api/stop", apiHandler.HandleStop)
	mux.HandleFunc("/api/status", apiHandler.HandleStatus)
	mux.HandleFunc("/api/reset", apiHandler.HandleReset)
	mux.HandleFunc("/api/exit", apiHandler.HandleExit)

	// 로그 엔드포인트
	mux.HandleFunc("/api/logs", logHandler.HandleLogs)
	mux.HandleFunc("/api/logs/clear", logHandler.HandleClearLogs)
	mux.HandleFunc("/api/log", logHandler.HandleLog)

	// 설정 엔드포인트
	mux.HandleFunc("/api/settings", settingsHandler.HandleSettings)
	mux.HandleFunc("/api/settings/load", settingsHandler.HandleLoadSettings)

	// 텔레그램 엔드포인트
	mux.HandleFunc("/api/telegram/config", telegramHandler.HandleTelegramConfig)
	mux.HandleFunc("/api/telegram/test", telegramHandler.HandleTelegramTest)
}
