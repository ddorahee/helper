package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"
	"time"

	"example.com/m/automation"
	"example.com/m/config"
	"example.com/m/server"
	"example.com/m/telegram"
	"example.com/m/utils"
	webview "github.com/webview/webview_go"
)

//go:embed ui/web
var webFiles embed.FS

// Application 구조체
type Application struct {
	WebView         webview.WebView
	Config          *config.AppConfig
	TimerManager    *utils.TimerManager
	KeyboardManager *automation.KeyboardManager
	TelegramBot     *telegram.TelegramBot
	Server          *server.Server
	AppContext      context.Context
	CancelFunc      context.CancelFunc
	AutoStopTimer   *time.Timer
}

// 인터페이스 구현 메서드들
func (app *Application) GetConfig() interface{} {
	return app.Config
}

func (app *Application) GetTimerManager() interface{} {
	return app.TimerManager
}

func (app *Application) GetKeyboardManager() interface{} {
	return app.KeyboardManager
}

func (app *Application) GetTelegramBot() interface{} {
	return app.TelegramBot
}

func (app *Application) SetupAutoStop(mode string, hours float64) {
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
			modeName := utils.GetModeName(mode)

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

func (app *Application) RunAutomation(mode string) {
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

	// 텔레그램 봇 초기화
	if app.Config.TelegramEnabled && app.Config.TelegramBot != nil {
		app.TelegramBot = app.Config.TelegramBot
	}

	// 로깅 설정
	setupLogging(app.Config)
	log.Println("애플리케이션 시작")

	// 서버 초기화
	app.Server = server.NewServer(app.Config, webFiles)

	// 라우트 설정
	mux := app.Server.SetupRoutes()

	// 핸들러 설정
	app.Server.SetupHandlers(app, mux)

	// HTTP 서버 초기화
	app.Server.InitializeServer(mux)

	// 서버 시작
	serverReady := make(chan bool)
	go app.Server.Start(serverReady)

	// 서버 준비 대기
	select {
	case <-serverReady:
		log.Printf("내장 서버가 포트 %d에서 시작되었습니다", app.Server.GetPort())
	case <-time.After(5 * time.Second):
		log.Fatal("서버 시작 시간 초과")
	}

	// WebView 초기화 및 실행
	app.initWebView()

	// 정리 작업
	app.shutdown()
	log.Println("애플리케이션 종료")
}

// initWebView는 WebView를 초기화하고 실행합니다
func (app *Application) initWebView() {
	debug := app.Config.DevelopmentMode
	app.WebView = webview.New(debug)
	defer app.WebView.Destroy()

	app.WebView.SetTitle("도우미")
	app.WebView.SetSize(1024, 768, webview.HintNone)

	// JavaScript API 바인딩
	app.bindJavaScriptAPI()

	// 페이지 로드
	url := app.Server.GetURL()
	app.WebView.Navigate(url)

	// WebView 실행 (블로킹)
	app.WebView.Run()
}

// bindJavaScriptAPI는 JavaScript API를 바인딩합니다
func (app *Application) bindJavaScriptAPI() {
	app.WebView.Bind("exitApp", func() {
		app.CancelFunc()
		app.WebView.Terminate()
	})

	app.WebView.Bind("logMessage", func(message string) {
		log.Printf("WebView: %s", message)
	})
}

// shutdown은 애플리케이션을 정리합니다
func (app *Application) shutdown() {
	log.Println("애플리케이션 종료 중...")

	if app.Server != nil {
		app.Server.Shutdown()
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

// setupLogging은 로깅을 설정합니다
func setupLogging(appConfig *config.AppConfig) {
	logFile := appConfig.GetLogFilePath()
	logDir := filepath.Dir(logFile)

	if !utils.DirExists(logDir) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			log.Printf("경고: 로그 디렉토리를 생성할 수 없습니다: %v\n", err)
		}
	}

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("경고: 로그 파일을 열 수 없습니다: %v\n", err)
		return
	}

	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}
