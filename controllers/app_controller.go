package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"example.com/m/services"
)

// AppController는 앱 관련 HTTP 요청을 처리합니다
type AppController struct {
	serviceContainer *services.ServiceContainer
}

// NewAppController는 새로운 앱 컨트롤러를 생성합니다
func NewAppController(serviceContainer *services.ServiceContainer) *AppController {
	return &AppController{
		serviceContainer: serviceContainer,
	}
}

// Start는 매크로 시작 요청을 처리합니다
func (c *AppController) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	err := c.serviceContainer.AppService.StartOperation(mode, autoStopHours, isResume)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Started")
}

// Stop은 매크로 중지 요청을 처리합니다
func (c *AppController) Stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := c.serviceContainer.AppService.StopOperation()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Stopped")
}

// Reset은 설정 재설정 요청을 처리합니다
func (c *AppController) Reset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := c.serviceContainer.AppService.ResetSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Reset")
}

// Exit는 애플리케이션 종료 요청을 처리합니다
func (c *AppController) Exit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c.serviceContainer.AppService.ExitApplication()

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Exiting")
}

// Status는 현재 상태 요청을 처리합니다
func (c *AppController) Status(w http.ResponseWriter, r *http.Request) {
	status := c.serviceContainer.AppService.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Settings는 설정 저장 요청을 처리합니다
func (c *AppController) Settings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settingType := r.FormValue("type")
	settingValue := r.FormValue("value")

	if settingType == "" || settingValue == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	err := c.serviceContainer.AppService.SaveSetting(settingType, settingValue)
	if err != nil {
		http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Settings updated")
}

// LoadSettings는 저장된 설정 로드 요청을 처리합니다
func (c *AppController) LoadSettings(w http.ResponseWriter, r *http.Request) {
	settings := c.serviceContainer.AppService.GetSettings()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
