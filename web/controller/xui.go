package controller

import (
	"encoding/json"
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// XUIController is the main controller for the X-UI panel, managing sub-controllers.
type XUIController struct {
	BaseController

	settingController     *SettingController
	xraySettingController *XraySettingController
}

// NewXUIController creates a new XUIController and initializes its routes.
func NewXUIController(g *gin.RouterGroup) *XUIController {
	a := &XUIController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the main panel routes and initializes sub-controllers.
func (a *XUIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/panel")
	g.Use(a.checkLogin)

	g.GET("/", a.index)
	g.GET("/user", a.user)
	g.GET("/inbounds", a.inbounds)
	g.GET("/settings", a.settings)
	g.GET("/xray", a.xraySettings)
	g.GET("/api/user/info", a.userInfo)

	a.settingController = NewSettingController(g)
	a.xraySettingController = NewXraySettingController(g)
}

// index renders the main panel index page. Non-admin users are redirected to the user dashboard.
func (a *XUIController) index(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user.Role != "admin" {
		c.Redirect(http.StatusTemporaryRedirect, "user")
		return
	}
	html(c, "index.html", "pages.index.title", nil)
}

// user renders the user dashboard page.
func (a *XUIController) user(c *gin.Context) {
	html(c, "user.html", "pages.user.title", nil)
}

// inbounds renders the inbounds management page.
func (a *XUIController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "pages.inbounds.title", nil)
}

// settings renders the settings management page.
func (a *XUIController) settings(c *gin.Context) {
	html(c, "settings.html", "pages.settings.title", nil)
}

// xraySettings renders the Xray settings page.
func (a *XUIController) xraySettings(c *gin.Context) {
	html(c, "xray.html", "pages.xray.title", nil)
}

// userInfo returns per-inbound traffic info for the logged-in user.
func (a *XUIController) userInfo(c *gin.Context) {
	user := session.GetLoginUser(c)

	inboundService := service.InboundService{}
	inbounds, err := inboundService.GetAllInbounds()
	if err != nil {
		jsonObj(c, nil, err)
		return
	}

	type UserInboundInfo struct {
		Remark     string `json:"remark"`
		Protocol   string `json:"protocol"`
		Up         int64  `json:"up"`
		Down       int64  `json:"down"`
		Total      int64  `json:"total"`
		ExpiryTime int64  `json:"expiryTime"`
		Enable     bool   `json:"enable"`
	}

	var userInbounds []UserInboundInfo

	for _, inbound := range inbounds {
		var settings map[string]any
		err := json.Unmarshal([]byte(inbound.Settings), &settings)
		if err != nil {
			continue
		}

		clientsInterface, ok := settings["clients"]
		if !ok {
			continue
		}

		clientsSlice, ok := clientsInterface.([]any)
		if !ok {
			continue
		}

		for _, ci := range clientsSlice {
			clientMap, ok := ci.(map[string]any)
			if !ok {
				continue
			}
			clientEmail, _ := clientMap["email"].(string)
			if clientEmail == user.Username {
				info := UserInboundInfo{
					Remark:   inbound.Remark,
					Protocol: string(inbound.Protocol),
					Enable:   true,
				}
				// Find matching traffic stats
				for _, stat := range inbound.ClientStats {
					if stat.Email == user.Username {
						info.Up = stat.Up
						info.Down = stat.Down
						info.Total = stat.Total
						info.ExpiryTime = stat.ExpiryTime
						info.Enable = stat.Enable
						break
					}
				}
				userInbounds = append(userInbounds, info)
				break
			}
		}
	}

	jsonObj(c, userInbounds, nil)
}
