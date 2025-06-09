// server/routes.go - 키 맵핑 라우트 수정
package server

import (
	"net/http"

	"example.com/m/handlers"
	"example.com/m/keymapping"
)

// Application 인터페이스 정의 (키 맵핑 매니저 추가)
type ApplicationInterface interface {
	GetConfig() interface{}
	GetTimerManager() interface{}
	GetKeyboardManager() interface{}
	GetKeyMappingManager() interface{} // 키 맵핑 매니저 추가
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

	// 키 맵핑 핸들러 생성 (타입 어설션 수정)
	if keyMappingManagerInterface := app.GetKeyMappingManager(); keyMappingManagerInterface != nil {
		if keyMappingManager, ok := keyMappingManagerInterface.(*keymapping.KeyMappingManager); ok {
			keyMappingHandler := handlers.NewKeyMappingHandler(keyMappingManager)

			// 키 맵핑 엔드포인트 설정
			mux.HandleFunc("/api/keymapping/mappings", keyMappingHandler.HandleMappings)
			mux.HandleFunc("/api/keymapping/toggle", keyMappingHandler.HandleMappingToggle)
			mux.HandleFunc("/api/keymapping/control", keyMappingHandler.HandleMappingControl)
			mux.HandleFunc("/api/keymapping/keys", keyMappingHandler.HandleAvailableKeys)
			mux.HandleFunc("/api/keymapping/status", keyMappingHandler.HandleMappingStatus)
		}
	}

	// 기존 API 엔드포인트 설정
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
