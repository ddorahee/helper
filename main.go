package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"example.com/m/automation"
	"example.com/m/config"
	"example.com/m/keymapping"
	"example.com/m/server"
	"example.com/m/telegram"
	"example.com/m/utils"
	webview "github.com/webview/webview_go"
)

//go:embed ui/web
var webFiles embed.FS

// Application 구조체
type Application struct {
	WebView           webview.WebView
	Config            *config.AppConfig
	TimerManager      *utils.TimerManager
	KeyboardManager   *automation.KeyboardManager
	KeyMappingManager *keymapping.KeyMappingManager
	TelegramBot       *telegram.TelegramBot
	Server            *server.Server
	AppContext        context.Context
	CancelFunc        context.CancelFunc
	AutoStopTimer     *time.Timer
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

func (app *Application) GetKeyMappingManager() interface{} {
	return app.KeyMappingManager
}

func (app *Application) GetTelegramBot() interface{} {
	if app.Config.TelegramEnabled && app.Config.TelegramBot != nil {
		return app.Config.TelegramBot
	}
	return nil
}

// SetupAutoStop 개선된 자동 중지 설정 (안전한 버전)
func (app *Application) SetupAutoStop(mode string, hours float64) {
	log.Printf("자동 중지 설정 시작: %s 모드, %.2f시간", mode, hours)

	// 기존 타이머 정리
	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		app.AutoStopTimer = nil
		log.Printf("기존 자동 중지 타이머 정리됨")
	}

	if hours <= 0 {
		log.Printf("자동 중지 시간이 0 이하, 설정하지 않음")
		return
	}

	duration := time.Duration(hours * float64(time.Hour))
	log.Printf("자동 중지 시간 설정: %v", duration)

	// 서버 기반 타이머에 시간 설정
	if app.TimerManager != nil {
		app.TimerManager.SetDuration(duration)
		log.Printf("타이머 매니저에 시간 설정 완료")
	}

	// 타이머 완료 콜백 설정
	if app.TimerManager != nil {
		app.TimerManager.SetTimeCompleteCallback(func() {
			log.Printf("타이머 완료 콜백 실행됨")
			modeName := utils.GetModeName(mode)

			// 작업 중지
			if app.KeyboardManager != nil {
				app.KeyboardManager.SetRunning(false)
				log.Printf("키보드 매니저 중지됨")
			}

			// 텔레그램 완료 알림
			if app.Config.TelegramEnabled && app.TelegramBot != nil {
				go func() {
					err := app.TelegramBot.SendCompletionNotification(modeName, duration)
					if err != nil {
						log.Printf("텔레그램 완료 알림 전송 실패: %v", err)
					} else {
						log.Printf("텔레그램 완료 알림 전송 성공")
					}
				}()
			}

			log.Printf("작업 완료: %s 모드, %v 실행", modeName, duration)

			// WebView에 완료 알림 전송 (안전한 버전)
			if app.WebView != nil {
				app.WebView.Dispatch(func() {
					jsCode := fmt.Sprintf(`
						try {
							window.dispatchEvent(new CustomEvent('timerComplete', {
								detail: {
									mode: '%s',
									duration: %d
								}
							}));
							console.log('타이머 완료 이벤트 전송됨');
						} catch (e) {
							console.error('타이머 완료 이벤트 전송 실패:', e);
						}
					`, modeName, int(duration.Seconds()))
					app.WebView.Eval(jsCode)
				})
			}
		})
	}

	// 시간 업데이트 콜백 설정 (WebView에 실시간 시간 전송)
	if app.TimerManager != nil {
		app.TimerManager.SetTimeUpdateCallback(func(remaining time.Duration) {
			if app.WebView != nil {
				remainingSeconds := int(remaining.Seconds())
				app.WebView.Dispatch(func() {
					jsCode := fmt.Sprintf(`
						try {
							window.dispatchEvent(new CustomEvent('timerUpdate', {
								detail: {
									remaining: %d
								}
							}));
						} catch (e) {
							console.error('타이머 업데이트 이벤트 전송 실패:', e);
						}
					`, remainingSeconds)
					app.WebView.Eval(jsCode)
				})
			}
		})
	}

	log.Printf("서버 기반 자동 중지 설정 완료: %v", duration)
}

func (app *Application) RunAutomation(mode string) {
	log.Printf("자동화 시작: %s", mode)

	// 키보드 매니저 초기화 확인
	if app.KeyboardManager == nil {
		log.Printf("키보드 매니저가 nil임")
		return
	}

	switch mode {
	case "daeya-entrance":
		log.Printf("대야 입장 모드 실행")
		app.KeyboardManager.DaeyaEnter()
	case "daeya-party":
		log.Printf("대야 파티 모드 실행")
		app.KeyboardManager.DaeyaParty()
	case "kanchen-entrance":
		log.Printf("칸첸 입장 모드 실행")
		app.KeyboardManager.KanchenEnter()
	case "kanchen-party":
		log.Printf("칸첸 파티 모드 실행")
		app.KeyboardManager.KanchenParty()
	default:
		log.Printf("알 수 없는 모드: %s", mode)
	}
}

func main() {
	// 애플리케이션 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("애플리케이션 초기화 시작")

	// 애플리케이션 초기화
	app := &Application{
		Config:            config.NewAppConfig(),
		TimerManager:      utils.NewTimerManager(),
		KeyboardManager:   automation.NewKeyboardManager(),
		KeyMappingManager: keymapping.NewKeyMappingManager(config.NewAppConfig().GetDataDir()),
		AppContext:        ctx,
		CancelFunc:        cancel,
	}

	// 각 컴포넌트가 제대로 초기화되었는지 확인
	if app.Config == nil {
		log.Fatal("Config 초기화 실패")
	}
	if app.TimerManager == nil {
		log.Fatal("TimerManager 초기화 실패")
	}
	if app.KeyboardManager == nil {
		log.Fatal("KeyboardManager 초기화 실패")
	}

	log.Println("모든 컴포넌트 초기화 완료")

	// 텔레그램 봇 초기화
	if app.Config.TelegramEnabled && app.Config.TelegramBot != nil {
		app.TelegramBot = app.Config.TelegramBot
		log.Println("텔레그램 봇 초기화 완료")
	}

	// 로깅 설정
	setupLogging(app.Config)
	log.Println("애플리케이션 시작 (개선된 타이머 시스템)")

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

	app.WebView.SetTitle("도우미 - 서버 기반 타이머")
	app.WebView.SetSize(1024, 768, webview.HintNone)

	// JavaScript API 바인딩 (안전한 버전)
	app.bindJavaScriptAPI()

	// 페이지 로드
	url := app.Server.GetURL()
	log.Printf("WebView 페이지 로드: %s", url)
	app.WebView.Navigate(url)

	// WebView 실행 (블로킹)
	log.Println("WebView 실행 시작")
	app.WebView.Run()
}

// bindJavaScriptAPI는 JavaScript API를 바인딩합니다 (안전한 버전)
func (app *Application) bindJavaScriptAPI() {
	log.Println("JavaScript API 바인딩 시작")

	// 기본 API
	app.WebView.Bind("exitApp", func() {
		log.Println("exitApp API 호출됨")
		app.CancelFunc()
		app.WebView.Terminate()
	})

	app.WebView.Bind("logMessage", func(message string) {
		log.Printf("WebView 로그: %s", message)
	})

	// 키 맵핑 관련 JavaScript API
	app.WebView.Bind("startKeyMapping", func() bool {
		log.Println("startKeyMapping API 호출됨")
		if app.KeyMappingManager == nil {
			log.Printf("KeyMappingManager가 nil임")
			return false
		}
		err := app.KeyMappingManager.Start()
		if err != nil {
			log.Printf("키 맵핑 시작 실패: %v", err)
			return false
		}
		return true
	})

	app.WebView.Bind("stopKeyMapping", func() {
		log.Println("stopKeyMapping API 호출됨")
		if app.KeyMappingManager != nil {
			app.KeyMappingManager.Stop()
		}
	})

	app.WebView.Bind("getKeyMappingStatus", func() map[string]interface{} {
		log.Println("getKeyMappingStatus API 호출됨")
		if app.KeyMappingManager != nil {
			return app.KeyMappingManager.GetMappingStats()
		}
		return map[string]interface{}{"running": false}
	})

	// 서버 기반 타이머 API 추가 (안전한 버전)
	app.WebView.Bind("getServerTime", func() map[string]interface{} {
		if app.TimerManager == nil {
			log.Printf("TimerManager가 nil임")
			return map[string]interface{}{
				"running":          false,
				"paused":           false,
				"remainingSeconds": 0,
				"elapsedSeconds":   0,
				"totalSeconds":     0,
				"progress":         0,
				"remainingString":  "00:00:00",
				"elapsedString":    "00:00:00",
			}
		}

		return map[string]interface{}{
			"running":          app.TimerManager.IsRunning(),
			"paused":           app.TimerManager.IsPaused(),
			"remainingSeconds": app.TimerManager.GetRemainingTimeSeconds(),
			"elapsedSeconds":   int(app.TimerManager.GetElapsedTime().Seconds()),
			"totalSeconds":     int(app.TimerManager.TotalDuration.Seconds()),
			"progress":         app.TimerManager.GetProgress(),
			"remainingString":  app.TimerManager.GetRemainingTimeString(),
			"elapsedString":    app.TimerManager.GetElapsedTimeString(),
		}
	})

	app.WebView.Bind("startServerTimer", func(durationSeconds int) bool {
		log.Printf("startServerTimer API 호출됨: %d초", durationSeconds)

		if app.TimerManager == nil {
			log.Printf("TimerManager가 nil임")
			return false
		}

		if durationSeconds <= 0 {
			log.Printf("잘못된 지속시간: %d초", durationSeconds)
			return false
		}

		duration := time.Duration(durationSeconds) * time.Second
		app.TimerManager.SetDuration(duration)
		app.TimerManager.Start()
		log.Printf("서버 타이머 시작 성공: %v", duration)
		return true
	})

	app.WebView.Bind("stopServerTimer", func() bool {
		log.Println("stopServerTimer API 호출됨")

		if app.TimerManager == nil {
			log.Printf("TimerManager가 nil임")
			return false
		}

		app.TimerManager.Stop()
		log.Printf("서버 타이머 일시정지 성공")
		return true
	})

	app.WebView.Bind("resetServerTimer", func() bool {
		log.Println("resetServerTimer API 호출됨")

		if app.TimerManager == nil {
			log.Printf("TimerManager가 nil임")
			return false
		}

		app.TimerManager.Reset()
		log.Printf("서버 타이머 리셋 성공")
		return true
	})

	log.Println("JavaScript API 바인딩 완료")
}

// shutdown은 애플리케이션을 정리합니다
func (app *Application) shutdown() {
	log.Println("애플리케이션 종료 중...")

	if app.Server != nil {
		app.Server.Shutdown()
		log.Println("서버 종료됨")
	}

	if app.TimerManager != nil && app.TimerManager.IsRunning() {
		app.TimerManager.Stop()
		log.Println("타이머 매니저 종료됨")
	}

	if app.KeyboardManager != nil && app.KeyboardManager.IsRunning() {
		app.KeyboardManager.SetRunning(false)
		log.Println("키보드 매니저 종료됨")
	}

	// 키 맵핑 시스템 정리
	if app.KeyMappingManager != nil && app.KeyMappingManager.IsRunning() {
		app.KeyMappingManager.Stop()
		log.Println("키 맵핑 매니저 종료됨")
	}

	if app.AutoStopTimer != nil {
		app.AutoStopTimer.Stop()
		log.Println("자동 중지 타이머 종료됨")
	}

	log.Println("모든 컴포넌트 정리 완료")
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

	// 터미널과 파일 모두에 출력
	multiWriter := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}
