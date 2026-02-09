package config

import (
	"os"
)

// 버전 정보 (빌드 시 -ldflags로 주입됨)
var (
	Version   = "dev"
	BuildDate = "unknown"
)

// AppConfig는 애플리케이션 설정을 관리합니다
type AppConfig struct {
	DevelopmentMode bool
	Version         string
	BuildDate       string
}

// NewAppConfig는 새로운 앱 설정을 생성합니다
func NewAppConfig() *AppConfig {
	// 빌드 플래그로 대체될 수 있는 값 (기본적으로 프로덕션 모드)
	cfg := &AppConfig{
		DevelopmentMode: false,
		Version:         Version,
		BuildDate:       BuildDate,
	}

	// 개발 모드 확인 (dev 태그로 빌드된 경우)
	if Version == "dev" {
		cfg.DevelopmentMode = true
	}

	// 개발 모드일 때 환경 변수 설정
	if cfg.DevelopmentMode {
		os.Setenv("DEV_MODE", "1")
	}

	return cfg
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