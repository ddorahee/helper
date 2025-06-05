package services

import (
	"example.com/m/automation"
	"example.com/m/config"
	"example.com/m/utils"
)

// ServiceContainer는 모든 서비스를 관리합니다
type ServiceContainer struct {
	Config          *config.AppConfig
	AppService      *AppService
	LogService      *LogService
	TelegramService *TelegramService
	TimerManager    *utils.TimerManager
	KeyboardManager *automation.KeyboardManager
}

// NewServiceContainer는 새로운 서비스 컨테이너를 생성합니다
func NewServiceContainer(appConfig *config.AppConfig) *ServiceContainer {
	timerManager := utils.NewTimerManager()
	keyboardManager := automation.NewKeyboardManager()

	container := &ServiceContainer{
		Config:          appConfig,
		TimerManager:    timerManager,
		KeyboardManager: keyboardManager,
	}

	// 서비스 초기화
	container.AppService = NewAppService(container)
	container.LogService = NewLogService(container)
	container.TelegramService = NewTelegramService(container)

	return container
}
