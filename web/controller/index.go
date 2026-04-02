package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/middleware"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// LoginForm represents the login request structure.
type LoginForm struct {
	Username      string `json:"username" form:"username"`
	Password      string `json:"password" form:"password"`
	TwoFactorCode string `json:"twoFactorCode" form:"twoFactorCode"`
}

// RegisterForm represents the registration request structure.
type RegisterForm struct {
	Username       string `json:"username" form:"username"`
	Password       string `json:"password" form:"password"`
	TurnstileToken string `json:"turnstileToken" form:"turnstileToken"`
}

// IndexController handles the main index and login-related routes.
type IndexController struct {
	BaseController

	settingService service.SettingService
	userService    service.UserService
	inboundService service.InboundService
	tgbot          service.Tgbot
}

// NewIndexController creates a new IndexController and initializes its routes.
func NewIndexController(g *gin.RouterGroup) *IndexController {
	a := &IndexController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for index, login, logout, and two-factor authentication.
func (a *IndexController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.index)
	g.GET("/logout", a.logout)

	g.POST("/login", a.login)
	g.POST("/register", middleware.RateLimitMiddleware(5, time.Minute), a.register)
	g.POST("/getTwoFactorEnable", a.getTwoFactorEnable)
	g.POST("/getTurnstileSiteKey", a.getTurnstileSiteKey)
}

// index handles the root route, redirecting logged-in users to the panel or showing the login page.
func (a *IndexController) index(c *gin.Context) {
	if session.IsLogin(c) {
		c.Redirect(http.StatusTemporaryRedirect, "panel/")
		return
	}
	html(c, "login.html", "pages.login.title", nil)
}

// login handles user authentication and session creation.
func (a *IndexController) login(c *gin.Context) {
	var form LoginForm

	if err := c.ShouldBind(&form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return
	}
	if form.Username == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return
	}

	user, checkErr := a.userService.CheckUser(form.Username, form.Password, form.TwoFactorCode)
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	safeUser := template.HTMLEscapeString(form.Username)
	safePass := template.HTMLEscapeString(form.Password)

	if user == nil {
		logger.Warningf("wrong username: \"%s\", password: \"%s\", IP: \"%s\"", safeUser, safePass, getRemoteIp(c))

		notifyPass := safePass

		if checkErr != nil && checkErr.Error() == "invalid 2fa code" {
			translatedError := a.tgbot.I18nBot("tgbot.messages.2faFailed")
			notifyPass = fmt.Sprintf("*** (%s)", translatedError)
		}

		a.tgbot.UserLoginNotify(safeUser, notifyPass, getRemoteIp(c), timeStr, 0)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.wrongUsernameOrPassword"))
		return
	}

	logger.Infof("%s logged in successfully, Ip Address: %s\n", safeUser, getRemoteIp(c))
	a.tgbot.UserLoginNotify(safeUser, ``, getRemoteIp(c), timeStr, 1)

	sessionMaxAge, err := a.settingService.GetSessionMaxAge()
	if err != nil {
		logger.Warning("Unable to get session's max age from DB")
	}

	session.SetMaxAge(c, sessionMaxAge*60)
	session.SetLoginUser(c, user)
	if err := sessions.Default(c).Save(); err != nil {
		logger.Warning("Unable to save session: ", err)
		return
	}

	logger.Infof("%s logged in successfully", safeUser)
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successLogin"), nil)
}

// register handles new user registration.
func (a *IndexController) register(c *gin.Context) {
	var form RegisterForm

	if err := c.ShouldBind(&form); err != nil {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidFormData"))
		return
	}

	// Trim whitespace
	form.Username = strings.TrimSpace(form.Username)
	form.Password = strings.TrimSpace(form.Password)

	if form.Username == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyUsername"))
		return
	}
	if form.Password == "" {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.emptyPassword"))
		return
	}
	if len(form.Username) < 3 || len(form.Username) > 64 {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidUsername"))
		return
	}
	if len(form.Password) < 8 || len(form.Password) > 128 {
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.invalidPassword"))
		return
	}

	// Verify Turnstile token if site key is configured
	turnstileSecretKey, err := a.settingService.GetTurnstileSecretKey()
	if err == nil && turnstileSecretKey != "" {
		if form.TurnstileToken == "" {
			pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.turnstileRequired"))
			return
		}
		if !service.VerifyTurnstile(turnstileSecretKey, form.TurnstileToken, getRemoteIp(c)) {
			pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.turnstileRequired"))
			return
		}
	}

	err = a.userService.RegisterUser(form.Username, form.Password, &a.inboundService)
	if err != nil {
		if errors.Is(err, service.ErrUsernameAlreadyExists) {
			pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.userExists"))
			return
		}
		logger.Warningf("register failed for user \"%s\": %s", template.HTMLEscapeString(form.Username), err)
		pureJsonMsg(c, http.StatusOK, false, I18nWeb(c, "pages.login.toasts.errorRegister"))
		return
	}

	logger.Infof("new user registered: %s", template.HTMLEscapeString(form.Username))
	jsonMsg(c, I18nWeb(c, "pages.login.toasts.successRegister"), nil)
}

// logout handles user logout by clearing the session and redirecting to the login page.
func (a *IndexController) logout(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user != nil {
		logger.Infof("%s logged out successfully", user.Username)
	}
	session.ClearSession(c)
	if err := sessions.Default(c).Save(); err != nil {
		logger.Warning("Unable to save session after clearing:", err)
	}
	c.Redirect(http.StatusTemporaryRedirect, c.GetString("base_path"))
}

// getTwoFactorEnable retrieves the current status of two-factor authentication.
func (a *IndexController) getTwoFactorEnable(c *gin.Context) {
	status, err := a.settingService.GetTwoFactorEnable()
	if err == nil {
		jsonObj(c, status, nil)
	}
}

// getTurnstileSiteKey returns the Cloudflare Turnstile site key for the registration form.
func (a *IndexController) getTurnstileSiteKey(c *gin.Context) {
	siteKey, err := a.settingService.GetTurnstileSiteKey()
	if err == nil {
		jsonObj(c, siteKey, nil)
	}
}
