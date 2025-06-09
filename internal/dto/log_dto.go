package dto

type LogsResponse struct {
	Logs []string `json:"logs"`
}

type AddLogRequest struct {
	Message string `json:"message" binding:"required"`
}