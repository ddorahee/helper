package services

import (
	"fmt"
	"log"
)

// TelegramService는 텔레그램 관련 비즈니스 로직을 처리합니다
type TelegramService struct {
	container *ServiceContainer
}

// NewTelegramService는 새로운 텔레그램 서비스를 생성합니다
func NewTelegramService(container *ServiceContainer) *TelegramService {
	return &TelegramService{
		container: container,
	}
}

// SaveConfig는 텔레그램 설정을 저장합니다
func (s *TelegramService) SaveConfig(token, chatID string) error {
	err := s.container.Config.SetTelegramConfig(token, chatID)
	if err != nil {
		log.Printf("텔레그램 설정 저장 실패: %v", err)
		return err
	}

	log.Printf("텔레그램 설정 저장됨")
	return nil
}

// GetStatus는 텔레그램 상태를 반환합니다
func (s *TelegramService) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"enabled": s.container.Config.TelegramEnabled,
	}
}

// TestConnection은 텔레그램 연결을 테스트합니다
func (s *TelegramService) TestConnection() error {
	if !s.container.Config.TelegramEnabled || s.container.Config.TelegramBot == nil {
		return fmt.Errorf("텔레그램이 설정되지 않았습니다")
	}

	err := s.container.Config.TelegramBot.TestConnection()
	if err != nil {
		log.Printf("텔레그램 테스트 실패: %v", err)
		return err
	}

	log.Printf("텔레그램 테스트 성공")
	return nil
}
