package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"example.com/m/utils"
)

// SettingsConfigInterface - 설정 핸들러 전용 인터페이스
type SettingsConfigInterface interface {
	SetDarkMode(enabled bool) error
	SetSoundEnabled(enabled bool) error
	SetAutoStartup(enabled bool) error
	SetTelegramEnabled(enabled bool) error
}

type SettingsHandler struct {
	Config interface{}
}

func NewSettingsHandler(config interface{}) *SettingsHandler {
	return &SettingsHandler{
		Config: config,
	}
}

func (h *SettingsHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Config를 실제 타입으로 변환
	config, ok := h.Config.(SettingsConfigInterface)
	if !ok {
		http.Error(w, "Config interface error", http.StatusInternalServerError)
		return
	}

	settingType := r.FormValue("type")
	settingValue := r.FormValue("value")

	if settingType == "" || settingValue == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	var err error
	switch settingType {
	case "dark_mode":
		enabled := settingValue == "1"
		err = config.SetDarkMode(enabled)
	case "sound_enabled":
		enabled := settingValue == "1"
		err = config.SetSoundEnabled(enabled)
	case "auto_startup":
		enabled := settingValue == "1"
		err = config.SetAutoStartup(enabled)
	case "telegram_enabled":
		enabled := settingValue == "1"
		err = config.SetTelegramEnabled(enabled)
	case "mode", "time":
		// 로그만 기록
		log.Printf("설정 변경: %s = %s", settingType, settingValue)
	default:
		http.Error(w, "Unknown setting type", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Settings updated")
}

func (h *SettingsHandler) HandleLoadSettings(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 여기서는 실제 설정값을 가져와야 하는데,
	// 인터페이스에서 개별 설정값을 가져오는 메서드가 필요합니다.
	// 임시로 기본값을 사용합니다.
	settings := SettingsResponse{
		DarkMode:        true,
		SoundEnabled:    true,
		AutoStartup:     false,
		TelegramEnabled: false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
