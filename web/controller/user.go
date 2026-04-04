package controller

import (
	"errors"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"
)

type managedUserForm struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
	Role     string `json:"role" form:"role"`
}

// UserController handles admin user management APIs.
type UserController struct {
	userService    service.UserService
	inboundService service.InboundService
}

// NewUserController creates a new UserController and initializes its routes.
func NewUserController(g *gin.RouterGroup) *UserController {
	a := &UserController{}
	a.initRouter(g)
	return a
}

func (a *UserController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.listUsers)
	g.POST("/add", a.addUser)
	g.POST("/update/:id", a.updateUser)
	g.POST("/del/:id", a.deleteUser)
}

func (a *UserController) listUsers(c *gin.Context) {
	users, err := a.userService.GetUsers()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.obtain"), err)
		return
	}
	jsonObj(c, users, nil)
}

func (a *UserController) addUser(c *gin.Context) {
	form := &managedUserForm{}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.create"), err)
		return
	}

	user, err := a.userService.CreateUser(form.Username, form.Password, form.Role, &a.inboundService)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.create"), a.localizeUserError(c, err))
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.users.toasts.create"), user, nil)
}

func (a *UserController) updateUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.update"), err)
		return
	}

	form := &managedUserForm{}
	if err := c.ShouldBind(form); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.update"), err)
		return
	}

	currentUser := session.GetLoginUser(c)
	user, err := a.userService.UpdateManagedUser(id, form.Username, form.Password, form.Role, currentUser.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.update"), a.localizeUserError(c, err))
		return
	}

	if currentUser != nil && currentUser.Id == id {
		currentUser.Username = user.Username
		currentUser.Role = user.Role
		if form.Password != "" {
			currentUser.Password, _ = crypto.HashPasswordAsBcrypt(form.Password)
		}
		session.SetLoginUser(c, currentUser)
		if saveErr := sessions.Default(c).Save(); saveErr != nil {
			jsonMsg(c, I18nWeb(c, "pages.users.toasts.update"), saveErr)
			return
		}
	}

	jsonMsgObj(c, I18nWeb(c, "pages.users.toasts.update"), user, nil)
}

func (a *UserController) deleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.delete"), err)
		return
	}

	currentUser := session.GetLoginUser(c)
	err = a.userService.DeleteUser(id, currentUser.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.users.toasts.delete"), a.localizeUserError(c, err))
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.users.toasts.delete"), nil)
}

func (a *UserController) localizeUserError(c *gin.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrUsernameAlreadyExists):
		return errors.New(I18nWeb(c, "pages.users.errors.userExists"))
	case errors.Is(err, service.ErrInvalidUserRole):
		return errors.New(I18nWeb(c, "pages.users.errors.invalidRole"))
	case errors.Is(err, service.ErrUserNotFound):
		return errors.New(I18nWeb(c, "pages.users.errors.userNotFound"))
	case errors.Is(err, service.ErrCannotDeleteSelf):
		return errors.New(I18nWeb(c, "pages.users.errors.cannotDeleteSelf"))
	case errors.Is(err, service.ErrLastAdminRequired):
		return errors.New(I18nWeb(c, "pages.users.errors.lastAdminRequired"))
	case errors.Is(err, service.ErrCannotDemoteSelf):
		return errors.New(I18nWeb(c, "pages.users.errors.cannotDemoteSelf"))
	default:
		return err
	}
}
