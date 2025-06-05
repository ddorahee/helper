package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"example.com/m/services"
)

// TelegramController는 텔레그램 관련 HTTP 요청을 처리합니다
type TelegramController struct {
	serviceContainer *services.ServiceContainer
}

// NewTelegramController는 새로운 텔레그램 컨트롤러를 생성합니다
func NewTelegramController(serviceContainer *services.ServiceContainer) *TelegramController {
	return &TelegramController{
		serviceContainer: serviceContainer,
	}
}

// Config는 텔레그램 설정 요청을 처리합니다
func (c *TelegramController) Config(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		token := r.FormValue("token")
		chatID := r.FormValue("chat_id")

		if token == "" || chatID == "" {
			http.Error(w, "Token과 Chat ID가 필요합니다", http.StatusBadRequest)
			return
		}

		err := c.serviceContainer.TelegramService.SaveConfig(token, chatID)
		if err != nil {
			http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "텔레그램 설정이 저장되었습니다")
		return
	}

	// GET 요청인 경우 현재 설정 반환
	status := c.serviceContainer.TelegramService.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Test는 텔레그램 연결 테스트 요청을 처리합니다
func (c *TelegramController) Test(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := c.serviceContainer.TelegramService.TestConnection()
	if err != nil {
		http.Error(w, fmt.Sprintf("테스트 실패: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "테스트 메시지가 전송되었습니다")
}
