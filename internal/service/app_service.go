package service

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"example.com/m/automation"
	"example.com/m/internal/config"
	"example.com/m/internal/dto"
	"example.com/m/internal/entity"
)

type AppService interface {
	StartOperation(mode string, autoStopHours float64, isResume bool) error
	StopOperation() error
	ResetSettings() error
	GetStatus() *dto.StatusResponse
	ExitApplication()
	SaveSetting(settingType, value string) error
	GetSettings() *dto.SettingsResponse
}

type appService struct {
	keyboardManager *automation.KeyboardManager
	config          *config.Config
	status          *entity.AppStatus
	autoStopTimer   *time.Timer
}

func NewAppService(keyboardManager *automation.KeyboardManager, config *config.Config) AppService {
	return &appService{
		keyboardManager: keyboardManager,
		config:          config,
		status: &entity.AppStatus{
			IsRunning: false,
		},
	}
}

func (s *appService) StartOperation(mode string, autoStopHours float64, isResume bool) error {
	if s.status.IsRunning {
		return fmt.Errorf("이미 실행 중입니다")
	}

	s.status.IsRunning = true
	s.status.Mode = mode
	s.status.StartTime = time.Now()

	s.keyboardManager.SetRunning(true)

	if autoStopHours > 0 {
		s.setupAutoStop(mode, autoStopHours)
	}

	go s.runAutomation(mode)

	log.Printf("작업 시작: %s 모드", s.getModeName(mode))
	return nil
}

func (s *appService) StopOperation() error {
	if !s.status.IsRunning {
		return fmt.Errorf("실행 중인 작업이 없습니다")
	}

	s.status.IsRunning = false
	s.keyboardManager.SetRunning(false)

	if s.autoStopTimer != nil {
		s.autoStopTimer.Stop()
		s.autoStopTimer = nil
	}

	log.Printf("작업 중지됨")
	return nil
}

func (s *appService) ResetSettings() error {
	if s.status.IsRunning {
		return fmt.Errorf("실행 중에는 재설정할 수 없습니다")
	}

	s.status = &entity.AppStatus{
		IsRunning: false,
	}

	log.Printf("설정 재설정됨")
	return nil
}

func (s *appService) GetStatus() *dto.StatusResponse {
	return &dto.StatusResponse{
		Running: s.status.IsRunning,
	}
}

func (s *appService) ExitApplication() {
	log.Printf("애플리케이션 종료 요청")
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

func (s *appService) SaveSetting(settingType, value string) error {
	switch settingType {
	case "dark_mode":
		enabled := value == "1"
		s.config.App.DarkMode = enabled
	case "sound_enabled":
		enabled := value == "1"
		s.config.App.SoundEnabled = enabled
	case "auto_startup":
		enabled := value == "1"
		s.config.App.AutoStartup = enabled
	case "telegram_enabled":
		enabled := value == "1"
		s.config.App.TelegramEnabled = enabled
	case "mode":
		log.Printf("모드 설정: %s", value)
	case "time":
		if hours, err := strconv.ParseFloat(value, 64); err == nil {
			log.Printf("시간 설정: %.1f시간", hours)
		}
	default:
		return fmt.Errorf("알 수 없는 설정 타입: %s", settingType)
	}

	return s.config.SaveAppConfig()
}

func (s *appService) GetSettings() *dto.SettingsResponse {
	return &dto.SettingsResponse{
		DarkMode:        s.config.App.DarkMode,
		SoundEnabled:    s.config.App.SoundEnabled,
		AutoStartup:     s.config.App.AutoStartup,
		TelegramEnabled: s.config.App.TelegramEnabled,
	}
}

func (s *appService) setupAutoStop(mode string, hours float64) {
	if s.autoStopTimer != nil {
		s.autoStopTimer.Stop()
	}

	if hours <= 0 {
		return
	}

	duration := time.Duration(hours * float64(time.Hour))
	s.autoStopTimer = time.AfterFunc(duration, func() {
		if s.status.IsRunning {
			modeName := s.getModeName(mode)
			s.StopOperation()
			log.Printf("작업 완료: %s 모드, %v 실행", modeName, duration)
		}
	})
}

func (s *appService) runAutomation(mode string) {
	switch mode {
	case "daeya-entrance":
		s.keyboardManager.DaeyaEnter()
	case "daeya-party":
		s.keyboardManager.DaeyaParty()
	case "kanchen-entrance":
		s.keyboardManager.KanchenEnter()
	case "kanchen-party":
		s.keyboardManager.KanchenParty()
	default:
		log.Printf("알 수 없는 모드: %s", mode)
	}
}

func (s *appService) getModeName(mode string) string {
	switch mode {
	case "daeya-entrance":
		return "대야 (입장)"
	case "daeya-party":
		return "대야 (파티)"
	case "kanchen-entrance":
		return "칸첸 (입장)"
	case "kanchen-party":
		return "칸첸 (파티)"
	default:
		return "알 수 없음"
	}
}
