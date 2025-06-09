package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// handleStaticFiles는 정적 파일을 서빙합니다
func (s *Server) handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	// 보안: localhost만 허용
	if !isLocalhost(r.Host, s.Port) {
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
	content, err := s.WebFiles.ReadFile(filePath)
	if err != nil {
		// SPA 라우팅을 위해 index.html 제공
		if path != "/index.html" {
			content, err = s.WebFiles.ReadFile("ui/web/index.html")
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

// isLocalhost는 localhost 요청인지 확인합니다
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

// isAllowedFile은 허용된 파일 확장자인지 확인합니다
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

// getContentType는 파일 확장자에 따른 Content-Type을 반환합니다
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
