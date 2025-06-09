package web

import (
	"embed"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func SetupStaticRoutes(router *gin.Engine, webFiles embed.FS) {
	// ui/web 하위의 파일들을 서빙하기 위한 설정
	webFS, err := fs.Sub(webFiles, "ui/web")
	if err != nil {
		panic("Failed to create web filesystem: " + err.Error())
	}

	// 디버깅을 위한 파일 목록 출력
	entries, _ := fs.ReadDir(webFS, ".")
	log.Println("=== Available static files ===")
	for _, entry := range entries {
		log.Printf("- %s", entry.Name())
		if entry.IsDir() {
			subEntries, _ := fs.ReadDir(webFS, entry.Name())
			for _, sub := range subEntries {
				log.Printf("  - %s/%s", entry.Name(), sub.Name())
			}
		}
	}
	log.Println("===============================")

	// NoRoute 핸들러 사용 (와일드카드 경로 대신)
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		log.Printf("NoRoute handling: %s", path)

		// API 경로는 404 처리
		if strings.HasPrefix(path, "/api/") {
			c.JSON(404, gin.H{"error": "API endpoint not found"})
			return
		}

		// 경로 정리
		if path == "" || path == "/" {
			path = "/index.html"
		}

		// 앞의 슬래시 제거
		cleanPath := strings.TrimPrefix(path, "/")

		log.Printf("Requesting: %s -> %s", path, cleanPath)

		// 파일 읽기 시도
		fileContent, err := fs.ReadFile(webFS, cleanPath)
		if err != nil {
			log.Printf("File not found: %s, serving index.html for SPA", cleanPath)
			// 파일이 없으면 index.html 제공 (SPA 라우팅)
			fileContent, err = fs.ReadFile(webFS, "index.html")
			if err != nil {
				log.Printf("index.html not found: %v", err)
				c.String(404, "File not found")
				return
			}
			cleanPath = "index.html"
		}

		// Content-Type 설정
		contentType := getContentType(cleanPath)
		c.Header("Content-Type", contentType)

		// CORS 헤더 추가
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		// 정적 파일에 대한 캐시 설정
		if strings.HasPrefix(cleanPath, "assets/") {
			c.Header("Cache-Control", "public, max-age=31536000")
		}

		log.Printf("Serving: %s (%s)", cleanPath, contentType)
		c.Data(200, contentType, fileContent)
	})
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
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}
