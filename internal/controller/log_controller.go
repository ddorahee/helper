package controller

import (
	"net/http"

	"example.com/m/internal/dto"
	"example.com/m/internal/service"
	"example.com/m/internal/utils"
	"github.com/gin-gonic/gin"
)

type LogController interface {
	GetLogs(c *gin.Context)
	ClearLogs(c *gin.Context)
	AddLog(c *gin.Context)
}

type logController struct {
	logService service.LogService
}

func NewLogController(logService service.LogService) LogController {
	return &logController{
		logService: logService,
	}
}

func (lc *logController) GetLogs(c *gin.Context) {
	logs, err := lc.logService.GetLogs()
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to get logs: "+err.Error())
		return
	}

	response := dto.LogsResponse{
		Logs: logs,
	}

	utils.SuccessResponse(c, "Logs retrieved successfully", response)
}

func (lc *logController) ClearLogs(c *gin.Context) {
	err := lc.logService.ClearLogs()
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to clear logs: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Logs cleared successfully", nil)
}

func (lc *logController) AddLog(c *gin.Context) {
	var req dto.AddLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	err := lc.logService.AddLog(req.Message)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to add log: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Log added successfully", nil)
}
