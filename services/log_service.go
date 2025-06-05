package services

import (
	"log"
	"os"
	"strings"
)

// LogService는 로그 관련 비즈니스 로직을 처리합니다
type LogService struct {
	container *ServiceContainer
}

// NewLogService는 새로운 로그 서비스를 생성합니다
func NewLogService(container *ServiceContainer) *LogService {
	return &LogService{
		container: container,
	}
}

// AddLog는 로그 메시지를 추가합니다
func (s *LogService) AddLog(message string) {
	log.Println(message)
}

// GetLogs는 로그 목록을 반환합니다
func (s *LogService) GetLogs() ([]string, error) {
	logContent, err := s.readLogFile()
	if err != nil {
		return []string{}, err
	}

	logs := s.splitLogToLines(logContent)

	// 마지막 100개 항목만 반환
	maxEntries := 100
	if len(logs) > maxEntries {
		logs = logs[len(logs)-maxEntries:]
	}

	return logs, nil
}

// ClearLogs는 로그를 지웁니다
func (s *LogService) ClearLogs() error {
	logFilePath := s.container.Config.GetLogFilePath()
	return os.WriteFile(logFilePath, []byte(""), 0666)
}

// readLogFile은 로그 파일을 읽습니다
func (s *LogService) readLogFile() (string, error) {
	logFilePath := s.container.Config.GetLogFilePath()
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// splitLogToLines는 로그 텍스트를 라인 단위로 분할합니다
func (s *LogService) splitLogToLines(logContent string) []string {
	lines := strings.Split(logContent, "\n")
	var result []string

	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}

	// 최신 로그가 맨 위에 오도록 역순 정렬
	s.reverseSlice(result)
	return result
}

// reverseSlice는 슬라이스를 역순으로 정렬합니다
func (s *LogService) reverseSlice(slice []string) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
