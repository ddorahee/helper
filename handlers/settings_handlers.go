// handlers/settings_handlers.go 완전 수정
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"example.com/m/config"
	"example.com/m/utils"
)

type SettingsHandler struct {
	Config *config.AppConfig // 직접 AppConfig 타입 사용
}

func NewSettingsHandler(appConfig *config.AppConfig) *SettingsHandler {
	return &SettingsHandler{
		Config: appConfig, // 직접 AppConfig 전달
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

	// Config 직접 사용
	if h.Config == nil {
		log.Printf("Config가 nil입니다")
		http.Error(w, "Config not initialized", http.StatusInternalServerError)
		return
	}

	settingType := r.FormValue("type")
	settingValue := r.FormValue("value")

	log.Printf("설정 요청 받음: type=%s, value=%s", settingType, settingValue)

	if settingType == "" || settingValue == "" {
		log.Printf("필수 파라미터 누락: type=%s, value=%s", settingType, settingValue)
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	var err error
	switch settingType {
	case "dark_mode":
		enabled := settingValue == "1"
		log.Printf("다크모드 설정 변경: %t", enabled)
		err = h.Config.SetDarkMode(enabled)
	case "auto_startup":
		enabled := settingValue == "1"
		log.Printf("자동시작 설정 변경: %t", enabled)
		err = h.Config.SetAutoStartup(enabled)
	case "telegram_enabled":
		enabled := settingValue == "1"
		log.Printf("텔레그램 설정 변경: %t", enabled)
		err = h.Config.SetTelegramEnabled(enabled)
	case "mode", "time":
		// 로그만 기록
		log.Printf("설정 변경: %s = %s", settingType, settingValue)
	default:
		log.Printf("알 수 없는 설정 타입: %s", settingType)
		http.Error(w, "Unknown setting type", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("설정 저장 실패: %v", err)
		http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("설정 저장 성공: %s = %s", settingType, settingValue)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Settings updated")
}

func (h *SettingsHandler) HandleLoadSettings(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("설정 로드 요청 받음")

	if h.Config == nil {
		log.Printf("Config가 nil입니다")
		http.Error(w, "Config not initialized", http.StatusInternalServerError)
		return
	}

	// 실제 설정값 반환
	settings := SettingsResponse{
		DarkMode:        h.Config.DarkMode,
		AutoStartup:     h.Config.AutoStartup,
		TelegramEnabled: h.Config.TelegramEnabled,
	}

	log.Printf("설정 로드 결과: DarkMode=%t, AutoStartup=%t, TelegramEnabled=%t",
		settings.DarkMode, settings.AutoStartup, settings.TelegramEnabled)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
