package controllers

import (
	"embed"
	"net/http"
	"strings"
)

// StaticController는 정적 파일 서빙을 처리합니다
type StaticController struct {
	webFiles embed.FS
}

// NewStaticController는 새로운 정적 파일 컨트롤러를 생성합니다
func NewStaticController(webFiles embed.FS) *StaticController {
	return &StaticController{
		webFiles: webFiles,
	}
}

// ServeStatic은 정적 파일을 서빙합니다
func (c *StaticController) ServeStatic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/ui/web/index.html"
	} else {
		path = "/ui/web" + path
	}

	// 정적 파일 제공
	content, err := c.webFiles.ReadFile(strings.TrimPrefix(path, "/"))
	if err != nil {
		// 파일이 없으면 index.html로 리다이렉트 (SPA 라우팅 지원)
		if !strings.Contains(path, ".") {
			content, err = c.webFiles.ReadFile("ui/web/index.html")
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	// 콘텐츠 유형 설정
	contentType := c.getContentType(path)

	// CORS 헤더 추가 (개발 환경에서 필요)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

// getContentType은 파일 확장자에 따라 콘텐츠 타입을 반환합니다
func (c *StaticController) getContentType(path string) string {
	if strings.HasSuffix(path, ".css") {
		return "text/css"
	} else if strings.HasSuffix(path, ".js") {
		return "application/javascript"
	} else if strings.HasSuffix(path, ".svg") {
		return "image/svg+xml"
	} else if strings.HasSuffix(path, ".png") {
		return "image/png"
	} else if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
		return "image/jpeg"
	} else if strings.HasSuffix(path, ".ico") {
		return "image/x-icon"
	} else if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	return "text/html"
}
