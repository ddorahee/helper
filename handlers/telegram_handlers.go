// handlers/telegram_handlers.go 수정
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"example.com/m/config"
	"example.com/m/utils"
)

// isLocalhost 함수 추가
func isLocalhost(host string) bool {
	return strings.HasPrefix(host, "127.0.0.1:") || strings.HasPrefix(host, "localhost:")
}

type TelegramHandler struct {
	Config *config.AppConfig // 직접 AppConfig 타입 사용
	App    ApplicationInterface
}

func NewTelegramHandler(appConfig *config.AppConfig, app ApplicationInterface) *TelegramHandler {
	return &TelegramHandler{
		Config: appConfig, // 직접 AppConfig 전달
		App:    app,
	}
}

func (h *TelegramHandler) HandleTelegramConfig(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodPost {
		// Config 직접 사용
		if h.Config == nil {
			log.Printf("텔레그램 핸들러: Config가 nil입니다")
			http.Error(w, "Config not initialized", http.StatusInternalServerError)
			return
		}

		token := r.FormValue("token")
		chatID := r.FormValue("chat_id")

		log.Printf("텔레그램 설정 요청: token=%s..., chatID=%s",
			func() string {
				if len(token) > 10 {
					return token[:10]
				}
				return token
			}(), chatID)

		if token == "" || chatID == "" {
			log.Printf("텔레그램 설정: 필수 필드 누락")
			http.Error(w, "Token과 Chat ID가 필요합니다", http.StatusBadRequest)
			return
		}

		err := h.Config.SetTelegramConfig(token, chatID)
		if err != nil {
			log.Printf("텔레그램 설정 저장 실패: %v", err)
			http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("텔레그램 설정 저장 성공")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "텔레그램 설정이 저장되었습니다")
		return
	}

	// GET 요청 - 상태 반환
	if h.Config == nil {
		log.Printf("텔레그램 상태 조회: Config가 nil입니다")
		http.Error(w, "Config not initialized", http.StatusInternalServerError)
		return
	}

	status := map[string]interface{}{
		"enabled": h.Config.IsTelegramEnabled(),
	}

	log.Printf("텔레그램 상태 조회 결과: enabled=%t", h.Config.IsTelegramEnabled())

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

	log.Printf("텔레그램 테스트 요청 받음")

	// 텔레그램 봇 가져오기
	telegramBot := h.App.GetTelegramBot()
	if telegramBot == nil {
		log.Printf("텔레그램 테스트: 봇이 설정되지 않음")
		http.Error(w, "텔레그램이 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	// 텔레그램 봇을 실제 타입으로 변환
	if bot, ok := telegramBot.(TelegramBotInterface); ok {
		err := bot.TestConnection()
		if err != nil {
			log.Printf("텔레그램 테스트 실패: %v", err)
			http.Error(w, fmt.Sprintf("테스트 실패: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("텔레그램 테스트 성공")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "테스트 메시지가 전송되었습니다")
	} else {
		log.Printf("텔레그램 테스트: 봇 인터페이스 변환 실패")
		http.Error(w, "텔레그램 봇 인터페이스 오류", http.StatusInternalServerError)
	}
}
