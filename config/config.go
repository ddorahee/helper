package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"example.com/m/telegram"
)

// 버전 정보 (빌드 시 -ldflags로 주입됨)
var (
	Version   = "dev"
	BuildDate = "unknown"
)

// ConfigData는 저장할 설정 데이터 구조체입니다
type ConfigData struct {
	TelegramToken   string `json:"telegram_token"`
	TelegramChatID  string `json:"telegram_chat_id"`
	TelegramEnabled bool   `json:"telegram_enabled"`
	DarkMode        bool   `json:"dark_mode"`
	SoundEnabled    bool   `json:"sound_enabled"`
	AutoStartup     bool   `json:"auto_startup"`
}

// AppConfig는 애플리케이션 설정을 관리합니다
type AppConfig struct {
	DevelopmentMode bool
	Version         string
	BuildDate       string
	TelegramBot     *telegram.TelegramBot
	TelegramEnabled bool
	DarkMode        bool
	SoundEnabled    bool
	AutoStartup     bool
	configFilePath  string
	logFilePath     string
}

// NewAppConfig는 새로운 앱 설정을 생성합니다
func NewAppConfig() *AppConfig {
	cfg := &AppConfig{
		DevelopmentMode: false,
		Version:         Version,
		BuildDate:       BuildDate,
		TelegramEnabled: false,
		DarkMode:        true,  // 기본값: 다크모드 켜짐
		SoundEnabled:    true,  // 기본값: 소리 켜짐
		AutoStartup:     false, // 기본값: 자동시작 꺼짐
	}

	// 경로 설정
	cfg.configFilePath = getConfigFilePath()
	cfg.logFilePath = getLogFilePath()

	// 개발 모드 확인
	if Version == "dev" {
		cfg.DevelopmentMode = true
	}

	// 개발 모드일 때 환경 변수 설정
	if cfg.DevelopmentMode {
		os.Setenv("DEV_MODE", "1")
	}

	// 저장된 설정 로드
	cfg.LoadSettings()

	return cfg
}

// getAppDataDir는 OS별 앱 데이터 디렉토리를 반환합니다
func getAppDataDir() string {
	var appDataDir string

	switch runtime.GOOS {
	case "windows":
		appDataDir = os.Getenv("APPDATA")
		if appDataDir == "" {
			appDataDir = "."
		}
		appDataDir = filepath.Join(appDataDir, "DoumiBrowser Helper")
	case "darwin":
		homeDir, _ := os.UserHomeDir()
		appDataDir = filepath.Join(homeDir, "Library", "Application Support", "DoumiBrowser Helper")
	case "linux":
		homeDir, _ := os.UserHomeDir()
		appDataDir = filepath.Join(homeDir, ".config", "doumibrowser-helper")
	default:
		appDataDir = "data"
	}

	// 디렉토리 생성
	if !dirExists(appDataDir) {
		os.MkdirAll(appDataDir, 0755)
	}

	return appDataDir
}

// getConfigFilePath는 설정 파일 경로를 반환합니다
func getConfigFilePath() string {
	appDataDir := getAppDataDir()
	return filepath.Join(appDataDir, "settings.json")
}

// getLogFilePath는 로그 파일 경로를 반환합니다
func getLogFilePath() string {
	appDataDir := getAppDataDir()
	return filepath.Join(appDataDir, "app.log")
}

// GetLogFilePath는 외부에서 로그 파일 경로를 가져올 수 있도록 합니다
func (cfg *AppConfig) GetLogFilePath() string {
	return cfg.logFilePath
}

// dirExists는 디렉토리 존재 여부를 확인합니다
func dirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// LoadSettings는 파일에서 설정을 로드합니다
func (cfg *AppConfig) LoadSettings() error {
	if _, err := os.Stat(cfg.configFilePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(cfg.configFilePath)
	if err != nil {
		return err
	}

	var configData ConfigData
	if err := json.Unmarshal(data, &configData); err != nil {
		return err
	}

	// 설정 적용
	cfg.TelegramEnabled = configData.TelegramEnabled
	cfg.DarkMode = configData.DarkMode
	cfg.SoundEnabled = configData.SoundEnabled
	cfg.AutoStartup = configData.AutoStartup

	// 텔레그램 봇 초기화
	if configData.TelegramToken != "" && configData.TelegramChatID != "" {
		cfg.TelegramBot = telegram.NewTelegramBot(configData.TelegramToken, configData.TelegramChatID)
		cfg.TelegramEnabled = true
	}

	return nil
}

// SaveSettings는 설정을 파일에 저장합니다
func (cfg *AppConfig) SaveSettings() error {
	configData := ConfigData{
		TelegramEnabled: cfg.TelegramEnabled,
		DarkMode:        cfg.DarkMode,
		SoundEnabled:    cfg.SoundEnabled,
		AutoStartup:     cfg.AutoStartup,
	}

	// 텔레그램 설정 저장
	if cfg.TelegramBot != nil {
		configData.TelegramToken = cfg.TelegramBot.Token
		configData.TelegramChatID = cfg.TelegramBot.ChatID
	}

	jsonData, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cfg.configFilePath, jsonData, 0644)
}

// SetTelegramConfig는 텔레그램 설정을 업데이트하고 저장합니다
func (cfg *AppConfig) SetTelegramConfig(token, chatID string) error {
	if token != "" && chatID != "" {
		cfg.TelegramBot = telegram.NewTelegramBot(token, chatID)
		cfg.TelegramEnabled = true
	} else {
		cfg.TelegramEnabled = false
		cfg.TelegramBot = nil
	}

	return cfg.SaveSettings()
}

// SetDarkMode는 다크모드 설정을 업데이트하고 저장합니다
func (cfg *AppConfig) SetDarkMode(enabled bool) error {
	cfg.DarkMode = enabled
	return cfg.SaveSettings()
}

// SetSoundEnabled는 소리 알림 설정을 업데이트하고 저장합니다
func (cfg *AppConfig) SetSoundEnabled(enabled bool) error {
	cfg.SoundEnabled = enabled
	return cfg.SaveSettings()
}

// SetAutoStartup는 자동 시작 설정을 업데이트하고 저장합니다
func (cfg *AppConfig) SetAutoStartup(enabled bool) error {
	cfg.AutoStartup = enabled
	return cfg.SaveSettings()
}

// SetTelegramEnabled는 텔레그램 활성화 설정을 업데이트하고 저장합니다
func (cfg *AppConfig) SetTelegramEnabled(enabled bool) error {
	cfg.TelegramEnabled = enabled
	return cfg.SaveSettings()
}

// GetModeText는 현재 모드의 텍스트 표현을 반환합니다
func (cfg *AppConfig) GetModeText() string {
	if cfg.DevelopmentMode {
		return "개발"
	}
	return "프로덕션"
}

// GetVersionInfo는 버전 정보를 반환합니다
func (cfg *AppConfig) GetVersionInfo() string {
	return cfg.Version + " (" + cfg.BuildDate + ")"
}
