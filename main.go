package main

import (
	"embed"
	"log"
	"os"

	"example.com/m/internal/app"
)

//go:embed ui/web/*
var webFiles embed.FS

func main() {
	// 개발 모드에서 ui/web 폴더가 없는 경우 체크
	if _, err := webFiles.ReadDir("ui/web"); err != nil {
		log.Println("Warning: ui/web 폴더를 찾을 수 없습니다.")
		log.Println("다음 단계를 따라주세요:")
		log.Println("1. cd webui")
		log.Println("2. npm install")
		log.Println("3. npm run build")
		log.Println("4. go run main.go")
		os.Exit(1)
	}

	application := app.NewApp(webFiles)
	if err := application.Run(); err != nil {
		log.Fatal("Failed to start application:", err)
	}
}
