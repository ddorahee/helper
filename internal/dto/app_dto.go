package dto

type StartOperationRequest struct {
	Mode     string  `form:"mode" binding:"required"`
	AutoStop float64 `form:"auto_stop"`
	Resume   bool    `form:"resume"`
}

type SettingsRequest struct {
	Type  string `form:"type" binding:"required"`
	Value string `form:"value" binding:"required"`
}

type StatusResponse struct {
	Running bool `json:"running"`
}

type SettingsResponse struct {
	DarkMode        bool `json:"dark_mode"`
	SoundEnabled    bool `json:"sound_enabled"`
	AutoStartup     bool `json:"auto_startup"`
	TelegramEnabled bool `json:"telegram_enabled"`
}
