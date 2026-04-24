package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/web/session"

	"github.com/gin-gonic/gin"
)

// APIController handles the main API routes for the 3x-ui panel, including inbounds and server management.
type APIController struct {
	BaseController
	inboundController *InboundController
	serverController  *ServerController
	userController    *UserController
	nodeController    *NodeController
	Tgbot             service.Tgbot
}

// NewAPIController creates a new APIController instance and initializes its routes.
func NewAPIController(g *gin.RouterGroup) *APIController {
	a := &APIController{}
	a.initRouter(g)
	return a
}

// checkAPIAuth is a middleware that returns 404 for unauthenticated API requests
// to hide the existence of API endpoints from unauthorized users
func (a *APIController) checkAPIAuth(c *gin.Context) {
	if !session.IsLogin(c) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	c.Next()
}

// initRouter sets up the API routes for inbounds, server, and other endpoints.
func (a *APIController) initRouter(g *gin.RouterGroup) {
	// Main API group
	api := g.Group("/panel/api")
	api.Use(a.checkAPIAuth)

	// Inbounds API
	inbounds := api.Group("/inbounds")
	a.inboundController = &InboundController{}
	inbounds.GET("/userInfo", a.inboundController.getUserInfo)
	inbounds.Use(a.checkAdmin)
	a.inboundController.initRouter(inbounds)

	// Server API
	server := api.Group("/server")
	server.Use(a.checkAdmin)
	a.serverController = NewServerController(server)

	// Users API
	users := api.Group("/users")
	users.Use(a.checkAdmin)
	a.userController = NewUserController(users)

	// Nodes API
	nodes := api.Group("/nodes")
	nodes.Use(a.checkAdmin)
	a.nodeController = NewNodeController(nodes)

	// Extra routes
	api.GET("/backuptotgbot", a.checkAdmin, a.BackuptoTgbot)
}

// BackuptoTgbot sends a backup of the panel data to Telegram bot admins.
func (a *APIController) BackuptoTgbot(c *gin.Context) {
	a.Tgbot.SendBackupToAdmins()
}
