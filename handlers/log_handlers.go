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

type LogHandler struct {
	Config interface{}
}

func NewLogHandler(config interface{}) *LogHandler {
	return &LogHandler{
		Config: config,
	}
}

func (h *LogHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	// CORS 헤더 추가
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	// 보안: localhost만 허용
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Config를 실제 타입으로 변환
	config, ok := h.Config.(ConfigInterface)
	if !ok {
		http.Error(w, "Config interface error", http.StatusInternalServerError)
		return
	}

	logFilePath := config.GetLogFilePath()
	log.Printf("로그 파일 경로: %s", logFilePath)

	// 로그 파일 존재 여부 확인
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		log.Printf("로그 파일이 존재하지 않음, 기본 로그 생성")
		// 기본 로그 메시지들
		defaultLogs := []string{
			fmt.Sprintf("%s [INFO] 프로그램이 시작되었습니다.", time.Now().Format("2006-01-02 15:04:05")),
			fmt.Sprintf("%s [INFO] 로그 시스템이 초기화되었습니다.", time.Now().Format("2006-01-02 15:04:05")),
			fmt.Sprintf("%s [DEBUG] 로그 파일 경로: %s", time.Now().Format("2006-01-02 15:04:05"), logFilePath),
		}

		response := map[string]interface{}{
			"logs":    defaultLogs,
			"success": true,
			"message": "기본 로그를 생성했습니다.",
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	// 로그 파일 읽기 시도
	logContent, err := utils.ReadLogFile(config)
	if err != nil {
		log.Printf("로그 파일 읽기 실패: %v", err)
		errorLogs := []string{
			fmt.Sprintf("%s [ERROR] 로그 파일 읽기 실패: %v", time.Now().Format("2006-01-02 15:04:05"), err),
			fmt.Sprintf("%s [INFO] 로그 파일 경로: %s", time.Now().Format("2006-01-02 15:04:05"), logFilePath),
			fmt.Sprintf("%s [INFO] 새로운 로그 파일이 생성됩니다.", time.Now().Format("2006-01-02 15:04:05")),
		}

		response := map[string]interface{}{
			"logs":    errorLogs,
			"success": false,
			"message": "로그 파일 읽기 실패",
		}

		json.NewEncoder(w).Encode(response)
		return
	}

	logs := utils.SplitLogToLines(logContent)

	// 로그가 비어있는 경우 기본 메시지 추가
	if len(logs) == 0 {
		logs = []string{
			fmt.Sprintf("%s [INFO] 로그가 비어있습니다.", time.Now().Format("2006-01-02 15:04:05")),
			fmt.Sprintf("%s [INFO] 새로운 활동이 시작되면 여기에 표시됩니다.", time.Now().Format("2006-01-02 15:04:05")),
		}
	}

	// 최대 100개로 제한
	maxEntries := 100
	if len(logs) > maxEntries {
		logs = logs[len(logs)-maxEntries:]
	}

	log.Printf("반환할 로그 개수: %d", len(logs))

	response := map[string]interface{}{
		"logs":    logs,
		"success": true,
		"count":   len(logs),
	}

	json.NewEncoder(w).Encode(response)
}

func (h *LogHandler) HandleClearLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 보안: localhost만 허용
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Config를 실제 타입으로 변환
	config, ok := h.Config.(ConfigInterface)
	if !ok {
		http.Error(w, "Config interface error", http.StatusInternalServerError)
		return
	}

	err := utils.ClearLogFile(config)
	if err != nil {
		http.Error(w, "Failed to clear log file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"success": true}`)
}

func (h *LogHandler) HandleLog(w http.ResponseWriter, r *http.Request) {
	// 보안: localhost만 허용
	if !utils.IsLocalhost(r.Host) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodPost {
		var logData struct {
			Message string `json:"message"`
		}

		err := json.NewDecoder(r.Body).Decode(&logData)
		if err != nil {
			http.Error(w, "Invalid log data", http.StatusBadRequest)
			return
		}

		log.Println(logData.Message)
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "로그 메시지"}`)
}
