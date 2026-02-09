package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// TelegramConfig 텔레그램 설정 데이터
type TelegramConfig struct {
	Token   string `json:"token"`
	ChatID  string `json:"chat_id"`
	Enabled bool   `json:"enabled"`
}

// TelegramStore 텔레그램 설정 저장소
type TelegramStore struct {
	Config   TelegramConfig
	filePath string
}

// NewTelegramStore 새 텔레그램 저장소 생성
func NewTelegramStore() *TelegramStore {
	return &TelegramStore{
		filePath: filepath.Join("config", "telegram.json"),
	}
}

// Load 설정 파일 로드
func (ts *TelegramStore) Load() error {
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &ts.Config); err != nil {
		return err
	}

	log.Printf("[텔레그램] 설정 로드 완료: enabled=%v", ts.Config.Enabled)
	return nil
}

// Save 설정 파일 저장
func (ts *TelegramStore) Save() error {
	dir := filepath.Dir(ts.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ts.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ts.filePath, data, 0644)
}

// IsConfigured 텔레그램이 설정되어 있는지 확인
func (ts *TelegramStore) IsConfigured() bool {
	return ts.Config.Token != "" && ts.Config.ChatID != "" && ts.Config.Enabled
}

// SetConfig 텔레그램 설정 업데이트
func (ts *TelegramStore) SetConfig(token, chatID string, enabled bool) {
	ts.Config.Token = token
	ts.Config.ChatID = chatID
	ts.Config.Enabled = enabled
}
