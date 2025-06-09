package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"example.com/m/automation"
	"example.com/m/config"
	"example.com/m/telegram"
	"example.com/m/utils"
	webview "github.com/webview/webview_go"
)

//go:embed ui/web
var webFiles embed.FS

// Application 구조체 - 간소화된 버전
type Application struct {
	WebView         webview.WebView
	Config          *config.AppConfig
	TimerManager    *utils.TimerManager
	KeyboardManager *automation.KeyboardManager
	TelegramBot     *telegram.TelegramBot
	Server          *http.Server
	ServerPort      int
	AppContext      context.Context
	CancelFunc      context.CancelFunc
	AutoStopTimer   *time.Timer
}

// API 응답 구조체들
type APIResponse struct {
	Success bool        `json:"success,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type StatusResponse struct {
	Running bool `json:"running"`
	Mode    int  `json:"mode,omitempty"`
}

type SettingsResponse struct {
	DarkMode        bool `json:"dark_mode"`
	SoundEnabled    bool `json:"sound_enabled"`
	AutoStartup     bool `json:"auto_startup"`
	TelegramEnabled bool `json:"telegram_enabled"`
}

func main() {
	// 애플리케이션 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 애플리케이션 초기화
	app := &Application{
		Config:          config.NewAppConfig(),
		TimerManager:    utils.NewTimerManager(),
		KeyboardManager: automation.NewKeyboardManager(),
		AppContext:      ctx,
		CancelFunc:      cancel,
	}

	// 텔레그램 봇 초기화 (설정이 있는 경우)
	if app.Config.TelegramEnabled && app.Config.TelegramBot != nil {
		app.TelegramBot = app.Config.TelegramBot
	}

	// 로깅 설정
	setupLogging(app.Config)
	log.Println("애플리케이션 시작")

	// 사용 가능한 포트 찾기 (localhost 전용)
	app.ServerPort = findAvailablePort()
	if app.ServerPort == 0 {
		log.Fatal("사용 가능한 포트를 찾을 수 없습니다")
	}

	// 내장 HTTP 서버 시작
	serverReady := make(chan bool)
	go app.startEmbeddedServer(serverReady)

	// 서버 준비 대기
	select {
	case <-serverReady:
		log.Printf("내장 서버가 포트 %d에서 시작되었습니다", app.ServerPort)
	case <-time.After(5 * time.Second):
		log.Fatal("서버 시작 시간 초과")
	}

	// WebView 초기화 및 실행
	app.initWebView()

	// 정리 작업
	app.shutdown()
	log.Println("애플리케이션 종료")
}

// findAvailablePort는 사용 가능한 로컬 포트를 찾습니다
func findAvailablePort() int {
	for port := 8000; port <= 9000; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port
		}
	}
	return 0
}

// startEmbeddedServer는 내장 HTTP 서버를 시작합니다
func (app *Application) startEmbeddedServer(ready chan<- bool) {
	mux := http.NewServeMux()

	// 정적 파일 핸들러
	mux.HandleFunc("/", app.handleStaticFiles)

	// API 엔드포인트 설정
	mux.HandleFunc("/api/start", app.handleStart)
	mux.HandleFunc("/api/stop", app.handleStop)
	mux.HandleFunc("/api/status", app.handleStatus)
	mux.HandleFunc("/api/settings", app.handleSettings)
	mux.HandleFunc("/api/settings/load", app.handleLoadSettings)
	mux.HandleFunc("/api/reset", app.handleReset)
	mux.HandleFunc("/api/exit", app.handleExit)
	mux.HandleFunc("/api/logs", app.handleLogs)
	mux.HandleFunc("/api/logs/clear", app.handleClearLogs)
	mux.HandleFunc("/api/log", app.handleLog)
	mux.HandleFunc("/api/telegram/config", app.handleTelegramConfig)
	mux.HandleFunc("/api/telegram/test", app.handleTelegramTest)

	// 서버 설정 (localhost만 바인딩)
	app.Server = &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%d", app.ServerPort),
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 16, // 64KB
	}

	// 서버 준비 알림
	go func() {
		time.Sleep(100 * time.Millisecond)
		ready <- true
	}()

	// 서버 시작
	if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("서버 오류: %v", err)
	}
}

// initWebView는 WebView를 초기화하고 실행합니다
func (app *Application) initWebView() {
	// WebView 생성
	debug := app.Config.DevelopmentMode
	app.WebView = webview.New(debug)
	defer app.WebView.Destroy()

	// WebView 설정
	app.WebView.SetTitle("도우미")
	app.WebView.SetSize(1024, 768, webview.HintNone)

	// JavaScript API 바인딩
	app.bindJavaScriptAPI()

	// 페이지 로드
	url := fmt.Sprintf("http://127.0.0.1:%d", app.ServerPort)
	app.WebView.Navigate(url)

	// WebView 실행 (블로킹)
	app.WebView.Run()
}

// bindJavaScriptAPI는 JavaScript API를 바인딩합니다
func (app *Application) bindJavaScriptAPI() {
	// 앱 종료
	app.WebView.Bind("exitApp", func() {
		app.CancelFunc()
		app.WebView.Terminate()
	})

	// 로그 메시지
	app.WebView.Bind("logMessage", func(message string) {
		log.Printf("WebView: %s", message)
	})
}

// handleStaticFiles는 정적 파일을 서빙합니다
func (app *Application) handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	// 보안: localhost만 허용
	if !isLocalhost(r.Host, app.ServerPort) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// 파일 확장자 체크
	if !isAllowedFile(path) {
		http.Error(w, "File type not allowed", http.StatusForbidden)
		return
	}

	// embed.FS에서 파일 읽기
	filePath := "ui/web" + path
	content, err := webFiles.ReadFile(filePath)
	if err != nil {
		// SPA 라우팅을 위해 index.html 제공
		if path != "/index.html" {
			content, err = webFiles.ReadFile("ui/web/index.html")
			if err != nil {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
		} else {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
	}

	// Content-Type 및 보안 헤더 설정
	w.Header().Set("Content-Type", getContentType(path))
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	w.Write(content)
}

// API 핸들러들
func (app *Application) handleStart(w http.ResponseWriter, r *http.Request) {
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

	if app.TimerManager.IsRunning() {
		http.Error(w, "Already running", http.StatusConflict)
		return
	}

	// 타이머 시작
	app.TimerManager.Start()

	// 키보드 매니저 시작
	app.KeyboardManager.SetRunning(true)

	// 자동 중지 설정
	if autoStopHours > 0 {
		app.setupAutoStop(mode, autoStopHours)
	}

	// 자동화 시작
	go app.runAutomation(mode)

	// 텔레그램 알림
	if !isResume && app.Config.TelegramEnabled && app.TelegramBot != nil {
		go func() {
			modeName := getModeName(mode)
			duration := time.Duration(autoStopHours * float64(time.Hour))
			err := app.TelegramBot.SendStartNotification(modeName, duration)
			if err != nil {
				log.Printf("텔레그램 시작 알림 전송 실패: %v", err)
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Started")
}

func (app *Application) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !app.TimerManager.IsRunning() {
		http.Error(w, "Not running", http.StatusConflict)
		return
	}

	app.TimerManager.Stop()
	app.KeyboardManager.SetRunning(false)

	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		app.AutoStopTimer = nil
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Stopped")
}

func (app *Application) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := StatusResponse{
		Running: app.TimerManager.IsRunning(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (app *Application) handleSettings(w http.ResponseWriter, r *http.Request) {
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

	var err error
	switch settingType {
	case "dark_mode":
		enabled := settingValue == "1"
		err = app.Config.SetDarkMode(enabled)
	case "sound_enabled":
		enabled := settingValue == "1"
		err = app.Config.SetSoundEnabled(enabled)
	case "auto_startup":
		enabled := settingValue == "1"
		err = app.Config.SetAutoStartup(enabled)
	case "telegram_enabled":
		enabled := settingValue == "1"
		err = app.Config.SetTelegramEnabled(enabled)
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

func (app *Application) handleLoadSettings(w http.ResponseWriter, r *http.Request) {
	settings := SettingsResponse{
		DarkMode:        app.Config.DarkMode,
		SoundEnabled:    app.Config.SoundEnabled,
		AutoStartup:     app.Config.AutoStartup,
		TelegramEnabled: app.Config.TelegramEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (app *Application) handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if app.TimerManager.IsRunning() {
		http.Error(w, "Cannot reset while running", http.StatusConflict)
		return
	}

	app.TimerManager.Reset()

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Reset")
}

func (app *Application) handleExit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Exiting")

	// 지연 후 종료
	go func() {
		time.Sleep(500 * time.Millisecond)
		app.CancelFunc()
		if app.WebView != nil {
			app.WebView.Terminate()
		}
		os.Exit(0)
	}()
}

func (app *Application) handleLog(w http.ResponseWriter, r *http.Request) {
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

func (app *Application) handleLogs(w http.ResponseWriter, r *http.Request) {
	logContent, err := readLogFile(app.Config)
	if err != nil {
		http.Error(w, "Failed to read log file", http.StatusInternalServerError)
		return
	}

	logs := splitLogToLines(logContent)
	maxEntries := 100
	if len(logs) > maxEntries {
		logs = logs[len(logs)-maxEntries:]
	}

	response := map[string]interface{}{
		"logs": logs,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (app *Application) handleClearLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := clearLogFile(app.Config)
	if err != nil {
		http.Error(w, "Failed to clear log file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"success": true}`)
}

func (app *Application) handleTelegramConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		token := r.FormValue("token")
		chatID := r.FormValue("chat_id")

		if token == "" || chatID == "" {
			http.Error(w, "Token과 Chat ID가 필요합니다", http.StatusBadRequest)
			return
		}

		err := app.Config.SetTelegramConfig(token, chatID)
		if err != nil {
			http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
			return
		}

		app.TelegramBot = app.Config.TelegramBot

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "텔레그램 설정이 저장되었습니다")
		return
	}

	status := map[string]interface{}{
		"enabled": app.Config.TelegramEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (app *Application) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !app.Config.TelegramEnabled || app.TelegramBot == nil {
		http.Error(w, "텔레그램이 설정되지 않았습니다", http.StatusBadRequest)
		return
	}

	err := app.TelegramBot.TestConnection()
	if err != nil {
		http.Error(w, fmt.Sprintf("테스트 실패: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "테스트 메시지가 전송되었습니다")
}

// 헬퍼 함수들
func (app *Application) setupAutoStop(mode string, hours float64) {
	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		app.AutoStopTimer = nil
	}

	if hours <= 0 {
		return
	}

	duration := time.Duration(hours * float64(time.Hour))
	app.AutoStopTimer = time.AfterFunc(duration, func() {
		if app.TimerManager.IsRunning() {
			modeName := getModeName(mode)

			app.TimerManager.Stop()
			app.KeyboardManager.SetRunning(false)

			if app.Config.TelegramEnabled && app.TelegramBot != nil {
				go func() {
					err := app.TelegramBot.SendCompletionNotification(modeName, duration)
					if err != nil {
						log.Printf("텔레그램 완료 알림 전송 실패: %v", err)
					}
				}()
			}

			log.Printf("작업 완료: %s 모드, %v 실행", modeName, duration)
		}
	})
}

func (app *Application) runAutomation(mode string) {
	switch mode {
	case "daeya-entrance":
		app.KeyboardManager.DaeyaEnter()
	case "daeya-party":
		app.KeyboardManager.DaeyaParty()
	case "kanchen-entrance":
		app.KeyboardManager.KanchenEnter()
	case "kanchen-party":
		app.KeyboardManager.KanchenParty()
	default:
		log.Printf("알 수 없는 모드: %s", mode)
	}
}

func (app *Application) shutdown() {
	log.Println("애플리케이션 종료 중...")

	if app.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := app.Server.Shutdown(ctx); err != nil {
			log.Printf("서버 종료 오류: %v", err)
		}
	}

	if app.TimerManager != nil && app.TimerManager.IsRunning() {
		app.TimerManager.Stop()
	}

	if app.KeyboardManager != nil && app.KeyboardManager.IsRunning() {
		app.KeyboardManager.SetRunning(false)
	}

	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
	}
}

// 유틸리티 함수들
func isLocalhost(host string, port int) bool {
	expectedHosts := []string{
		fmt.Sprintf("127.0.0.1:%d", port),
		fmt.Sprintf("localhost:%d", port),
	}

	for _, expectedHost := range expectedHosts {
		if host == expectedHost {
			return true
		}
	}
	return false
}

func isAllowedFile(path string) bool {
	allowedExtensions := []string{
		".html", ".css", ".js", ".json", ".png", ".jpg", ".jpeg",
		".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot",
	}

	for _, ext := range allowedExtensions {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true
		}
	}
	return path == "/" || path == "/index.html"
}

func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	default:
		return "application/octet-stream"
	}
}

func getModeName(mode string) string {
	switch mode {
	case "daeya-entrance":
		return "대야 (입장)"
	case "daeya-party":
		return "대야 (파티)"
	case "kanchen-entrance":
		return "칸첸 (입장)"
	case "kanchen-party":
		return "칸첸 (파티)"
	default:
		return "알 수 없음"
	}
}

func setupLogging(appConfig *config.AppConfig) {
	logFile := appConfig.GetLogFilePath()

	logDir := filepath.Dir(logFile)
	if !dirExists(logDir) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			fmt.Printf("경고: 로그 디렉토리를 생성할 수 없습니다: %v\n", err)
		}
	}

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("경고: 로그 파일을 열 수 없습니다: %v\n", err)
		return
	}

	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func dirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func readLogFile(appConfig *config.AppConfig) (string, error) {
	logFilePath := appConfig.GetLogFilePath()
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func clearLogFile(appConfig *config.AppConfig) error {
	logFilePath := appConfig.GetLogFilePath()
	return os.WriteFile(logFilePath, []byte(""), 0666)
}

func splitLogToLines(logContent string) []string {
	lines := strings.Split(logContent, "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	reverseSlice(result)
	return result
}

func reverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
