package server

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"example.com/m/config"
)

type Server struct {
	Config     *config.AppConfig
	HTTPServer *http.Server
	Port       int
	WebFiles   embed.FS
}

func NewServer(config *config.AppConfig, webFiles embed.FS) *Server {
	port := findAvailablePort()
	if port == 0 {
		log.Fatal("사용 가능한 포트를 찾을 수 없습니다")
	}

	return &Server{
		Config:   config,
		Port:     port,
		WebFiles: webFiles,
	}
}

func (s *Server) Start(ready chan<- bool) {
	// 서버 준비 알림
	go func() {
		time.Sleep(100 * time.Millisecond)
		ready <- true
	}()

	// 서버 시작
	if err := s.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("서버 오류: %v", err)
	}
}

func (s *Server) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// 정적 파일 핸들러
	mux.HandleFunc("/", s.handleStaticFiles)

	return mux
}

func (s *Server) InitializeServer(mux *http.ServeMux) {
	// 서버 설정 (localhost만 바인딩)
	s.HTTPServer = &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%d", s.Port),
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 16, // 64KB
	}
}

func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.HTTPServer.Shutdown(ctx); err != nil {
		log.Printf("서버 종료 오류: %v", err)
	}
}

func (s *Server) GetPort() int {
	return s.Port
}

func (s *Server) GetURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.Port)
}

// findAvailablePort는 사용 가능한 로컬 포트를 찾습니다
func findAvailablePort() int {
	for port := 8000; port <= 9000; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port
		}
	}
	return 0
}
