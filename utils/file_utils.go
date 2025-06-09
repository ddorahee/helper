package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfigInterface 정의 (순환 import 방지)
type ConfigInterface interface {
	GetLogFilePath() string
}

// ReadLogFile은 로그 파일을 읽어서 내용을 반환합니다
func ReadLogFile(appConfig interface{}) (string, error) {
	config, ok := appConfig.(ConfigInterface)
	if !ok {
		return "", fmt.Errorf("config interface error")
	}

	logFilePath := config.GetLogFilePath()

	// 파일 존재 여부 확인
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		// 로그 디렉토리 생성
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return "", fmt.Errorf("로그 디렉토리 생성 실패: %v", err)
		}

		// 빈 로그 파일 생성
		if err := os.WriteFile(logFilePath, []byte(""), 0666); err != nil {
			return "", fmt.Errorf("로그 파일 생성 실패: %v", err)
		}

		return "", nil
	}

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		return "", fmt.Errorf("로그 파일 읽기 실패: %v", err)
	}

	return string(content), nil
}

// ClearLogFile은 로그 파일을 비웁니다
func ClearLogFile(appConfig interface{}) error {
	config, ok := appConfig.(ConfigInterface)
	if !ok {
		return fmt.Errorf("config interface error")
	}

	logFilePath := config.GetLogFilePath()
	return os.WriteFile(logFilePath, []byte(""), 0666)
}

// SplitLogToLines는 로그 내용을 라인별로 분할합니다
func SplitLogToLines(logContent string) []string {
	lines := strings.Split(logContent, "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	ReverseSlice(result)
	return result
}

// ReverseSlice는 문자열 슬라이스를 뒤집습니다
func ReverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// DirExists는 디렉토리 존재 여부를 확인합니다
func DirExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}
