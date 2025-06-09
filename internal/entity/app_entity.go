package entity

import "time"

type AppStatus struct {
	IsRunning    bool      `json:"running"`
	Mode         string    `json:"mode"`
	StartTime    time.Time `json:"start_time"`
	ElapsedTime  int64     `json:"elapsed_time"`
	AutoStopTime int64     `json:"auto_stop_time"`
}

type Settings struct {
	DarkMode        bool `json:"dark_mode"`
	SoundEnabled    bool `json:"sound_enabled"`
	AutoStartup     bool `json:"auto_startup"`
	TelegramEnabled bool `json:"telegram_enabled"`
}
