package service

import (
	"fmt"
	"log"
	"time"

	"example.com/m/internal/config"
	"example.com/m/internal/dto"
	"example.com/m/telegram"
)

type TelegramService interface {
	SaveConfig(token, chatID string) error
	TestConnection() error
	GetStatus() *dto.TelegramStatusResponse
	SendStartNotification(modeName string, duration float64) error
	SendCompletionNotification(modeName string, duration float64) error
	SendErrorNotification(modeName, errorMsg string) error
}

type telegramService struct {
	telegramBot *telegram.TelegramBot
	config      *config.Config
}

func NewTelegramService(telegramBot *telegram.TelegramBot, config *config.Config) TelegramService {
	return &telegramService{
		telegramBot: telegramBot,
		config:      config,
	}
}

func (s *telegramService) SaveConfig(token, chatID string) error {
	// 새 텔레그램 봇 인스턴스 생성
	s.telegramBot = telegram.NewTelegramBot(token, chatID)

	// 설정 업데이트
	s.config.Telegram.Token = token
	s.config.Telegram.ChatID = chatID
	s.config.Telegram.Enabled = true
	s.config.App.TelegramEnabled = true

	// 설정 저장
	err := s.config.SaveTelegramConfig()
	if err != nil {
		log.Printf("Failed to save telegram config: %v", err)
		return err
	}

	log.Printf("Telegram config saved successfully")
	return nil
}

func (s *telegramService) TestConnection() error {
	if s.telegramBot == nil || !s.config.Telegram.Enabled {
		return fmt.Errorf("텔레그램이 설정되지 않았습니다")
	}

	err := s.telegramBot.TestConnection()
	if err != nil {
		log.Printf("Telegram test failed: %v", err)
		return err
	}

	log.Printf("Telegram test successful")
	return nil
}

func (s *telegramService) GetStatus() *dto.TelegramStatusResponse {
	return &dto.TelegramStatusResponse{
		Enabled: s.config.Telegram.Enabled,
	}
}

func (s *telegramService) SendStartNotification(modeName string, duration float64) error {
	if s.telegramBot == nil || !s.config.Telegram.Enabled {
		return fmt.Errorf("텔레그램이 활성화되지 않았습니다")
	}

	durationTime := time.Duration(duration * float64(time.Hour))
	return s.telegramBot.SendStartNotification(modeName, durationTime)
}

func (s *telegramService) SendCompletionNotification(modeName string, duration float64) error {
	if s.telegramBot == nil || !s.config.Telegram.Enabled {
		return fmt.Errorf("텔레그램이 활성화되지 않았습니다")
	}

	durationTime := time.Duration(duration * float64(time.Hour))
	return s.telegramBot.SendCompletionNotification(modeName, durationTime)
}

func (s *telegramService) SendErrorNotification(modeName, errorMsg string) error {
	if s.telegramBot == nil || !s.config.Telegram.Enabled {
		return fmt.Errorf("텔레그램이 활성화되지 않았습니다")
	}

	return s.telegramBot.SendErrorNotification(modeName, errorMsg)
}
