package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"example.com/m/config"
	"example.com/m/routes"
	"example.com/m/services"
)

//go:embed webui
var webFiles embed.FS

func main() {
	// 설정 초기화
	appConfig := config.NewAppConfig()

	// 로그 설정
	setupLogging(appConfig)

	// 서비스 초기화
	serviceContainer := services.NewServiceContainer(appConfig)

	// 라우터 설정
	router := routes.NewRouter(serviceContainer, webFiles)

	// 서버 시작
	log.Printf("웹 서버를 포트 8080에서 시작합니다...")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Printf("서버 시작 오류: %v", err)
		os.Exit(1)
	}
}

func setupLogging(appConfig *config.AppConfig) {
	logFile := appConfig.GetLogFilePath()

	// 로그 디렉토리 생성
	logDir := filepath.Dir(logFile)
	if !dirExists(logDir) {
		err := os.MkdirAll(logDir, 0755)
		if err != nil {
			log.Printf("경고: 로그 디렉토리를 생성할 수 없습니다: %v", err)
		}
	}

	// 로그 파일 열기
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("경고: 로그 파일을 열 수 없습니다: %v", err)
		return
	}

	// 표준 로그 설정
	log.SetOutput(f)
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
