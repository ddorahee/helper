package services

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// AppService는 앱 관련 비즈니스 로직을 처리합니다
type AppService struct {
	container     *ServiceContainer
	autoStopTimer *time.Timer
}

// NewAppService는 새로운 앱 서비스를 생성합니다
func NewAppService(container *ServiceContainer) *AppService {
	return &AppService{
		container: container,
	}
}

// StartOperation은 매크로 작업을 시작합니다
func (s *AppService) StartOperation(mode string, autoStopHours float64, isResume bool) error {
	if s.container.TimerManager.IsRunning() {
		return fmt.Errorf("이미 실행 중입니다")
	}

	// 타이머 시작
	s.container.TimerManager.Start()

	// 키보드 매니저 시작
	s.container.KeyboardManager.SetRunning(true)

	// 자동 중지 설정
	if autoStopHours > 0 {
		s.setupAutoStop(mode, autoStopHours)
	}

	// 선택된 모드에 따라 자동화 시작
	go s.runAutomation(mode)

	// 텔레그램 시작 알림 전송
	if !isResume && s.container.Config.TelegramEnabled && s.container.Config.TelegramBot != nil {
		modeName := s.GetModeName(mode)
		duration := time.Duration(autoStopHours * float64(time.Hour))
		go func() {
			err := s.container.Config.TelegramBot.SendStartNotification(modeName, duration)
			if err != nil {
				log.Printf("텔레그램 시작 알림 전송 실패: %v", err)
			}
		}()
	}

	log.Printf("작업 시작: %s 모드", s.GetModeName(mode))
	return nil
}

// StopOperation은 매크로 작업을 중지합니다
func (s *AppService) StopOperation() error {
	if !s.container.TimerManager.IsRunning() {
		return fmt.Errorf("실행 중인 작업이 없습니다")
	}

	// 타이머 중지
	s.container.TimerManager.Stop()

	// 키보드 매니저 중지
	s.container.KeyboardManager.SetRunning(false)

	// 자동 중지 타이머 중지
	if s.autoStopTimer != nil {
		s.autoStopTimer.Stop()
		s.autoStopTimer = nil
	}

	log.Printf("작업 중지됨")
	return nil
}

// ResetSettings는 설정을 초기화합니다
func (s *AppService) ResetSettings() error {
	if s.container.TimerManager.IsRunning() {
		return fmt.Errorf("실행 중에는 재설정할 수 없습니다")
	}

	// 타이머 재설정
	s.container.TimerManager.Reset()

	log.Printf("설정 재설정됨")
	return nil
}

// GetStatus는 현재 상태를 반환합니다
func (s *AppService) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"running": s.container.TimerManager.IsRunning(),
	}
}

// ExitApplication은 애플리케이션을 종료합니다
func (s *AppService) ExitApplication() {
	log.Printf("애플리케이션 종료 요청")
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

// SaveSetting은 설정을 저장합니다
func (s *AppService) SaveSetting(settingType, value string) error {
	switch settingType {
	case "dark_mode":
		enabled := value == "1"
		return s.container.Config.SetDarkMode(enabled)
	case "sound_enabled":
		enabled := value == "1"
		return s.container.Config.SetSoundEnabled(enabled)
	case "auto_startup":
		enabled := value == "1"
		return s.container.Config.SetAutoStartup(enabled)
	case "telegram_enabled":
		enabled := value == "1"
		return s.container.Config.SetTelegramEnabled(enabled)
	case "mode":
		// 모드 설정은 별도 처리 없이 로그만
		log.Printf("모드 설정: %s", value)
		return nil
	case "time":
		// 시간 설정은 별도 처리 없이 로그만
		if hours, err := strconv.ParseFloat(value, 64); err == nil {
			log.Printf("시간 설정: %.1f시간", hours)
		}
		return nil
	default:
		return fmt.Errorf("알 수 없는 설정 타입: %s", settingType)
	}
}

// GetSettings는 저장된 설정을 반환합니다
func (s *AppService) GetSettings() map[string]interface{} {
	return map[string]interface{}{
		"dark_mode":        s.container.Config.DarkMode,
		"sound_enabled":    s.container.Config.SoundEnabled,
		"auto_startup":     s.container.Config.AutoStartup,
		"telegram_enabled": s.container.Config.TelegramEnabled,
	}
}

// setupAutoStop은 자동 중지를 설정합니다
func (s *AppService) setupAutoStop(mode string, hours float64) {
	if s.autoStopTimer != nil {
		s.autoStopTimer.Stop()
		s.autoStopTimer = nil
	}

	if hours <= 0 {
		return
	}

	duration := time.Duration(hours * float64(time.Hour))
	s.autoStopTimer = time.AfterFunc(duration, func() {
		if s.container.TimerManager.IsRunning() {
			modeName := s.GetModeName(mode)

			// 작업 중지
			s.StopOperation()

			// 텔레그램 완료 알림 전송
			if s.container.Config.TelegramEnabled && s.container.Config.TelegramBot != nil {
				go func() {
					err := s.container.Config.TelegramBot.SendCompletionNotification(modeName, duration)
					if err != nil {
						log.Printf("텔레그램 완료 알림 전송 실패: %v", err)
					}
				}()
			}

			log.Printf("작업 완료: %s 모드, %v 실행", modeName, duration)
		}
	})
}

// runAutomation은 선택된 모드에 따라 자동화를 실행합니다
func (s *AppService) runAutomation(mode string) {
	switch mode {
	case "daeya-entrance":
		s.container.KeyboardManager.DaeyaEnter()
	case "daeya-party":
		s.container.KeyboardManager.DaeyaParty()
	case "kanchen-entrance":
		s.container.KeyboardManager.KanchenEnter()
	case "kanchen-party":
		s.container.KeyboardManager.KanchenParty()
	default:
		log.Printf("알 수 없는 모드: %s", mode)
	}
}

// GetModeName은 모드 이름을 반환합니다
func (s *AppService) GetModeName(mode string) string {
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
