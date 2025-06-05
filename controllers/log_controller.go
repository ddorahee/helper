package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"example.com/m/services"
)

// LogController는 로그 관련 HTTP 요청을 처리합니다
type LogController struct {
	serviceContainer *services.ServiceContainer
}

// NewLogController는 새로운 로그 컨트롤러를 생성합니다
func NewLogController(serviceContainer *services.ServiceContainer) *LogController {
	return &LogController{
		serviceContainer: serviceContainer,
	}
}

// Log는 로그 메시지 전송 요청을 처리합니다
func (c *LogController) Log(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var logData struct {
			Message string `json:"message"`
		}

		err := json.NewDecoder(r.Body).Decode(&logData)
		if err != nil {
			http.Error(w, "Invalid log data", http.StatusBadRequest)
			return
		}

		c.serviceContainer.LogService.AddLog(logData.Message)

		w.WriteHeader(http.StatusOK)
		return
	}

	// GET 요청인 경우 현재 로그 반환
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "로그 메시지"}`)
}

// Logs는 로그 목록 요청을 처리합니다
func (c *LogController) Logs(w http.ResponseWriter, r *http.Request) {
	logs, err := c.serviceContainer.LogService.GetLogs()
	if err != nil {
		http.Error(w, "Failed to read log file", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"logs": logs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ClearLogs는 로그 지우기 요청을 처리합니다
func (c *LogController) ClearLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := c.serviceContainer.LogService.ClearLogs()
	if err != nil {
		http.Error(w, "Failed to clear log file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"success": true}`)
}
