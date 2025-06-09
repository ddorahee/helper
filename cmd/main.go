package main

import (
	"embed"
	"log"

	"example.com/m/internal/app"
)

//go:embed ui/web/*
var webFiles embed.FS

func main() {
	application := app.NewApp(webFiles)
	if err := application.Run(); err != nil {
		log.Fatal("Failed to start application:", err)
	}
}
