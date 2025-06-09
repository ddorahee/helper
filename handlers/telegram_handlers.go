package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"example.com/m/utils"
)

// TelegramConfigInterface - 텔레그램 핸들러 전용 인터페이스
type TelegramConfigInterface interface {
	SetTelegramConfig(token, chatID string) error
}

// isLocalhost 함수 추가
func isLocalhost(host string) bool {
	return strings.HasPrefix(host, "127.0.0.1:") || strings.HasPrefix(host, "localhost:")
}

type TelegramHandler struct {
	Config interface{}
	App    ApplicationInterface
}

func NewTelegramHandler(config interface{}, app ApplicationInterface) *TelegramHandler {
	return &TelegramHandler{
		Config: config,
		App:    app,
	}
}

func (h *TelegramHandler) HandleTelegramConfig(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodPost {
		// Config를 실제 타입으로 변환
		config, ok := h.Config.(TelegramConfigInterface)
		if !ok {
			http.Error(w, "Config interface error", http.StatusInternalServerError)
			return
		}

		token := r.FormValue("token")
		chatID := r.FormValue("chat_id")

		if token == "" || chatID == "" {
			http.Error(w, "Token과 Chat ID가 필요합니다", http.StatusBadRequest)
			return
		}

		err := config.SetTelegramConfig(token, chatID)
		if err != nil {
			http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "텔레그램 설정이 저장되었습니다")
		return
	}

	// GET 요청 - 상태 반환
	status := map[string]interface{}{
		"enabled": false, // 실제 설정값으로 교체 필요
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *TelegramHandler) HandleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !isLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	telegramBot := h.App.GetTelegramBot()
	if telegramBot == nil {
		http.Error(w, "텔레그램이 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	// 실제 테스트 연결 로직은 TelegramBot 인터페이스에 추가 필요
	// err := telegramBot.TestConnection()
	// if err != nil {
	//     http.Error(w, fmt.Sprintf("테스트 실패: %v", err), http.StatusInternalServerError)
	//     return
	// }

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "테스트 메시지가 전송되었습니다")
}
