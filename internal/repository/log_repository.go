package repository

import (
	"bufio"
	"os"
	"strings"
	"time"
)

type LogRepository interface {
	GetLogs() ([]string, error)
	AddLog(message string) error
	ClearLogs() error
}

type logRepository struct {
	logFilePath string
}

func NewLogRepository(logFilePath string) LogRepository {
	return &logRepository{
		logFilePath: logFilePath,
	}
}

func (r *logRepository) GetLogs() ([]string, error) {
	// 로그 파일이 존재하지 않으면 빈 슬라이스 반환
	if _, err := os.Stat(r.logFilePath); os.IsNotExist(err) {
		return []string{}, nil
	}

	file, err := os.Open(r.logFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var logs []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			logs = append(logs, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// 최신 로그가 먼저 오도록 역순 정렬
	r.reverseSlice(logs)

	// 최대 1000개까지만 반환
	if len(logs) > 1000 {
		logs = logs[:1000]
	}

	return logs, nil
}

func (r *logRepository) AddLog(message string) error {
	// 로그 디렉토리 생성
	if err := r.ensureLogDirectory(); err != nil {
		return err
	}

	file, err := os.OpenFile(r.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// 타임스탬프와 함께 로그 작성
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := timestamp + " - " + message + "\n"

	_, err = file.WriteString(logLine)
	return err
}

func (r *logRepository) ClearLogs() error {
	return os.WriteFile(r.logFilePath, []byte(""), 0644)
}

func (r *logRepository) ensureLogDirectory() error {
	logDir := strings.TrimSuffix(r.logFilePath, "/app.log")
	return os.MkdirAll(logDir, 0755)
}

func (r *logRepository) reverseSlice(slice []string) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
