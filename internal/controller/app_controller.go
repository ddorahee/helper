package controller

import (
	"net/http"

	"example.com/m/internal/dto"
	"example.com/m/internal/service"
	"example.com/m/internal/utils"
	"github.com/gin-gonic/gin"
)

type AppController interface {
	Start(c *gin.Context)
	Stop(c *gin.Context)
	Reset(c *gin.Context)
	Exit(c *gin.Context)
	Status(c *gin.Context)
	Settings(c *gin.Context)
	LoadSettings(c *gin.Context)
}

type appController struct {
	appService service.AppService
}

func NewAppController(appService service.AppService) AppController {
	return &appController{
		appService: appService,
	}
}

func (ac *appController) Start(c *gin.Context) {
	var req dto.StartOperationRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	err := ac.appService.StartOperation(req.Mode, req.AutoStop, req.Resume)
	if err != nil {
		utils.ErrorResponse(c, http.StatusConflict, err.Error())
		return
	}

	utils.SuccessResponse(c, "Started", nil)
}

func (ac *appController) Stop(c *gin.Context) {
	err := ac.appService.StopOperation()
	if err != nil {
		utils.ErrorResponse(c, http.StatusConflict, err.Error())
		return
	}

	utils.SuccessResponse(c, "Stopped", nil)
}

func (ac *appController) Reset(c *gin.Context) {
	err := ac.appService.ResetSettings()
	if err != nil {
		utils.ErrorResponse(c, http.StatusConflict, err.Error())
		return
	}

	utils.SuccessResponse(c, "Reset", nil)
}

func (ac *appController) Exit(c *gin.Context) {
	ac.appService.ExitApplication()
	utils.SuccessResponse(c, "Exiting", nil)
}

func (ac *appController) Status(c *gin.Context) {
	status := ac.appService.GetStatus()
	utils.SuccessResponse(c, "Status retrieved", status)
}

func (ac *appController) Settings(c *gin.Context) {
	var req dto.SettingsRequest
	if err := c.ShouldBind(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	err := ac.appService.SaveSetting(req.Type, req.Value)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(c, "Settings updated", nil)
}

func (ac *appController) LoadSettings(c *gin.Context) {
	settings := ac.appService.GetSettings()
	utils.SuccessResponse(c, "Settings loaded", settings)
}
