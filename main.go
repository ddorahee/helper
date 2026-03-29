package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"example.com/m/automation"
	"example.com/m/config"
	"example.com/m/keymapping"
	"example.com/m/telegram"
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
	WindowManager    *automation.WindowManager
	MouseAutomation  *automation.MouseAutomation
	RotationManager  *automation.RotationManager
	OCRManager       *automation.OCRManager
	ItemScanner      *automation.ItemScanner
	DaeyaBattle      *automation.DaeyaBattle
	CharacterStore    *config.CharacterStore
	KeyMappingStore   *config.KeyMappingStore
	KeyMappingMgr     *keymapping.KeyMappingManager
	TelegramStore     *config.TelegramStore
	TelegramBot       *telegram.TelegramBot
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

// levenshtein 두 rune 슬라이스 간의 편집 거리 계산
func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			dp[i][j] = dp[i-1][j] + 1 // 삭제
			if dp[i][j-1]+1 < dp[i][j] {
				dp[i][j] = dp[i][j-1] + 1 // 삽입
			}
			if dp[i-1][j-1]+cost < dp[i][j] {
				dp[i][j] = dp[i-1][j-1] + cost // 대체
			}
		}
	}
	return dp[la][lb]
}

func main() {
	// 로그 파일 설정
	setupLogging()

	// 시작 로그
	log.Println("도우미 애플리케이션 시작")

	// 애플리케이션 생성
	app := NewApplication()

	// 키보드 매니저 생성
	keyboardManager := automation.NewKeyboardManager()
	app.KeyboardManager = keyboardManager

	// 키 맵핑 저장소 초기화
	keyMappingStore := config.NewKeyMappingStore()
	if err := keyMappingStore.Load(); err != nil {
		log.Printf("키 맵핑 데이터 로드 오류: %v", err)
	}
	app.KeyMappingStore = keyMappingStore
	applyKeyMappings(app)

	// 키 매핑 매니저 초기화
	keyMappingMgr := keymapping.NewKeyMappingManager("config")
	app.KeyMappingMgr = keyMappingMgr

	// 텔레그램 초기화
	telegramStore := config.NewTelegramStore()
	if err := telegramStore.Load(); err != nil {
		log.Printf("텔레그램 설정 로드 오류: %v", err)
	}
	app.TelegramStore = telegramStore
	if telegramStore.IsConfigured() {
		app.TelegramBot = telegram.NewTelegramBot(telegramStore.Config.Token, telegramStore.Config.ChatID)
		log.Println("텔레그램 봇 초기화 완료")
	}

	// 자동 사냥 관련 초기화
	characterStore := config.NewCharacterStore()
	if err := characterStore.Load(); err != nil {
		log.Printf("캐릭터 데이터 로드 오류: %v", err)
	}
	app.CharacterStore = characterStore

	windowManager := automation.NewWindowManager()
	app.WindowManager = windowManager

	// OCR 매니저 초기화
	ocrManager := automation.NewOCRManager(windowManager)
	ocrCfg := characterStore.GetOCRConfig()
	ocrManager.SetConfig(automation.OCRConfig{
		NameRegionX:      ocrCfg.NameRegionX,
		NameRegionY:      ocrCfg.NameRegionY,
		NameRegionWidth:  ocrCfg.NameRegionWidth,
		NameRegionHeight: ocrCfg.NameRegionHeight,
	})
	app.OCRManager = ocrManager

	// 상주 PowerShell OCR 프로세스 시작
	if err := ocrManager.StartPersistentOCR(); err != nil {
		log.Printf("[OCR] 상주 PowerShell 시작 실패 (나중에 재시도): %v", err)
	}
	defer ocrManager.StopPersistentOCR()

	mouseAutomation := automation.NewMouseAutomation(windowManager)
	app.MouseAutomation = mouseAutomation

	itemScanner := automation.NewItemScanner(ocrManager, keyboardManager, mouseAutomation)
	itemScanner.SetLogFunc(func(msg string) {
		sendEvent(app, "rotationLog", map[string]string{"message": msg})
	})
	// 저장된 아이템 습득 설정 불러오기
	savedPickupCfg := characterStore.GetItemPickupConfig()
	if len(savedPickupCfg.Items) > 0 {
		loadedItems := make([]automation.TargetItem, len(savedPickupCfg.Items))
		for i, it := range savedPickupCfg.Items {
			loadedItems[i] = automation.TargetItem{Name: it.Name, Color: it.Color}
		}
		scanInterval := savedPickupCfg.ScanInterval
		if scanInterval < 1 {
			scanInterval = 1
		}
		itemScanner.SetConfig(automation.ItemScannerConfig{
			Enabled:      savedPickupCfg.Enabled,
			Items:        loadedItems,
			ScanInterval: scanInterval,
			TilePixelW:   savedPickupCfg.TilePixelW,
			TilePixelH:   savedPickupCfg.TilePixelH,
			OriginX:      savedPickupCfg.OriginX,
			OriginY:      savedPickupCfg.OriginY,
			TargetMap:    savedPickupCfg.TargetMap,
			WrongMap:     savedPickupCfg.WrongMap,
			SkillKeys:    savedPickupCfg.SkillKeys,
		})
		log.Printf("아이템 습득 설정 로드: %d개 아이템 (원점: %d,%d)", len(loadedItems), savedPickupCfg.OriginX, savedPickupCfg.OriginY)
	}
	app.ItemScanner = itemScanner

	daeyaBattle := automation.NewDaeyaBattle(ocrManager, keyboardManager, windowManager)
	daeyaBattle.SetLogFunc(func(msg string) {
		sendEvent(app, "rotationLog", map[string]string{"message": msg})
	})
	app.DaeyaBattle = daeyaBattle

	rotationManager := automation.NewRotationManager(windowManager, mouseAutomation)
	rotationManager.SetEventCallback(func(eventType string, payload interface{}) {
		sendEvent(app, eventType, payload)
	})
	app.RotationManager = rotationManager

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
	app.WebView.SetSize(app.WindowWidth, app.WindowHeight, webview.HintMin)
	app.WebView.SetSize(app.WindowWidth, app.WindowHeight, webview.HintMax)

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
		TimeOption:       TimeOption3Hour, // 기본값: 3시간
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
	// 시작 API
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

		// 자동 종료 시간 파라미터 가져오기 (옵션)
		autoStopStr := r.FormValue("auto_stop")
		var autoStopHours int = 0
		if autoStopStr != "" {
			fmt.Sscanf(autoStopStr, "%d", &autoStopHours)
		}

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
				// 대야 입장: OCR 기반 맵 감지 + 스킬/좌표 이동 자동화
				if windows, err := app.WindowManager.FindGameWindows(); err == nil && len(windows) > 0 {
					app.DaeyaBattle.Start(windows[0].HWND)
				} else {
					// 게임 창 미발견 시 기존 키 시퀀스 폴백
					km.DaeyaEnter()
				}
			case ModeDaeyaParty:
				km.DaeyaParty()
			case ModeKanchenEnter:
				km.KanchenEnter()
			case ModeKanchenParty:
				km.KanchenParty()
			}
		}()

		// 아이템 스캐너 시작 (칸첸 모드만)
		if internalMode == ModeKanchenEnter || internalMode == ModeKanchenParty {
			if windows, err := app.WindowManager.FindGameWindows(); err == nil && len(windows) > 0 {
				app.ItemScanner.Start(windows[0].HWND)
			}
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

		// 아이템 스캐너 중지
		app.ItemScanner.Stop()

		// 대야전투 자동화 중지
		app.DaeyaBattle.Stop()

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
		logContent, err := readLogFile()
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
		err := clearLogFile()
		if err != nil {
			http.Error(w, "Failed to clear log file", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"success": true}`)
	})

	// 설정 API
	http.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 설정 타입과 값 가져오기
		settingType := r.FormValue("type")
		settingValue := r.FormValue("value")

		if settingType == "" || settingValue == "" {
			http.Error(w, "Missing parameters", http.StatusBadRequest)
			return
		}

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
			// 시간 설정
			var hours int
			fmt.Sscanf(settingValue, "%d", &hours)
			switch hours {
			case 1:
				app.TimeOption = TimeOption1Hour
			case 2:
				app.TimeOption = TimeOption2Hour
			case 3:
				app.TimeOption = TimeOption3Hour
			case 4:
				app.TimeOption = TimeOption4Hour
			}
		case "auto_startup":
			// 자동 시작 설정
			var enabled int
			fmt.Sscanf(settingValue, "%d", &enabled)
			app.AutoStartup = (enabled == 1)
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Settings updated")
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

		// 시간 설정 초기화 - 3시간(기본값)으로 설정
		app.TimeOption = TimeOption3Hour
		sendEvent(app, "resetTimeOption", map[string]int{"option": TimeOption3Hour})

		// 타이머 값 초기화 이벤트 추가
		sendEvent(app, "resetTimer", nil)

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Settings reset")
	})

	// === 자동 사냥 API ===

	// 캐릭터 목록 조회 / 추가
	http.HandleFunc("/api/rotation/characters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			chars := app.CharacterStore.GetByOrder()
			json.NewEncoder(w).Encode(chars)

		case http.MethodPost:
			var profile config.CharacterProfile
			if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			if profile.ID == "" {
				profile.ID = fmt.Sprintf("char_%d", time.Now().UnixNano())
			}
			app.CharacterStore.Add(profile)
			if err := app.CharacterStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(profile)

		case http.MethodPut:
			var profile config.CharacterProfile
			if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			app.CharacterStore.Update(profile)
			if err := app.CharacterStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(profile)

		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "ID 필요", http.StatusBadRequest)
				return
			}
			app.CharacterStore.Remove(id)
			if err := app.CharacterStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, `{"success":true}`)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 캐릭터 순서 변경
	http.HandleFunc("/api/rotation/characters/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID        string `json:"id"`
			Direction int    `json:"direction"` // -1=위, +1=아래
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}
		app.CharacterStore.MoveOrder(req.ID, req.Direction)
		if err := app.CharacterStore.Save(); err != nil {
			http.Error(w, "저장 실패", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true}`)
	})

	// 캐릭터 활성화/비활성화
	http.HandleFunc("/api/rotation/characters/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}
		app.CharacterStore.SetEnabled(req.ID, req.Enabled)
		if err := app.CharacterStore.Save(); err != nil {
			http.Error(w, "저장 실패", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true}`)
	})

	// 게임 창 감지
	http.HandleFunc("/api/rotation/windows", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		windows, err := app.WindowManager.FindGameWindows()
		if err != nil {
			http.Error(w, fmt.Sprintf("창 감지 실패: %v", err), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(windows)
	})

	// 창 스크린샷 캡처
	http.HandleFunc("/api/rotation/screenshot", func(w http.ResponseWriter, r *http.Request) {
		hwndStr := r.URL.Query().Get("hwnd")
		if hwndStr == "" {
			http.Error(w, "hwnd 필요", http.StatusBadRequest)
			return
		}
		var hwnd uint64
		fmt.Sscanf(hwndStr, "%d", &hwnd)

		screenshot, err := app.WindowManager.CaptureWindow(hwnd)
		if err != nil {
			http.Error(w, fmt.Sprintf("스크린샷 실패: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"image": screenshot})
	})

	// 창 스크린샷 캡처 (원본 해상도 - OCR 영역 설정용)
	http.HandleFunc("/api/rotation/screenshot/full", func(w http.ResponseWriter, r *http.Request) {
		hwndStr := r.URL.Query().Get("hwnd")
		if hwndStr == "" {
			http.Error(w, "hwnd 필요", http.StatusBadRequest)
			return
		}
		var hwnd uint64
		fmt.Sscanf(hwndStr, "%d", &hwnd)

		img, _, err := app.WindowManager.CaptureWindowRaw(hwnd)
		if err != nil {
			http.Error(w, fmt.Sprintf("스크린샷 실패: %v", err), http.StatusInternalServerError)
			return
		}

		// 원본 크기 JPEG 인코딩
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
			http.Error(w, fmt.Sprintf("JPEG 인코딩 실패: %v", err), http.StatusInternalServerError)
			return
		}

		b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"image":  "data:image/jpeg;base64," + b64,
			"width":  img.Bounds().Dx(),
			"height": img.Bounds().Dy(),
		})
	})

	// OCR 한국어 지원 확인
	http.HandleFunc("/api/rotation/ocr/check", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		available, err := app.OCRManager.CheckKoreanOCRAvailable()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"error":     err.Error(),
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"available": available,
		})
	})

	// OCR 설정 조회/수정
	http.HandleFunc("/api/rotation/ocr/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			cfg := app.CharacterStore.GetOCRConfig()
			json.NewEncoder(w).Encode(cfg)

		case http.MethodPost:
			var cfg config.OCRRegionConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			app.CharacterStore.SetOCRConfig(cfg)
			if err := app.CharacterStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}
			app.OCRManager.SetConfig(automation.OCRConfig{
				NameRegionX:      cfg.NameRegionX,
				NameRegionY:      cfg.NameRegionY,
				NameRegionWidth:  cfg.NameRegionWidth,
				NameRegionHeight: cfg.NameRegionHeight,
			})
			json.NewEncoder(w).Encode(cfg)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// OCR 크롭 이미지 디버그 - 실제 크롭 영역 확인용
	http.HandleFunc("/api/rotation/ocr/debug-crop", func(w http.ResponseWriter, r *http.Request) {
		hwndStr := r.URL.Query().Get("hwnd")
		if hwndStr == "" {
			http.Error(w, "hwnd 필요", http.StatusBadRequest)
			return
		}
		var hwnd uint64
		fmt.Sscanf(hwndStr, "%d", &hwnd)

		img, err := app.OCRManager.CaptureNameRegion(hwnd)
		if err != nil {
			http.Error(w, fmt.Sprintf("크롭 실패: %v", err), http.StatusInternalServerError)
			return
		}

		// PNG로 인코딩하여 반환
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			http.Error(w, fmt.Sprintf("PNG 인코딩 실패: %v", err), http.StatusInternalServerError)
			return
		}

		b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"image":  "data:image/png;base64," + b64,
			"width":  img.Bounds().Dx(),
			"height": img.Bounds().Dy(),
			"config": app.OCRManager.GetConfig(),
		})
	})

	// 윈도우 감지 + OCR 자동 매칭
	http.HandleFunc("/api/rotation/detect-with-ocr", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// 1. 게임 창 감지
		windows, err := app.WindowManager.FindGameWindows()
		if err != nil {
			http.Error(w, fmt.Sprintf("창 감지 실패: %v", err), http.StatusInternalServerError)
			return
		}

		// 2. 등록된 캐릭터 목록
		chars := app.CharacterStore.GetByOrder()

		// 3. 각 창에 대해 OCR 실행 및 매칭
		type WindowOCRResult struct {
			HWND         uint64 `json:"hwnd"`
			Title        string `json:"title"`
			PID          uint32 `json:"pid"`
			DetectedName string `json:"detectedName"`
			MatchedID    string `json:"matchedId,omitempty"`
			MatchedName  string `json:"matchedName,omitempty"`
			Confidence   string `json:"confidence"`
			Error        string `json:"error,omitempty"`
		}

		var results []WindowOCRResult
		for _, win := range windows {
			result := WindowOCRResult{
				HWND:       win.HWND,
				Title:      win.Title,
				PID:        win.PID,
				Confidence: "none",
			}

			name, err := app.OCRManager.DetectCharacterName(win.HWND)
			if err != nil {
				log.Printf("[OCR] 실패 (hwnd=%d): %v", win.HWND, err)
				result.DetectedName = ""
				result.Error = err.Error()
			} else {
				log.Printf("[OCR] 감지 (hwnd=%d): '%s'", win.HWND, name)
				result.DetectedName = name
				// 등록된 캐릭터와 매칭 (빈 문자열이면 매칭 안 함)
				if name != "" {
					bestDist := 999
					nameRunes := []rune(name)

					for _, c := range chars {
						charRunes := []rune(c.Name)
						charLen := len(charRunes)

						// 1. 정확 일치
						if c.Name == name {
							result.MatchedID = c.ID
							result.MatchedName = c.Name
							result.Confidence = "exact"
							bestDist = 0
							break
						}

						// 2. 전체 문자열 편집 거리
						dist := levenshtein(nameRunes, charRunes)
						if dist <= 2 && dist < bestDist {
							bestDist = dist
							result.MatchedID = c.ID
							result.MatchedName = c.Name
							result.Confidence = "partial"
						}

						// 3. 부분 문자열 매칭: OCR 결과가 더 길 때
						//    OCR 결과 내에서 캐릭터 이름 길이만큼의 윈도우를 슬라이딩하며 최소 거리 탐색
						if len(nameRunes) > charLen {
							for start := 0; start <= len(nameRunes)-charLen; start++ {
								sub := nameRunes[start : start+charLen]
								d := levenshtein(sub, charRunes)
								if d <= 1 && d < bestDist {
									bestDist = d
									result.MatchedID = c.ID
									result.MatchedName = c.Name
									result.Confidence = "partial"
								}
							}
						}
					}
					if bestDist > 0 && bestDist <= 2 && result.MatchedID != "" {
						log.Printf("[OCR] 유사도 매칭: '%s' ≈ '%s' (거리=%d)", name, result.MatchedName, bestDist)
					}
				}
			}

			results = append(results, result)
		}

		// 소거법: 미매칭 창이 있고 미매칭 캐릭터가 있으면 자동 배정
		matchedIDs := make(map[string]bool)
		for _, res := range results {
			if res.MatchedID != "" {
				matchedIDs[res.MatchedID] = true
			}
		}
		var unmatchedChars []struct{ ID, Name string }
		for _, c := range chars {
			if !matchedIDs[c.ID] {
				unmatchedChars = append(unmatchedChars, struct{ ID, Name string }{c.ID, c.Name})
			}
		}
		var unmatchedIdxs []int
		for i, res := range results {
			if res.MatchedID == "" {
				unmatchedIdxs = append(unmatchedIdxs, i)
			}
		}
		// 미매칭 창 수와 미매칭 캐릭터 수가 같으면 순서대로 배정
		if len(unmatchedIdxs) > 0 && len(unmatchedIdxs) == len(unmatchedChars) {
			for j, idx := range unmatchedIdxs {
				results[idx].MatchedID = unmatchedChars[j].ID
				results[idx].MatchedName = unmatchedChars[j].Name
				results[idx].Confidence = "remaining"
				log.Printf("[OCR] 소거법 매칭: hwnd=%d → '%s'", results[idx].HWND, unmatchedChars[j].Name)
			}
		}

		json.NewEncoder(w).Encode(results)
	})

	// 아이템 자동 습득 설정 API
	http.HandleFunc("/api/item-pickup/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(app.ItemScanner.GetConfig())
		} else if r.Method == http.MethodPost {
			var cfg automation.ItemScannerConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "잘못된 요청", http.StatusBadRequest)
				return
			}
			log.Printf("[아이템습득] 설정 수신: enabled=%v, items=%d개, origin=(%d,%d)",
				cfg.Enabled, len(cfg.Items), cfg.OriginX, cfg.OriginY)
			app.ItemScanner.SetConfig(cfg)
			// 설정을 CharacterStore에도 영속화
			persistItems := make([]config.ItemPickupTargetItem, len(cfg.Items))
			for i, it := range cfg.Items {
				persistItems[i] = config.ItemPickupTargetItem{Name: it.Name, Color: it.Color}
			}
			app.CharacterStore.SetItemPickupConfig(config.ItemPickupConfig{
				Enabled:      cfg.Enabled,
				Items:        persistItems,
				ScanInterval: cfg.ScanInterval,
				TilePixelW:   cfg.TilePixelW,
				TilePixelH:   cfg.TilePixelH,
				OriginX:      cfg.OriginX,
				OriginY:      cfg.OriginY,
				TargetMap:    cfg.TargetMap,
				WrongMap:     cfg.WrongMap,
				SkillKeys:    cfg.SkillKeys,
			})
			if err := app.CharacterStore.Save(); err != nil {
				log.Printf("아이템 습득 설정 저장 실패: %v", err)
			}
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		}
	})

	// 아이템 스캔 테스트 API
	http.HandleFunc("/api/item-pickup/test-scan", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		windows, err := app.WindowManager.FindGameWindows()
		if err != nil || len(windows) == 0 {
			http.Error(w, "게임 창을 찾을 수 없습니다", http.StatusNotFound)
			return
		}

		hwnd := windows[0].HWND

		// 전체 화면 캡처
		rawImg, _, captErr := app.WindowManager.CaptureWindowRaw(hwnd)
		if captErr != nil {
			http.Error(w, "캡처 실패: "+captErr.Error(), http.StatusInternalServerError)
			return
		}

		imgW := rawImg.Bounds().Dx()
		imgH := rawImg.Bounds().Dy()

		words, msg, scanErr := app.ItemScanner.TestScan(hwnd)
		if scanErr != nil {
			http.Error(w, scanErr.Error(), http.StatusInternalServerError)
			return
		}

		// 전체 화면에 아이템 감지 위치 + 화면 중심 마커를 그린 디버그 이미지 생성
		var markedImageB64 string
		{
			// 원본 복사
			marked := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
			for y := 0; y < imgH; y++ {
				for x := 0; x < imgW; x++ {
					marked.Set(x, y, rawImg.At(x, y))
				}
			}

			// 화면 중심에 흰색 십자 (캐릭터 위치)
			cx, cy := imgW/2, imgH/2
			for d := -15; d <= 15; d++ {
				if cx+d >= 0 && cx+d < imgW {
					marked.Pix[(cy*imgW+(cx+d))*4+0] = 255
					marked.Pix[(cy*imgW+(cx+d))*4+1] = 255
					marked.Pix[(cy*imgW+(cx+d))*4+2] = 255
				}
				if cy+d >= 0 && cy+d < imgH {
					marked.Pix[((cy+d)*imgW+cx)*4+0] = 255
					marked.Pix[((cy+d)*imgW+cx)*4+1] = 255
					marked.Pix[((cy+d)*imgW+cx)*4+2] = 255
				}
			}

			// 감지된 아이템 위치에 색상별 십자 마커
			scanConfig := app.ItemScanner.GetConfig()
			for _, item := range scanConfig.Items {
				if item.Name == "" {
					continue
				}
				for _, wd := range words {
					if automation.FuzzyMatchItemPublic(wd.Text, item.Name) {
						ix := int(wd.X + wd.Width/2)
						iy := int(wd.Y + wd.Height/2)
						// 색상 결정: green→초록, yellow→노란, 기타→빨강
						var mr, mg, mb uint8 = 255, 0, 0
						if item.Color == "green" {
							mr, mg, mb = 0, 255, 0
						} else if item.Color == "yellow" {
							mr, mg, mb = 255, 255, 0
						}
						for d := -20; d <= 20; d++ {
							if ix+d >= 0 && ix+d < imgW {
								marked.Pix[((iy)*imgW+(ix+d))*4+0] = mr
								marked.Pix[((iy)*imgW+(ix+d))*4+1] = mg
								marked.Pix[((iy)*imgW+(ix+d))*4+2] = mb
							}
							if iy+d >= 0 && iy+d < imgH {
								marked.Pix[((iy+d)*imgW+ix)*4+0] = mr
								marked.Pix[((iy+d)*imgW+ix)*4+1] = mg
								marked.Pix[((iy+d)*imgW+ix)*4+2] = mb
							}
						}
						break
					}
				}
			}

			var buf bytes.Buffer
			if jpeg.Encode(&buf, marked, &jpeg.Options{Quality: 80}) == nil {
				markedImageB64 = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":     msg,
			"words":       words,
			"wordCount":   len(words),
			"markedImage": markedImageB64,
			"imgSize":     fmt.Sprintf("%dx%d", imgW, imgH),
		})
	})

	// 좌표 OCR 테스트 API (디버그 이미지 포함)
	http.HandleFunc("/api/item-pickup/test-coords", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		windows, err := app.WindowManager.FindGameWindows()
		if err != nil || len(windows) == 0 {
			http.Error(w, "게임 창을 찾을 수 없습니다", http.StatusNotFound)
			return
		}

		hwnd := windows[0].HWND
		rawImg, _, captErr := app.WindowManager.CaptureWindowRaw(hwnd)
		if captErr != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "캡처 실패: " + captErr.Error(),
			})
			return
		}

		// 디버그: 우하단 크롭 이미지 생성
		imgW := rawImg.Bounds().Dx()
		imgH := rawImg.Bounds().Dy()
		cropW := 200
		cropH := 40
		cropX := imgW - cropW - 5
		cropY := imgH - cropH - 5
		if cropX < 0 {
			cropX = 0
		}
		if cropY < 0 {
			cropY = 0
		}
		cropped := rawImg.SubImage(image.Rect(cropX, cropY, cropX+cropW, cropY+cropH))

		var cropB64 string
		var buf bytes.Buffer
		if png.Encode(&buf, cropped) == nil {
			cropB64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
		}

		// 디버그: 크롭 이미지를 testdata에 저장
		os.MkdirAll("testdata", 0755)
		if tf, terr := os.Create(fmt.Sprintf("testdata/crop_%d.png", time.Now().UnixMilli())); terr == nil {
			png.Encode(tf, cropped)
			tf.Close()
		}

		coords, debugTexts, err := app.OCRManager.ReadCoordinatesFromImage(rawImg)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":    false,
				"error":      err.Error(),
				"cropImage":  cropB64,
				"imgWidth":   imgW,
				"imgHeight":  imgH,
				"ocrResults": debugTexts,
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"x":          coords.X,
			"y":          coords.Y,
			"cropImage":  cropB64,
			"imgWidth":   imgW,
			"imgHeight":  imgH,
			"ocrResults": debugTexts,
		})
	})

	// 창-캐릭터 매핑
	http.HandleFunc("/api/rotation/assign", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var assignments []struct {
			CharacterID string `json:"characterId"`
			WindowHWND  uint64 `json:"windowHwnd"`
		}

		if err := json.NewDecoder(r.Body).Decode(&assignments); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}

		app.CharacterStore.ClearAllAssignments()
		for _, a := range assignments {
			app.CharacterStore.SetWindowHWND(a.CharacterID, a.WindowHWND)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true}`)
	})

	// 자동 사냥 시작
	http.HandleFunc("/api/rotation/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 기존 키보드 자동화와 상호배제
		if app.RunningOperation {
			http.Error(w, "키보드 자동화가 실행 중입니다. 먼저 중지해주세요.", http.StatusConflict)
			return
		}

		if app.RotationManager.IsRunning() {
			http.Error(w, "이미 자동 사냥이 실행 중입니다.", http.StatusConflict)
			return
		}

		// 캐릭터 목록 가져오기
		chars := app.CharacterStore.GetByOrder()
		if len(chars) == 0 {
			http.Error(w, "등록된 캐릭터가 없습니다.", http.StatusBadRequest)
			return
		}

		// RotationCharacter로 변환 (활성화 + 윈도우 할당된 것만)
		var rotChars []automation.RotationCharacter
		for _, c := range chars {
			if !c.Assigned || !c.Enabled {
				continue
			}
			rotChars = append(rotChars, automation.RotationCharacter{
				ID:            c.ID,
				Name:          c.Name,
				HuntingArea:   c.HuntingArea.Name,
				DropdownIndex: c.HuntingArea.DropdownIndex,
				DurationMins:  c.DurationMins,
				Order:         c.Order,
				WindowHWND:    c.WindowHWND,
			})
		}

		if len(rotChars) == 0 {
			http.Error(w, "활성화되고 윈도우가 할당된 캐릭터가 없습니다. 감지 후 시작해주세요.", http.StatusBadRequest)
			return
		}

		// 좌표 가져오기
		coordsCfg := app.CharacterStore.GetCoordinates()
		coords := automation.GameUICoords{
			SwordButtonX:       coordsCfg.SwordButtonX,
			SwordButtonY:       coordsCfg.SwordButtonY,
			DropdownArrowX:     coordsCfg.DropdownArrowX,
			DropdownArrowY:     coordsCfg.DropdownArrowY,
			DropdownItemHeight: coordsCfg.DropdownItemHeight,
			DropdownFirstItemY: coordsCfg.DropdownFirstItemY,
			StartButtonX:       coordsCfg.StartButtonX,
			StartButtonY:       coordsCfg.StartButtonY,
			ConfirmButtonX:     coordsCfg.ConfirmButtonX,
			ConfirmButtonY:     coordsCfg.ConfirmButtonY,
			AlertConfirmX:      coordsCfg.AlertConfirmX,
			AlertConfirmY:      coordsCfg.AlertConfirmY,
		}

		if err := app.RotationManager.Start(rotChars, coords); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true}`)
	})

	// 자동 사냥 중지
	http.HandleFunc("/api/rotation/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		app.RotationManager.Stop()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true}`)
	})

	// 자동 사냥 상태 조회
	http.HandleFunc("/api/rotation/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := app.RotationManager.GetStatus()
		json.NewEncoder(w).Encode(status)
	})

	// UI 좌표 설정 조회/수정
	http.HandleFunc("/api/rotation/coordinates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			coords := app.CharacterStore.GetCoordinates()
			json.NewEncoder(w).Encode(coords)

		case http.MethodPost:
			var coords config.GameUICoordinates
			if err := json.NewDecoder(r.Body).Decode(&coords); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			app.CharacterStore.SetCoordinates(coords)
			if err := app.CharacterStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(coords)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 키 맵핑 API
	http.HandleFunc("/api/keymapping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			data := app.KeyMappingStore.GetAll()
			json.NewEncoder(w).Encode(data)

		case http.MethodPost:
			var data config.KeyMappingData
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			app.KeyMappingStore.SetAll(data)
			if err := app.KeyMappingStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}
			// KeyboardManager에 즉시 적용
			applyKeyMappings(app)
			json.NewEncoder(w).Encode(map[string]bool{"success": true})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// === 키 매핑 시스템 API ===

	// 매핑 CRUD
	http.HandleFunc("/api/keymapping/mappings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			mappings := app.KeyMappingMgr.GetAllMappings()
			json.NewEncoder(w).Encode(mappings)

		case http.MethodPost:
			var req struct {
				Name        string                `json:"name"`
				StartKey    string                `json:"start_key"`
				KeySequence string                `json:"key_sequence"`
				Keys        []keymapping.MappedKey `json:"keys"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			// keys가 직접 전달된 경우 사용, 아니면 key_sequence 파싱
			keys := req.Keys
			if len(keys) == 0 && req.KeySequence != "" {
				var err error
				keys, err = app.KeyMappingMgr.ParseKeySequence(req.KeySequence)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			if err := app.KeyMappingMgr.AddMapping(req.Name, req.StartKey, keys); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"success": true})

		case http.MethodPut:
			var req struct {
				ID          string                `json:"id"`
				Name        string                `json:"name"`
				StartKey    string                `json:"start_key"`
				KeySequence string                `json:"key_sequence"`
				Keys        []keymapping.MappedKey `json:"keys"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}
			keys := req.Keys
			if len(keys) == 0 && req.KeySequence != "" {
				var err error
				keys, err = app.KeyMappingMgr.ParseKeySequence(req.KeySequence)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
			if err := app.KeyMappingMgr.UpdateMappingByID(req.ID, req.Name, req.StartKey, keys); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(map[string]bool{"success": true})

		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "ID 필요", http.StatusBadRequest)
				return
			}
			if _, err := app.KeyMappingMgr.RemoveMappingByID(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			fmt.Fprint(w, `{"success":true}`)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 매핑 토글
	http.HandleFunc("/api/keymapping/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}
		if err := app.KeyMappingMgr.ToggleMappingByID(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true}`)
	})

	// 시스템 시작/중지
	http.HandleFunc("/api/keymapping/control", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Action string `json:"action"` // "start" or "stop"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch req.Action {
		case "start":
			if err := app.KeyMappingMgr.Start(); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			fmt.Fprint(w, `{"success":true,"running":true}`)
		case "stop":
			app.KeyMappingMgr.Stop()
			fmt.Fprint(w, `{"success":true,"running":false}`)
		default:
			http.Error(w, "잘못된 액션", http.StatusBadRequest)
		}
	})

	// 상태 조회
	http.HandleFunc("/api/keymapping/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := app.KeyMappingMgr.GetMappingStats()
		json.NewEncoder(w).Encode(stats)
	})

	// 사용 가능한 키 목록
	http.HandleFunc("/api/keymapping/keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		keys := app.KeyMappingMgr.GetAvailableKeys()
		json.NewEncoder(w).Encode(keys)
	})

	// === 텔레그램 API ===

	// 텔레그램 설정 조회/저장
	http.HandleFunc("/api/telegram/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled":    app.TelegramStore.Config.Enabled,
				"configured": app.TelegramStore.Config.Token != "" && app.TelegramStore.Config.ChatID != "",
				"chat_id":    app.TelegramStore.Config.ChatID,
			})

		case http.MethodPost:
			var req struct {
				Token   string `json:"token"`
				ChatID  string `json:"chat_id"`
				Enabled bool   `json:"enabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "잘못된 데이터", http.StatusBadRequest)
				return
			}

			app.TelegramStore.SetConfig(req.Token, req.ChatID, req.Enabled)
			if err := app.TelegramStore.Save(); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}

			// 봇 인스턴스 업데이트
			if req.Token != "" && req.ChatID != "" && req.Enabled {
				app.TelegramBot = telegram.NewTelegramBot(req.Token, req.ChatID)
			} else {
				app.TelegramBot = nil
			}

			json.NewEncoder(w).Encode(map[string]bool{"success": true})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 텔레그램 연결 테스트
	http.HandleFunc("/api/telegram/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if app.TelegramBot == nil {
			http.Error(w, "텔레그램이 설정되지 않았습니다", http.StatusBadRequest)
			return
		}

		if err := app.TelegramBot.TestConnection(); err != nil {
			http.Error(w, fmt.Sprintf("테스트 실패: %v", err), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})

	// 텔레그램 활성화/비활성화 토글
	http.HandleFunc("/api/telegram/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}

		app.TelegramStore.Config.Enabled = req.Enabled
		if err := app.TelegramStore.Save(); err != nil {
			http.Error(w, "저장 실패", http.StatusInternalServerError)
			return
		}

		// 봇 인스턴스 업데이트
		if req.Enabled && app.TelegramStore.Config.Token != "" && app.TelegramStore.Config.ChatID != "" {
			app.TelegramBot = telegram.NewTelegramBot(app.TelegramStore.Config.Token, app.TelegramStore.Config.ChatID)
		} else if !req.Enabled {
			app.TelegramBot = nil
		}

		json.NewEncoder(w).Encode(map[string]bool{"success": true, "enabled": req.Enabled})
	})

	// 마우스 위치 캡처 API (좌표 찾기 모드)
	http.HandleFunc("/api/rotation/capture", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			WindowHWND uint64 `json:"windowHwnd"`
			DelaySec   int    `json:"delaySec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "잘못된 데이터", http.StatusBadRequest)
			return
		}
		if req.DelaySec <= 0 {
			req.DelaySec = 3
		}

		relX, relY, err := app.MouseAutomation.CaptureClickPosition(req.WindowHWND, req.DelaySec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"x": relX, "y": relY})
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

// getModeName 모드 이름 반환
func getModeName(mode int) string {
	switch mode {
	case ModeDaeyaEnter:
		return "대야 입장"
	case ModeDaeyaParty:
		return "대야 파티"
	case ModeKanchenEnter:
		return "간첸 입장"
	case ModeKanchenParty:
		return "간첸 파티"
	default:
		return "알 수 없음"
	}
}

// 시작 버튼 클릭 처리
func startOperation(app *Application) {
	if app.TimerManager == nil || app.TimerManager.IsRunning() {
		return
	}

	if app.ActiveMode == ModeNone {
		sendEvent(app, "operationStatus", map[string]bool{"running": false})
		return
	}

	// 자동 사냥과 상호배제
	if app.RotationManager != nil && app.RotationManager.IsRunning() {
		sendEvent(app, "log", LogPayload{Message: "자동 사냥이 실행 중입니다. 먼저 중지해주세요."})
		return
	}

	// 실행 시간 설정 확인
	var hours int
	switch app.TimeOption {
	case TimeOption1Hour:
		hours = 1
	case TimeOption2Hour:
		hours = 2
	case TimeOption3Hour:
		hours = 3
	case TimeOption4Hour:
		hours = 4
	default:
		hours = 3
	}

	// 상태 업데이트
	app.RunningOperation = true
	sendEvent(app, "operationStatus", map[string]bool{"running": true})

	// 타이머 시작
	app.TimerManager.Start()

	// 자동 중지 타이머 설정
	setupAutoStop(app, hours)

	// 텔레그램 시작 알림
	if app.TelegramBot != nil {
		go func() {
			modeName := getModeName(app.ActiveMode)
			duration := time.Duration(hours) * time.Hour
			if err := app.TelegramBot.SendStartNotification(modeName, duration); err != nil {
				log.Printf("텔레그램 시작 알림 실패: %v", err)
			}
		}()
	}

	// 키보드 매니저 시작
	if app.KeyboardManager != nil {
		app.KeyboardManager.SetRunning(true)

		// 선택된 모드에 따라 자동화 시작
		switch app.ActiveMode {
		case ModeDaeyaEnter:
			if windows, err := app.WindowManager.FindGameWindows(); err == nil && len(windows) > 0 {
				go app.DaeyaBattle.Start(windows[0].HWND)
			} else {
				go app.KeyboardManager.DaeyaEnter()
			}
		case ModeDaeyaParty:
			go app.KeyboardManager.DaeyaParty()
		case ModeKanchenEnter:
			go app.KeyboardManager.KanchenEnter()
		case ModeKanchenParty:
			go app.KeyboardManager.KanchenParty()
		}

		// 아이템 스캐너 시작 (칸첸 모드만)
		if app.ActiveMode == ModeKanchenEnter || app.ActiveMode == ModeKanchenParty {
			scanCfg := app.ItemScanner.GetConfig()
			log.Printf("[시작] 아이템 스캐너 config: enabled=%v, items=%d개", scanCfg.Enabled, len(scanCfg.Items))
			if windows, err := app.WindowManager.FindGameWindows(); err == nil && len(windows) > 0 {
				log.Printf("[시작] 게임 창 발견: hwnd=%d, 아이템 스캐너 Start() 호출", windows[0].HWND)
				app.ItemScanner.Start(windows[0].HWND)
			} else {
				log.Printf("[시작] 게임 창 미발견 — 아이템 스캐너 시작 불가")
			}
		}
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

	// 아이템 스캐너 중지
	if app.ItemScanner != nil {
		app.ItemScanner.Stop()
	}

	// 대야전투 자동화 중지
	if app.DaeyaBattle != nil {
		app.DaeyaBattle.Stop()
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

	// 시간 설정 초기화 - 3시간(기본값)으로 설정
	app.TimeOption = TimeOption3Hour
	sendEvent(app, "resetTimeOption", map[string]int{"option": TimeOption3Hour})

	// 타이머 값 초기화 이벤트 추가
	sendEvent(app, "resetTimer", nil)
}

// 시간이 지난 후 자동 중지 처리
func setupAutoStop(app *Application, hours int) {
	// 이전 타이머가 있다면 중지
	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		app.AutoStopTimer = nil
	}

	if hours <= 0 {
		return
	}

	// 새 타이머 설정
	duration := time.Duration(hours) * time.Hour
	app.AutoStopTimer = time.AfterFunc(duration, func() {
		if app.TimerManager != nil && app.TimerManager.IsRunning() {
			// 상태 업데이트
			app.RunningOperation = false
			sendEvent(app, "operationStatus", map[string]bool{"running": false})

			// 타이머 중지
			app.TimerManager.Stop()

			// 키보드 매니저 중지
			if app.KeyboardManager != nil {
				app.KeyboardManager.SetRunning(false)
			}

			// 텔레그램 완료 알림
			if app.TelegramBot != nil {
				go func() {
					modeName := getModeName(app.ActiveMode)
					d := time.Duration(hours) * time.Hour
					if err := app.TelegramBot.SendCompletionNotification(modeName, d); err != nil {
						log.Printf("텔레그램 완료 알림 실패: %v", err)
					}
				}()
			}
		}
	})
}

// applyKeyMappings 저장된 키 맵핑을 KeyboardManager에 적용
func applyKeyMappings(app *Application) {
	data := app.KeyMappingStore.GetAll()
	for mode, seq := range data.Sequences {
		var delays []time.Duration
		for _, d := range seq.Delays {
			delays = append(delays, time.Duration(d*1000)*time.Millisecond)
		}
		app.KeyboardManager.SetSequence(mode, seq.Keys, delays)
	}
}

// 로그 파일 설정
func setupLogging() {
	// 로그 폴더 생성
	logDir := "logs"
	if !dirExists(logDir) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			fmt.Println("경고: 로그 디렉토리를 생성할 수 없습니다:", err)
		}
	}

	// 로그 파일 이름 설정
	logFile := filepath.Join(logDir, "app.log")

	// 로그 파일 열기
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("경고: 로그 파일을 열 수 없습니다:", err)
		return
	}

	// 표준 로그 설정: 파일 + 터미널(stdout) 동시 출력
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
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
func readLogFile() (string, error) {
	logFilePath := filepath.Join("logs", "app.log")
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// 로그 파일 지우기 함수
func clearLogFile() error {
	logFilePath := filepath.Join("logs", "app.log")
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
