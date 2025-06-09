package entity

import "time"

type LogEntry struct {
	ID        int       `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
}
