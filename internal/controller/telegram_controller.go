package controller

import (
	"net/http"

	"example.com/m/internal/dto"
	"example.com/m/internal/service"
	"example.com/m/internal/utils"
	"github.com/gin-gonic/gin"
)

type TelegramController interface {
	Config(c *gin.Context)
	Test(c *gin.Context)
	GetConfig(c *gin.Context)
}

type telegramController struct {
	telegramService service.TelegramService
}

func NewTelegramController(telegramService service.TelegramService) TelegramController {
	return &telegramController{
		telegramService: telegramService,
	}
}

func (tc *telegramController) Config(c *gin.Context) {
	var req dto.TelegramConfigRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	err := tc.telegramService.SaveConfig(req.Token, req.ChatID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Telegram config saved successfully", nil)
}

func (tc *telegramController) Test(c *gin.Context) {
	err := tc.telegramService.TestConnection()
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Test failed: "+err.Error())
		return
	}

	utils.SuccessResponse(c, "Test message sent successfully", nil)
}

func (tc *telegramController) GetConfig(c *gin.Context) {
	status := tc.telegramService.GetStatus()
	utils.SuccessResponse(c, "Config retrieved successfully", status)
}
