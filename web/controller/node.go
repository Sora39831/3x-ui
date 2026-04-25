package controller

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"

	"github.com/gin-gonic/gin"
)

// NodeController handles node management API endpoints.
type NodeController struct {
	BaseController
}

// NewNodeController creates a new NodeController and initializes its routes.
func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{}
	a.initRouter(g)
	return a
}

// initRouter sets up the routes for node management.
func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/config", a.getConfig)
	g.POST("/config", a.updateConfig)
}

// NodeView is the JSON shape returned to the frontend for each node.
type NodeView struct {
	NodeID          string `json:"nodeId"`
	NodeRole        string `json:"nodeRole"`
	Online          bool   `json:"online"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
	LastSyncAt      int64  `json:"lastSyncAt"`
	LastSeenVersion int64  `json:"lastSeenVersion"`
	LastError       string `json:"lastError"`
}

// getNodeStatesFromShared queries node_states from the shared MariaDB.
// In shared mode, the master may use SQLite locally, so we must query
// the shared MariaDB directly to see worker heartbeats.
func getNodeStatesFromShared() ([]model.NodeState, error) {
	// If current DB is already MariaDB, use it directly
	if config.GetDBTypeFromJSON() == "mariadb" {
		states, err := database.GetNodeStates()
		if err != nil {
			log.Printf("[NodeList] GetNodeStates error: %v", err)
		}
		return states, err
	}

	// Otherwise, open a temporary connection to the shared MariaDB
	dbConfig := config.GetDBConfigFromJSON()
	db, err := database.OpenMariaDB(dbConfig)
	if err != nil {
		log.Printf("[NodeList] failed to open shared MariaDB: %v", err)
		return nil, err
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var states []model.NodeState
	err = db.Order("node_id").Find(&states).Error
	if err != nil {
		log.Printf("[NodeList] failed to query shared MariaDB node_states: %v", err)
	}
	return states, err
}

// list returns connected nodes. Master sees all workers; worker sees the master.
func (a *NodeController) list(c *gin.Context) {
	nodeCfg := config.GetNodeConfigFromJSON()
	states, err := getNodeStatesFromShared()
	if err != nil {
		jsonMsg(c, "get node states", err)
		return
	}
	log.Printf("[NodeList] role=%s nodeId=%s, found %d states in shared DB", nodeCfg.Role, nodeCfg.NodeID, len(states))

	syncInterval := nodeCfg.SyncIntervalSeconds
	if syncInterval <= 0 {
		syncInterval = 30
	}
	offlineThreshold := int64(syncInterval * 2)
	now := time.Now().Unix()

	var nodes []NodeView
	for _, s := range states {
		// Master shows workers; worker shows master
		if nodeCfg.Role == config.NodeRoleMaster && s.NodeRole != string(config.NodeRoleWorker) {
			continue
		}
		if nodeCfg.Role == config.NodeRoleWorker && s.NodeRole != string(config.NodeRoleMaster) {
			continue
		}
		online := (now - s.LastHeartbeatAt) < offlineThreshold
		nodes = append(nodes, NodeView{
			NodeID:          s.NodeID,
			NodeRole:        s.NodeRole,
			Online:          online,
			LastHeartbeatAt: s.LastHeartbeatAt,
			LastSyncAt:      s.LastSyncAt,
			LastSeenVersion: s.LastSeenVersion,
			LastError:       s.LastError,
		})
	}
	if nodes == nil {
		nodes = []NodeView{}
	}

	jsonObj(c, nodes, nil)
}

// NodeConfigView is the JSON shape for node configuration.
type NodeConfigView struct {
	Role                 string `json:"role"`
	NodeID               string `json:"nodeId"`
	SyncInterval         int    `json:"syncInterval"`
	TrafficFlushInterval int    `json:"trafficFlushInterval"`
	DBType               string `json:"dbType"`
	DBHost               string `json:"dbHost"`
	DBPort               string `json:"dbPort"`
	DBUser               string `json:"dbUser"`
	DBPass               string `json:"dbPass"`
	DBName               string `json:"dbName"`
}

// getConfig returns the current node configuration.
func (a *NodeController) getConfig(c *gin.Context) {
	nodeCfg := config.GetNodeConfigFromJSON()
	dbCfg := config.GetDBConfigFromJSON()

	jsonObj(c, NodeConfigView{
		Role:                 string(nodeCfg.Role),
		NodeID:               nodeCfg.NodeID,
		SyncInterval:         nodeCfg.SyncIntervalSeconds,
		TrafficFlushInterval: nodeCfg.TrafficFlushSeconds,
		DBType:               dbCfg.Type,
		DBHost:               dbCfg.Host,
		DBPort:               dbCfg.Port,
		DBUser:               dbCfg.User,
		DBPass:               dbCfg.Password,
		DBName:               dbCfg.Name,
	}, nil)
}

// updateConfigRequest is the form body for updating node config.
type updateConfigRequest struct {
	SyncInterval         int    `json:"syncInterval" form:"syncInterval"`
	TrafficFlushInterval int    `json:"trafficFlushInterval" form:"trafficFlushInterval"`
	DBType               string `json:"dbType" form:"dbType"`
	DBHost               string `json:"dbHost" form:"dbHost"`
	DBPort               string `json:"dbPort" form:"dbPort"`
	DBUser               string `json:"dbUser" form:"dbUser"`
	DBPass               string `json:"dbPass" form:"dbPass"`
	DBName               string `json:"dbName" form:"dbName"`
}

// updateConfig updates the node configuration in x-ui.json.
func (a *NodeController) updateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "invalid request", err)
		return
	}

	// Validate
	if req.SyncInterval <= 0 {
		jsonMsg(c, "syncInterval must be positive", os.ErrInvalid)
		return
	}
	if req.TrafficFlushInterval <= 0 {
		jsonMsg(c, "trafficFlushInterval must be positive", os.ErrInvalid)
		return
	}

	// Write each setting to JSON config
	settings := map[string]string{
		"syncInterval":         strconv.Itoa(req.SyncInterval),
		"trafficFlushInterval": strconv.Itoa(req.TrafficFlushInterval),
		"dbType":               req.DBType,
		"dbHost":               req.DBHost,
		"dbPort":               req.DBPort,
		"dbUser":               req.DBUser,
		"dbPassword":           req.DBPass,
		"dbName":               req.DBName,
	}

	for key, value := range settings {
		if err := config.WriteSettingToJSON(key, value); err != nil {
			jsonMsg(c, "save "+key, err)
			return
		}
	}

	jsonMsg(c, I18nWeb(c, "pages.nodes.saveSuccess"), nil)
}
