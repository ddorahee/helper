// handlers/api_handlers.go 수정
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/m/utils"
)

type APIHandler struct {
	App ApplicationInterface
}

func NewAPIHandler(app ApplicationInterface) *APIHandler {
	return &APIHandler{
		App: app,
	}
}

func (h *APIHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 보안 체크
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	mode := r.FormValue("mode")
	if mode == "" {
		http.Error(w, "Mode not specified", http.StatusBadRequest)
		return
	}

	autoStopStr := r.FormValue("auto_stop")
	var autoStopHours float64 = 0
	if autoStopStr != "" {
		fmt.Sscanf(autoStopStr, "%f", &autoStopHours)
	}

	isResume := r.FormValue("resume") == "true"

	log.Printf("시작 요청 받음: mode=%s, autoStop=%.2f시간, resume=%t", mode, autoStopHours, isResume)

	// 타입 어설션을 사용하여 실제 타입으로 변환
	timerManager, ok := h.App.GetTimerManager().(TimerManagerInterface)
	if !ok {
		log.Printf("Timer manager interface error")
		http.Error(w, "Timer manager interface error", http.StatusInternalServerError)
		return
	}

	keyboardManager, ok := h.App.GetKeyboardManager().(KeyboardManagerInterface)
	if !ok {
		log.Printf("Keyboard manager interface error")
		http.Error(w, "Keyboard manager interface error", http.StatusInternalServerError)
		return
	}

	// 상태 검사 및 강제 정리
	if timerManager.IsRunning() {
		log.Printf("이미 실행 중인 타이머 발견, 강제 정리 시도")

		// 강제로 이전 작업 정리
		timerManager.Stop()
		keyboardManager.SetRunning(false)

		// 잠시 대기 (상태 정리 시간)
		time.Sleep(100 * time.Millisecond)

		// 다시 한번 확인
		if timerManager.IsRunning() {
			log.Printf("타이머 정리 실패, 409 에러 반환")
			http.Error(w, "Already running", http.StatusConflict)
			return
		}

		log.Printf("이전 작업 정리 완료")
	}

	// 키보드 매니저 상태도 확인
	if keyboardManager.IsRunning() {
		log.Printf("키보드 매니저가 실행 중, 강제 정리")
		keyboardManager.SetRunning(false)
		time.Sleep(100 * time.Millisecond)
	}

	// 타이머 및 키보드 매니저 시작
	log.Printf("새 작업 시작")
	timerManager.Start()
	keyboardManager.SetRunning(true)

	// 자동 중지 설정
	if autoStopHours > 0 {
		log.Printf("자동 중지 설정: %.2f시간", autoStopHours)
		h.App.SetupAutoStop(mode, autoStopHours)
	}

	// 자동화 시작
	go h.App.RunAutomation(mode)

	// 텔레그램 시작 알림 (재개가 아닌 경우에만)
	if !isResume {
		telegramBot := h.App.GetTelegramBot()
		if telegramBot != nil {
			// 텔레그램 봇을 실제 타입으로 변환하여 알림 전송
			if bot, ok := telegramBot.(TelegramBotInterface); ok {
				go func() {
					modeName := utils.GetModeName(mode)
					duration := time.Duration(autoStopHours * float64(time.Hour))

					err := bot.SendStartNotification(modeName, duration)
					if err != nil {
						log.Printf("텔레그램 시작 알림 전송 실패: %v", err)
					} else {
						log.Printf("텔레그램 시작 알림 전송 성공: %s", modeName)
					}
				}()
			}
		}
	}

	log.Printf("작업 시작 완료: %s", mode)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Started")
}

func (h *APIHandler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("중지 요청 받음")

	timerManager, ok := h.App.GetTimerManager().(TimerManagerInterface)
	if !ok {
		http.Error(w, "Timer manager interface error", http.StatusInternalServerError)
		return
	}

	keyboardManager, ok := h.App.GetKeyboardManager().(KeyboardManagerInterface)
	if !ok {
		http.Error(w, "Keyboard manager interface error", http.StatusInternalServerError)
		return
	}

	// 실행 중이 아니어도 강제로 중지 (상태 정리)
	log.Printf("작업 중지 실행")
	timerManager.Stop()
	keyboardManager.SetRunning(false)

	log.Printf("작업 중지 완료")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Stopped")
}

func (h *APIHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	timerManager, ok := h.App.GetTimerManager().(TimerManagerInterface)
	if !ok {
		http.Error(w, "Timer manager interface error", http.StatusInternalServerError)
		return
	}

	keyboardManager, ok := h.App.GetKeyboardManager().(KeyboardManagerInterface)
	if !ok {
		http.Error(w, "Keyboard manager interface error", http.StatusInternalServerError)
		return
	}

	// 상태 동기화 체크
	timerRunning := timerManager.IsRunning()
	keyboardRunning := keyboardManager.IsRunning()

	// 둘 중 하나라도 실행 중이면 실행 중으로 판단
	isRunning := timerRunning || keyboardRunning

	log.Printf("상태 조회: timer=%t, keyboard=%t, result=%t", timerRunning, keyboardRunning, isRunning)

	status := StatusResponse{
		Running: isRunning,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *APIHandler) HandleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("리셋 요청 받음")

	timerManager, ok := h.App.GetTimerManager().(TimerManagerInterface)
	if !ok {
		http.Error(w, "Timer manager interface error", http.StatusInternalServerError)
		return
	}

	keyboardManager, ok := h.App.GetKeyboardManager().(KeyboardManagerInterface)
	if !ok {
		http.Error(w, "Keyboard manager interface error", http.StatusInternalServerError)
		return
	}

	// 강제로 모든 것을 중지하고 리셋
	log.Printf("강제 중지 및 리셋 실행")
	timerManager.Stop()
	keyboardManager.SetRunning(false)

	// 잠시 대기
	time.Sleep(100 * time.Millisecond)

	// 타이머 리셋
	timerManager.Reset()

	log.Printf("리셋 완료")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Reset")
}

func (h *APIHandler) HandleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	log.Printf("종료 요청 받음")

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Exiting")

	// 지연 후 종료
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
