package dto

type TelegramConfigRequest struct {
	Token  string `form:"token" binding:"required"`
	ChatID string `form:"chat_id" binding:"required"`
}

type TelegramStatusResponse struct {
	Enabled bool `json:"enabled"`
}
