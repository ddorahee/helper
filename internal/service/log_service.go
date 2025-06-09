package service

import (
	"log"

	"example.com/m/internal/repository"
)

type LogService interface {
	GetLogs() ([]string, error)
	AddLog(message string) error
	ClearLogs() error
}

type logService struct {
	logRepo repository.LogRepository
}

func NewLogService(logRepo repository.LogRepository) LogService {
	return &logService{
		logRepo: logRepo,
	}
}

func (s *logService) GetLogs() ([]string, error) {
	logs, err := s.logRepo.GetLogs()
	if err != nil {
		log.Printf("Failed to get logs: %v", err)
		return nil, err
	}

	return logs, nil
}

func (s *logService) AddLog(message string) error {
	err := s.logRepo.AddLog(message)
	if err != nil {
		log.Printf("Failed to add log: %v", err)
		return err
	}

	// 표준 출력에도 로그 출력
	log.Println(message)
	return nil
}

func (s *logService) ClearLogs() error {
	err := s.logRepo.ClearLogs()
	if err != nil {
		log.Printf("Failed to clear logs: %v", err)
		return err
	}

	log.Println("Logs cleared")
	return nil
}
