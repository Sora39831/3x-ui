package service

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type NodeSyncService struct {
	xrayService     XrayService
	cachePath       string
	lastSeenVersion int64
	loadVersion     func() (int64, error)
	loadSnapshot    func() (*SharedAccountsSnapshot, error)
	applySnapshot   func(*SharedAccountsSnapshot) error
}

func NewNodeSyncService() *NodeSyncService {
	svc := &NodeSyncService{
		cachePath: config.GetSharedCachePath(),
	}
	svc.loadVersion = func() (int64, error) {
		return database.GetSharedAccountsVersion(database.GetDB())
	}
	svc.loadSnapshot = func() (*SharedAccountsSnapshot, error) {
		inbounds, err := svc.xrayService.inboundService.GetAllInbounds()
		if err != nil {
			return nil, err
		}
		return &SharedAccountsSnapshot{Inbounds: inbounds}, nil
	}
	svc.applySnapshot = svc.xrayService.ApplySharedSnapshot
	return svc
}

func (s *NodeSyncService) updateNodeState(version int64, syncErr error, didSync bool) {
	nodeCfg := config.GetNodeConfigFromJSON()
	if nodeCfg.NodeID == "" {
		return
	}
	now := time.Now().Unix()
	state := &model.NodeState{}
	if err := database.GetDB().First(state, "node_id = ?", nodeCfg.NodeID).Error; err != nil {
		// First heartbeat — record doesn't exist yet, that's OK
		state = &model.NodeState{}
	}
	state.NodeID = nodeCfg.NodeID
	state.NodeRole = string(nodeCfg.Role)
	state.LastHeartbeatAt = now
	state.LastSeenVersion = version
	if didSync {
		state.LastSyncAt = now
	}
	if syncErr != nil {
		state.LastError = syncErr.Error()
	} else {
		state.LastError = ""
	}
	if err := database.UpsertNodeState(database.GetDB(), state); err != nil {
		log.Printf("[NodeSync] failed to upsert node state for %s: %v", nodeCfg.NodeID, err)
	}

	// Master also writes heartbeat to shared MariaDB so workers can see it
	if nodeCfg.Role == config.NodeRoleMaster {
		s.writeStateToSharedMariaDB(state)
	}
}

// writeStateToSharedMariaDB opens a temporary connection to the shared
// MariaDB and upserts the given node state. This is needed when the master
// uses SQLite locally but workers query the shared MariaDB for heartbeats.
func (s *NodeSyncService) writeStateToSharedMariaDB(state *model.NodeState) {
	dbConfig := config.GetDBConfigFromJSON()
	// Only attempt shared write if MariaDB connection settings are configured.
	// dbUser is the most reliable indicator — it has no default value.
	if dbConfig.User == "" || dbConfig.Host == "" {
		return
	}
	sharedDB, err := database.OpenMariaDB(dbConfig)
	if err != nil {
		log.Printf("[NodeSync] failed to open shared MariaDB for heartbeat: %v", err)
		return
	}
	sqlDB, _ := sharedDB.DB()
	defer sqlDB.Close()
	if err := database.UpsertNodeState(sharedDB, state); err != nil {
		log.Printf("[NodeSync] failed to upsert node state to shared MariaDB: %v", err)
	}
}

func (s *NodeSyncService) BootstrapFromCache() error {
	snapshot, err := LoadSharedAccountsSnapshot(s.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if snapshot == nil {
		return errors.New("shared snapshot is nil")
	}
	if err := s.applySnapshot(snapshot); err != nil {
		return err
	}
	s.lastSeenVersion = snapshot.Version
	return nil
}

func (s *NodeSyncService) SyncOnce() (bool, error) {
	version, err := s.loadVersion()
	if err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return false, err
	}
	if version == s.lastSeenVersion {
		s.updateNodeState(version, nil, false)
		return false, nil
	}

	snapshot, err := s.loadSnapshot()
	if err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return false, err
	}
	if snapshot == nil {
		err = errors.New("shared snapshot is nil")
		s.updateNodeState(s.lastSeenVersion, err, false)
		return false, err
	}

	snapshot.Version = version
	if err := SaveSharedAccountsSnapshot(s.cachePath, snapshot); err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return false, err
	}
	if err := s.applySnapshot(snapshot); err != nil {
		s.updateNodeState(s.lastSeenVersion, err, false)
		return false, err
	}

	s.lastSeenVersion = version
	s.updateNodeState(version, nil, true)
	return true, nil
}

func (s *NodeSyncService) Run(ctx context.Context, interval time.Duration) {
	_ = s.BootstrapFromCache()
	_, _ = s.SyncOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.SyncOnce()
		}
	}
}

func (s *NodeSyncService) RunHeartbeatLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			version, _ := database.GetSharedAccountsVersion(database.GetDB())
			s.updateNodeState(version, nil, false)
		}
	}
}
