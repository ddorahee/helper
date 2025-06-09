package handlers

import (
	"encoding/json"
	"fmt"
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

	// 타입 어설션을 사용하여 실제 타입으로 변환
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

	if timerManager.IsRunning() {
		http.Error(w, "Already running", http.StatusConflict)
		return
	}

	// 타이머 및 키보드 매니저 시작
	timerManager.Start()
	keyboardManager.SetRunning(true)

	// 자동 중지 설정
	if autoStopHours > 0 {
		h.App.SetupAutoStop(mode, autoStopHours)
	}

	// 자동화 시작
	go h.App.RunAutomation(mode)

	// 텔레그램 알림
	if !isResume {
		telegramBot := h.App.GetTelegramBot()
		if telegramBot != nil {
			go func() {
				// 텔레그램 알림 로직 (기존 코드에서 가져옴)
			}()
		}
	}

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

	if !timerManager.IsRunning() {
		http.Error(w, "Not running", http.StatusConflict)
		return
	}

	timerManager.Stop()
	keyboardManager.SetRunning(false)

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

	status := StatusResponse{
		Running: timerManager.IsRunning(),
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

	timerManager, ok := h.App.GetTimerManager().(TimerManagerInterface)
	if !ok {
		http.Error(w, "Timer manager interface error", http.StatusInternalServerError)
		return
	}

	if timerManager.IsRunning() {
		http.Error(w, "Cannot reset while running", http.StatusConflict)
		return
	}

	timerManager.Reset()

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

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Exiting")

	// 지연 후 종료
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}
