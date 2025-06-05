package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"example.com/m/automation"
	"example.com/m/config"
	"example.com/m/utils"
	webview "github.com/webview/webview_go"
)

//go:embed ui/web
var webFiles embed.FS

// 모드 타입 상수
const (
	ModeNone = iota
	ModeDaeyaEnter
	ModeDaeyaParty
	ModeKanchenEnter
	ModeKanchenParty
)

// 시간 설정 옵션 상수
const (
	TimeOption1Hour = iota
	TimeOption2Hour
	TimeOption3Hour
	TimeOption4Hour
)

// Application 구조체는 애플리케이션의 상태를 관리합니다
type Application struct {
	WebView          webview.WebView
	Config           *config.AppConfig
	TimerManager     *utils.TimerManager
	KeyboardManager  *automation.KeyboardManager
	ActiveMode       int
	TimeOption       int
	AutoStopTimer    *time.Timer
	WindowWidth      int
	WindowHeight     int
	RunningOperation bool
	AutoStartup      bool
	ServerPort       string
	ServerReady      chan bool
}

// 웹뷰에 전송할 이벤트 구조체
type UIEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// 타이머 이벤트 페이로드
type TimerPayload struct {
	Time      string `json:"time"`
	IsRunning bool   `json:"isRunning"`
}

// 로그 이벤트 페이로드
type LogPayload struct {
	Message string `json:"message"`
}

// 모드 변경 이벤트 페이로드
type ModePayload struct {
	Mode int `json:"mode"`
}

// 버전 정보 페이로드
type VersionPayload struct {
	Version   string `json:"version"`
	BuildDate string `json:"buildDate"`
}

func setupLogging(appConfig *config.AppConfig) {
	logFile := appConfig.GetLogFilePath()

	// 로그 파일의 디렉토리 생성 (이미 config에서 생성되지만 안전장치)
	logDir := filepath.Dir(logFile)
	if !dirExists(logDir) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			fmt.Printf("경고: 로그 디렉토리를 생성할 수 없습니다: %v\n", err)
		}
	}

	// 로그 파일 열기
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("경고: 로그 파일을 열 수 없습니다: %v\n", err)
		return
	}

	// 표준 로그 설정
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func main() {
	// 애플리케이션 생성 (로그 설정보다 먼저)
	app := NewApplication()

	// 로그 파일 설정 - AppConfig를 매개변수로 전달
	setupLogging(app.Config)

	// 키보드 매니저 생성
	keyboardManager := automation.NewKeyboardManager()
	app.KeyboardManager = keyboardManager

	// 타이머 매니저 생성
	timerManager := utils.NewTimerManager()
	app.TimerManager = timerManager

	// HTTP 서버 시작
	go startServer(app, timerManager, keyboardManager)

	// 서버 준비될 때까지 대기
	<-app.ServerReady

	// 애플리케이션 초기화 및 실행
	log.Println("애플리케이션 초기화 시작")

	// 웹뷰 초기화
	app.WebView = webview.New(true)
	app.WebView.SetTitle("도우미")
	app.WebView.SetSize(app.WindowWidth, app.WindowHeight, webview.HintNone)

	// 콜백 함수 바인딩
	bindJavaScriptCallbacks(app)

	// 웹뷰에 URL 로드
	app.WebView.Navigate(fmt.Sprintf("http://localhost:%s", app.ServerPort))

	// 앱 버전 정보 전송
	time.AfterFunc(1*time.Second, func() {
		sendEvent(app, "appVersion", VersionPayload{
			Version:   app.Config.Version,
			BuildDate: app.Config.BuildDate,
		})
	})

	log.Println("애플리케이션 초기화 완료")
	log.Println("애플리케이션 실행 시작")

	// 애플리케이션 실행
	app.WebView.Run()

	log.Println("애플리케이션 종료")
}

// NewApplication은 새로운 애플리케이션 인스턴스를 생성합니다
func NewApplication() *Application {
	return &Application{
		Config:           config.NewAppConfig(),
		ActiveMode:       ModeDaeyaEnter,  // 기본값: 대야 (입장)
		TimeOption:       TimeOption3Hour, // 기본값: 3시간 10분
		WindowWidth:      1024,
		WindowHeight:     768,
		RunningOperation: false,
		AutoStartup:      false,
		ServerPort:       "8080",
		ServerReady:      make(chan bool), // 서버 준비 상태를 알리는 채널
	}
}

// 웹 서버 시작
func startServer(app *Application, timerManager *utils.TimerManager, keyboardManager *automation.KeyboardManager) {
	// 정적 파일 제공 핸들러
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/ui/web/index.html"
		} else {
			path = "/ui/web" + path
		}

		// 정적 파일 제공
		content, err := webFiles.ReadFile(strings.TrimPrefix(path, "/"))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// 콘텐츠 유형 설정
		contentType := "text/html"
		if strings.HasSuffix(path, ".css") {
			contentType = "text/css"
		} else if strings.HasSuffix(path, ".js") {
			contentType = "application/javascript"
		} else if strings.HasSuffix(path, ".svg") {
			contentType = "image/svg+xml"
		} else if strings.HasSuffix(path, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
			contentType = "image/jpeg"
		}

		w.Header().Set("Content-Type", contentType)
		w.Write(content)
	})

	// API 엔드포인트 설정
	setupAPIHandlers(app, keyboardManager, timerManager)

	// 서버 시작
	log.Printf("웹 서버를 포트 %s에서 시작합니다...", app.ServerPort)
	go func() {
		app.ServerReady <- true // 서버 준비 완료 알림
	}()

	if err := http.ListenAndServe(fmt.Sprintf(":%s", app.ServerPort), nil); err != nil {
		log.Printf("서버 시작 오류: %v", err)
		os.Exit(1)
	}
}

// API 핸들러 설정
func setupAPIHandlers(app *Application, km *automation.KeyboardManager, tm *utils.TimerManager) {
	// 시작 API - 수정된 버전
	http.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 모드 파라미터 가져오기
		mode := r.FormValue("mode")
		if mode == "" {
			http.Error(w, "Mode not specified", http.StatusBadRequest)
			return
		}

		// 자동 종료 시간 파라미터 가져오기 (옵션) - float64로 수정
		autoStopStr := r.FormValue("auto_stop")
		var autoStopHours float64 = 0
		if autoStopStr != "" {
			fmt.Sscanf(autoStopStr, "%f", &autoStopHours)
		}

		// 재시작 여부 확인 (추가)
		isResume := r.FormValue("resume") == "true"

		// 현재 실행 중인지 확인
		if tm.IsRunning() {
			http.Error(w, "Already running", http.StatusConflict)
			return
		}

		// 모드에 따라 내부 모드 설정
		internalMode := ModeDaeyaEnter // 기본값
		if mode == "daeya-entrance" {
			internalMode = ModeDaeyaEnter
		} else if mode == "daeya-party" {
			internalMode = ModeDaeyaParty
		} else if mode == "kanchen-entrance" {
			internalMode = ModeKanchenEnter
		} else if mode == "kanchen-party" {
			internalMode = ModeKanchenParty
		}

		// 애플리케이션 설정 업데이트
		app.ActiveMode = internalMode

		// 타이머 시작
		tm.Start()

		// 키보드 매니저 설정
		km.SetRunning(true)

		// 상태 업데이트
		app.RunningOperation = true
		sendEvent(app, "operationStatus", map[string]bool{"running": true})

		// 자동 중지 설정
		if autoStopHours > 0 {
			setupAutoStop(app, autoStopHours)
		}

		// 선택된 모드에 따라 자동화 시작
		go func() {
			switch internalMode {
			case ModeDaeyaEnter:
				km.DaeyaEnter()
			case ModeDaeyaParty:
				km.DaeyaParty()
			case ModeKanchenEnter:
				km.KanchenEnter()
			case ModeKanchenParty:
				km.KanchenParty()
			}
		}()

		// 텔레그램 알림 전송 - 재시작이 아닐 때만 시작 알림 전송
		if app.Config.TelegramEnabled && app.Config.TelegramBot != nil && !isResume {
			modeName := getModeName(internalMode)
			duration := time.Duration(autoStopHours * float64(time.Hour))
			go func() {
				err := app.Config.TelegramBot.SendStartNotification(modeName, duration)
				if err != nil {
					log.Printf("텔레그램 시작 알림 전송 실패: %v", err)
				}
			}()
		} else if isResume {
			// 재시작 로그만 기록
			log.Printf("작업 재개: %s 모드", getModeName(internalMode))
		}

		// 응답 전송
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Started")
	})

	// 중지 API
	http.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 현재 실행 중인지 확인
		if !tm.IsRunning() {
			http.Error(w, "Not running", http.StatusConflict)
			return
		}

		// 타이머 중지
		tm.Stop()

		// 키보드 매니저 중지
		km.SetRunning(false)

		// 상태 업데이트
		app.RunningOperation = false
		sendEvent(app, "operationStatus", map[string]bool{"running": false})

		// 자동 중지 타이머 중지
		if app.AutoStopTimer != nil {
			app.AutoStopTimer.Stop()
			app.AutoStopTimer = nil
		}

		// 응답 전송
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Stopped")
	})

	// 설정 API
	http.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
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

		// 설정 타입에 따라 처리
		switch settingType {
		case "mode":
			// 모드 설정
			if settingValue == "daeya-entrance" {
				app.ActiveMode = ModeDaeyaEnter
			} else if settingValue == "daeya-party" {
				app.ActiveMode = ModeDaeyaParty
			} else if settingValue == "kanchen-entrance" {
				app.ActiveMode = ModeKanchenEnter
			} else if settingValue == "kanchen-party" {
				app.ActiveMode = ModeKanchenParty
			}
		case "time":
			// 시간 설정 - float64로 처리하도록 수정
			var hours float64
			fmt.Sscanf(settingValue, "%f", &hours)
			// 10분 추가된 시간으로 시간 옵션 결정
			if hours >= 1.0 && hours < 1.5 {
				app.TimeOption = TimeOption1Hour // 1시간 10분
			} else if hours >= 2.0 && hours < 2.5 {
				app.TimeOption = TimeOption2Hour // 2시간 10분
			} else if hours >= 3.0 && hours < 3.5 {
				app.TimeOption = TimeOption3Hour // 3시간 10분
			} else if hours >= 4.0 && hours < 4.5 {
				app.TimeOption = TimeOption4Hour // 4시간 10분
			}
		case "dark_mode":
			var enabled int
			fmt.Sscanf(settingValue, "%d", &enabled)
			err = app.Config.SetDarkMode(enabled == 1)
		case "sound_enabled":
			var enabled int
			fmt.Sscanf(settingValue, "%d", &enabled)
			err = app.Config.SetSoundEnabled(enabled == 1)
		case "auto_startup":
			var enabled int
			fmt.Sscanf(settingValue, "%d", &enabled)
			err = app.Config.SetAutoStartup(enabled == 1)
		case "telegram_enabled":
			var enabled int
			fmt.Sscanf(settingValue, "%d", &enabled)
			err = app.Config.SetTelegramEnabled(enabled == 1)
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Settings updated")
	})

	// 설정 로드 API 추가 - 페이지 로드 시 저장된 설정 불러오기
	http.HandleFunc("/api/settings/load", func(w http.ResponseWriter, r *http.Request) {
		settings := map[string]interface{}{
			"dark_mode":        app.Config.DarkMode,
			"sound_enabled":    app.Config.SoundEnabled,
			"auto_startup":     app.Config.AutoStartup,
			"telegram_enabled": app.Config.TelegramEnabled,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	})

	// 상태 API
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		// 상태 정보 구성
		status := map[string]interface{}{
			"running": tm.IsRunning(),
			"mode":    app.ActiveMode,
		}

		// JSON 응답 전송
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// 로그 API
	http.HandleFunc("/api/log", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// 로그 메시지 받기
			var logData struct {
				Message string `json:"message"`
			}

			err := json.NewDecoder(r.Body).Decode(&logData)
			if err != nil {
				http.Error(w, "Invalid log data", http.StatusBadRequest)
				return
			}

			// 로그 메시지 기록
			log.Println(logData.Message)

			w.WriteHeader(http.StatusOK)
			return
		}

		// GET 요청인 경우 현재 로그 반환
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"message": "로그 메시지"}`) // 현재 로그를 가져오는 함수가 없으므로 임시 값 사용
	})

	// 로그 API 핸들러
	http.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		// 로그 파일 읽기
		logContent, err := readLogFile(app.Config)
		if err != nil {
			http.Error(w, "Failed to read log file", http.StatusInternalServerError)
			return
		}

		// 로그 항목 분할 및 역순 정렬 (최신 로그가 위로)
		logs := splitLogToLines(logContent)

		// 마지막 100개 항목만 보여주기 (로그가 너무 많을 경우)
		maxEntries := 100
		if len(logs) > maxEntries {
			logs = logs[len(logs)-maxEntries:]
		}

		// JSON 응답 생성
		response := map[string]interface{}{
			"logs": logs,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// 로그 지우기 API
	http.HandleFunc("/api/logs/clear", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 로그 파일 지우기
		err := clearLogFile(app.Config)
		if err != nil {
			http.Error(w, "Failed to clear log file", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"success": true}`)
	})

	// 텔레그램 설정 API
	http.HandleFunc("/api/telegram/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// 텔레그램 설정 저장
			token := r.FormValue("token")
			chatID := r.FormValue("chat_id")

			if token == "" || chatID == "" {
				http.Error(w, "Token과 Chat ID가 필요합니다", http.StatusBadRequest)
				return
			}

			// 설정 저장 (파일에 자동 저장됨)
			err := app.Config.SetTelegramConfig(token, chatID)
			if err != nil {
				http.Error(w, fmt.Sprintf("설정 저장 실패: %v", err), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "텔레그램 설정이 저장되었습니다")
			return
		}

		// GET 요청인 경우 현재 설정 반환
		status := map[string]interface{}{
			"enabled": app.Config.TelegramEnabled,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})

	// 텔레그램 테스트 API
	http.HandleFunc("/api/telegram/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !app.Config.TelegramEnabled || app.Config.TelegramBot == nil {
			http.Error(w, "텔레그램이 설정되지 않았습니다", http.StatusBadRequest)
			return
		}

		err := app.Config.TelegramBot.TestConnection()
		if err != nil {
			http.Error(w, fmt.Sprintf("테스트 실패: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "테스트 메시지가 전송되었습니다")
	})

	// 재설정 API
	http.HandleFunc("/api/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 타이머 재설정
		tm.Reset()

		// 모드 초기화 - 대야 입장(기본값)으로 설정
		app.ActiveMode = ModeDaeyaEnter
		sendEvent(app, "resetMode", ModePayload{Mode: ModeDaeyaEnter})

		// 시간 설정 초기화 - 3시간 10분(기본값)으로 설정
		app.TimeOption = TimeOption3Hour
		sendEvent(app, "resetTimeOption", map[string]int{"option": TimeOption3Hour})

		// 타이머 값 초기화 이벤트 추가
		sendEvent(app, "resetTimer", nil)

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Settings reset")
	})

	// 종료 API
	http.HandleFunc("/api/exit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 애플리케이션 종료 예약
		go func() {
			// 응답 전송 후 짧은 지연
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}()

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Exiting")
	})
}

// 자바스크립트 콜백 함수 바인딩
func bindJavaScriptCallbacks(app *Application) {
	// 모드 변경 바인딩
	app.WebView.Bind("setMode", func(mode int) {
		app.ActiveMode = mode
	})

	// 시간 설정 변경 바인딩
	app.WebView.Bind("setTimeOption", func(option int) {
		app.TimeOption = option
	})

	// 자동 시작 설정 바인딩
	app.WebView.Bind("setAutoStartup", func(enabled bool) {
		app.AutoStartup = enabled
	})

	// 시작 버튼 클릭 바인딩
	app.WebView.Bind("startOperation", func() {
		startOperation(app)
	})

	// 중지 버튼 클릭 바인딩
	app.WebView.Bind("stopOperation", func() {
		stopOperation(app)
	})

	// 재설정 버튼 클릭 바인딩
	app.WebView.Bind("resetSettings", func() {
		resetSettings(app)
	})

	// 종료 버튼 클릭 바인딩
	app.WebView.Bind("exitApplication", func() {
		go func() {
			time.Sleep(500 * time.Millisecond)
			app.WebView.Terminate()
		}()
	})
}

// 웹뷰에 이벤트 전송
func sendEvent(app *Application, eventType string, payload interface{}) {
	if app.WebView == nil {
		return
	}

	event := UIEvent{
		Type:    eventType,
		Payload: payload,
	}

	// JSON으로 직렬화
	jsonData, err := json.Marshal(event)
	if err != nil {
		return
	}

	// 스크립트로 이벤트 전송
	script := fmt.Sprintf("window.dispatchAppEvent(%s);", string(jsonData))
	app.WebView.Dispatch(func() {
		if app.WebView != nil {
			app.WebView.Eval(script)
		}
	})
}

// 시작 버튼 클릭 처리 - 수정된 버전
func startOperation(app *Application) {
	if app.TimerManager == nil || app.TimerManager.IsRunning() {
		return
	}

	if app.ActiveMode == ModeNone {
		sendEvent(app, "operationStatus", map[string]bool{"running": false})
		return
	}

	// 실행 시간 설정 확인 - 10분 추가된 시간 계산
	var hours float64
	switch app.TimeOption {
	case TimeOption1Hour:
		hours = 1 + (10.0 / 60.0) // 1시간 10분
	case TimeOption2Hour:
		hours = 2 + (10.0 / 60.0) // 2시간 10분
	case TimeOption3Hour:
		hours = 3 + (10.0 / 60.0) // 3시간 10분
	case TimeOption4Hour:
		hours = 4 + (10.0 / 60.0) // 4시간 10분
	default:
		hours = 3 + (10.0 / 60.0) // 기본값: 3시간 10분
	}

	// 상태 업데이트
	app.RunningOperation = true
	sendEvent(app, "operationStatus", map[string]bool{"running": true})

	// 타이머 시작
	app.TimerManager.Start()

	// 자동 중지 타이머 설정
	setupAutoStop(app, hours)

	// 키보드 매니저 시작
	if app.KeyboardManager != nil {
		app.KeyboardManager.SetRunning(true)

		// 선택된 모드에 따라 자동화 시작
		switch app.ActiveMode {
		case ModeDaeyaEnter:
			go app.KeyboardManager.DaeyaEnter()
		case ModeDaeyaParty:
			go app.KeyboardManager.DaeyaParty()
		case ModeKanchenEnter:
			go app.KeyboardManager.KanchenEnter()
		case ModeKanchenParty:
			go app.KeyboardManager.KanchenParty()
		}
	}

	// 텔레그램 시작 알림 전송
	if app.Config.TelegramEnabled && app.Config.TelegramBot != nil {
		modeName := getModeName(app.ActiveMode)
		duration := time.Duration(hours * float64(time.Hour))
		go func() {
			err := app.Config.TelegramBot.SendStartNotification(modeName, duration)
			if err != nil {
				log.Printf("텔레그램 시작 알림 전송 실패: %v", err)
			}
		}()
	}
}

// 중지 버튼 클릭 처리
func stopOperation(app *Application) {
	if app.TimerManager == nil || !app.TimerManager.IsRunning() {
		return
	}

	// 상태 업데이트
	app.RunningOperation = false
	sendEvent(app, "operationStatus", map[string]bool{"running": false})

	// 타이머 중지
	app.TimerManager.Stop()

	// 자동 중지 타이머가 있으면 중지
	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		app.AutoStopTimer = nil
	}

	// 키보드 매니저 중지
	if app.KeyboardManager != nil {
		app.KeyboardManager.SetRunning(false)
	}
}

// 재설정 버튼 클릭 처리
func resetSettings(app *Application) {
	if app.TimerManager == nil || app.TimerManager.IsRunning() {
		return
	}

	// 타이머 재설정
	app.TimerManager.Reset()

	// 모드 초기화 - 대야 입장(기본값)으로 설정
	app.ActiveMode = ModeDaeyaEnter
	sendEvent(app, "resetMode", ModePayload{Mode: ModeDaeyaEnter})

	// 시간 설정 초기화 - 3시간 10분(기본값)으로 설정
	app.TimeOption = TimeOption3Hour
	sendEvent(app, "resetTimeOption", map[string]int{"option": TimeOption3Hour})

	// 타이머 값 초기화 이벤트 추가
	sendEvent(app, "resetTimer", nil)
}

// 시간이 지난 후 자동 중지 처리 - float64로 수정
func setupAutoStop(app *Application, hours float64) {
	// 이전 타이머가 있다면 중지
	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		app.AutoStopTimer = nil
	}

	if hours <= 0 {
		return
	}

	// 새 타이머 설정 - float64를 time.Duration으로 변환
	duration := time.Duration(hours * float64(time.Hour))
	app.AutoStopTimer = time.AfterFunc(duration, func() {
		if app.TimerManager != nil && app.TimerManager.IsRunning() {
			// 현재 모드 이름 가져오기
			modeName := getModeName(app.ActiveMode)

			// 상태 업데이트
			app.RunningOperation = false
			sendEvent(app, "operationStatus", map[string]bool{"running": false})

			// 타이머 중지
			app.TimerManager.Stop()

			// 키보드 매니저 중지
			if app.KeyboardManager != nil {
				app.KeyboardManager.SetRunning(false)
			}

			// 텔레그램 완료 알림 전송
			if app.Config.TelegramEnabled && app.Config.TelegramBot != nil {
				go func() {
					err := app.Config.TelegramBot.SendCompletionNotification(modeName, duration)
					if err != nil {
						log.Printf("텔레그램 완료 알림 전송 실패: %v", err)
					}
				}()
			}

			log.Printf("작업 완료: %s 모드, %v 실행", modeName, duration)
		}
	})
}

// 모드 이름 가져오기
func getModeName(mode int) string {
	switch mode {
	case ModeDaeyaEnter:
		return "대야 (입장)"
	case ModeDaeyaParty:
		return "대야 (파티)"
	case ModeKanchenEnter:
		return "칸첸 (입장)"
	case ModeKanchenParty:
		return "칸첸 (파티)"
	default:
		return "알 수 없음"
	}
}

// 폴더 존재 확인
func dirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// 로그 파일 읽기 함수
func readLogFile(appConfig *config.AppConfig) (string, error) {
	logFilePath := appConfig.GetLogFilePath()
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// 로그 파일 지우기 함수
func clearLogFile(appConfig *config.AppConfig) error {
	logFilePath := appConfig.GetLogFilePath()
	// 파일을 비우는 방식으로 지우기
	return os.WriteFile(logFilePath, []byte(""), 0666)
}

// 로그 텍스트를 라인 단위로 분할
func splitLogToLines(logContent string) []string {
	lines := strings.Split(logContent, "\n")
	// 빈 줄 제거
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	// 최신 로그가 맨 위에 오도록 역순 정렬
	reverseSlice(result)
	return result
}

// 슬라이스 역순 정렬 함수
func reverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
