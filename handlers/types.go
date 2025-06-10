// handlers/types.go 수정
package handlers

import "time"

// API 응답 구조체들
type APIResponse struct {
	Success bool        `json:"success,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type StatusResponse struct {
	Running bool `json:"running"`
	Mode    int  `json:"mode,omitempty"`
}

type SettingsResponse struct {
	DarkMode        bool `json:"dark_mode"`
	AutoStartup     bool `json:"auto_startup"`
	TelegramEnabled bool `json:"telegram_enabled"`
}

// 인터페이스 정의들 (순환 import 방지)
type ApplicationInterface interface {
	GetConfig() interface{}
	GetTimerManager() interface{}
	GetKeyboardManager() interface{}
	GetTelegramBot() interface{}
	SetupAutoStop(mode string, hours float64)
	RunAutomation(mode string)
}

type ConfigInterface interface {
	GetLogFilePath() string
	GetDataDir() string
	GetConfigFilePath() string
	SetTelegramConfig(token, chatID string) error
	SetDarkMode(enabled bool) error
	SetSoundEnabled(enabled bool) error
	SetAutoStartup(enabled bool) error
	SetTelegramEnabled(enabled bool) error
	// 기타 설정 관련 메서드들
}

type TimerManagerInterface interface {
	IsRunning() bool
	Start()
	Stop()
	Reset()
	GetElapsedTime() time.Duration
}

type KeyboardManagerInterface interface {
	IsRunning() bool
	SetRunning(running bool)
}

type TelegramBotInterface interface {
	SendStartNotification(modeName string, duration time.Duration) error
	SendCompletionNotification(modeName string, duration time.Duration) error
	SendErrorNotification(modeName string, errorMsg string) error
	TestConnection() error
}
