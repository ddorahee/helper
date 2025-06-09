package web

import (
	"embed"
	"io/fs"
	"net/http"
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

	// 정적 파일 핸들러
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 경로는 404 처리
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}

		// 루트 경로는 index.html로 리다이렉트
		if path == "" || path == "/" {
			path = "/index.html"
		}

		// 파일 확장자가 없으면 index.html 제공 (SPA 라우팅)
		if !strings.Contains(filepath.Base(path), ".") {
			path = "/index.html"
		}

		// 앞의 "/" 제거
		path = strings.TrimPrefix(path, "/")

		// 파일 읽기
		fileContent, err := fs.ReadFile(webFS, path)
		if err != nil {
			// 파일이 없으면 index.html 제공
			fileContent, err = fs.ReadFile(webFS, "index.html")
			if err != nil {
				c.String(http.StatusNotFound, "File not found")
				return
			}
			path = "index.html"
		}

		// Content-Type 설정
		contentType := getContentType(path)
		c.Header("Content-Type", contentType)

		// CORS 헤더 추가
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		// 캐시 헤더 설정 (정적 파일인 경우)
		if isStaticFile(path) {
			c.Header("Cache-Control", "public, max-age=31536000") // 1년
		}

		// 파일 내용 반환
		c.Data(http.StatusOK, contentType, fileContent)
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

func isStaticFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	staticExts := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".webp", ".mp4", ".webm", ".mp3", ".wav"}

	for _, staticExt := range staticExts {
		if ext == staticExt {
			return true
		}
	}
	return false
}
