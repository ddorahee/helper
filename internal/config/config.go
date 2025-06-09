package config

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Mode        string
	Version     string
	BuildDate   string
	LogFilePath string
	DataDir     string
	App         AppConfig
	Telegram    TelegramConfig
}

type AppConfig struct {
	DarkMode        bool
	SoundEnabled    bool
	AutoStartup     bool
	TelegramEnabled bool
}

type TelegramConfig struct {
	Token   string
	ChatID  string
	Enabled bool
}

func Load() *Config {
	cfg := &Config{
		Mode:      getEnv("APP_MODE", "development"),
		Version:   getEnv("VERSION", "dev"),
		BuildDate: getEnv("BUILD_DATE", "unknown"),
	}

	cfg.DataDir = getAppDataDir()
	cfg.LogFilePath = filepath.Join(cfg.DataDir, "app.log")

	cfg.loadAppConfig()
	cfg.loadTelegramConfig()

	return cfg
}

func (c *Config) GetLogFilePath() string {
	return c.LogFilePath
}

func (c *Config) SaveAppConfig() error {
	// 설정 저장 로직
	return nil
}

func (c *Config) SaveTelegramConfig() error {
	// 텔레그램 설정 저장 로직
	return nil
}

func (c *Config) loadAppConfig() {
	// 앱 설정 로드 로직
	c.App = AppConfig{
		DarkMode:        true,
		SoundEnabled:    true,
		AutoStartup:     false,
		TelegramEnabled: false,
	}
}

func (c *Config) loadTelegramConfig() {
	// 텔레그램 설정 로드 로직
	c.Telegram = TelegramConfig{
		Token:   getEnv("TELEGRAM_TOKEN", ""),
		ChatID:  getEnv("TELEGRAM_CHAT_ID", ""),
		Enabled: false,
	}
}

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

	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		log.Printf("Failed to create data directory: %v", err)
	}

	return appDataDir
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
