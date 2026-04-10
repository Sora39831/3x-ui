package service

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

func writeNodeSyncSettings(t *testing.T, nodeID string) {
	t.Helper()
	writeNodeGuardSettings(t, map[string]any{
		"dbType":   "mariadb",
		"nodeRole": "worker",
		"nodeId":   nodeID,
	})
}

func loadNodeState(t *testing.T, nodeID string) *model.NodeState {
	t.Helper()
	state := &model.NodeState{}
	if err := database.GetDB().First(state, "node_id = ?", nodeID).Error; err != nil {
		t.Fatalf("load node state error: %v", err)
	}
	return state
}

func TestLoadAndSaveSharedAccountsSnapshot(t *testing.T) {
	setupTestDB(t)

	cachePath := filepath.Join(t.TempDir(), "shared-cache.json")
	snapshot := &SharedAccountsSnapshot{
		Version: 7,
		Inbounds: []*model.Inbound{
			{
				Id:       11,
				Enable:   true,
				Port:     443,
				Protocol: model.VLESS,
				Settings: `{"clients":[{"id":"u-1","email":"alice@example.com"}]}`,
			},
		},
	}

	if err := SaveSharedAccountsSnapshot(cachePath, snapshot); err != nil {
		t.Fatalf("SaveSharedAccountsSnapshot error: %v", err)
	}

	loaded, err := LoadSharedAccountsSnapshot(cachePath)
	if err != nil {
		t.Fatalf("LoadSharedAccountsSnapshot error: %v", err)
	}
	if loaded.Version != snapshot.Version {
		t.Fatalf("expected version %d, got %d", snapshot.Version, loaded.Version)
	}
	if len(loaded.Inbounds) != 1 || loaded.Inbounds[0].Tag != snapshot.Inbounds[0].Tag {
		t.Fatalf("expected one inbound to round-trip")
	}
}

func TestSyncOnceSkipsApplyWhenVersionUnchanged(t *testing.T) {
	setupTestDB(t)
	writeNodeSyncSettings(t, "worker-skip")

	cachePath := filepath.Join(t.TempDir(), "shared-cache.json")
	applyCalled := false
	loadSnapshotCalled := false
	syncSvc := &NodeSyncService{
		cachePath:        cachePath,
		lastSeenVersion:  9,
		loadVersion:      func() (int64, error) { return 9, nil },
		loadSnapshot:     func() (*SharedAccountsSnapshot, error) { loadSnapshotCalled = true; return nil, nil },
		applySnapshot:    func(*SharedAccountsSnapshot) error { applyCalled = true; return nil },
	}

	didSync, err := syncSvc.SyncOnce()
	if err != nil {
		t.Fatalf("SyncOnce error: %v", err)
	}
	if didSync {
		t.Fatal("expected unchanged version to skip sync")
	}
	if loadSnapshotCalled {
		t.Fatal("loadSnapshot should not be called when version is unchanged")
	}
	if applyCalled {
		t.Fatal("applySnapshot should not be called when version is unchanged")
	}

	state := loadNodeState(t, "worker-skip")
	if state.LastSeenVersion != 9 {
		t.Fatalf("expected last seen version 9, got %d", state.LastSeenVersion)
	}
	if state.LastSyncAt != 0 {
		t.Fatalf("expected LastSyncAt to remain 0, got %d", state.LastSyncAt)
	}
	if state.LastHeartbeatAt == 0 {
		t.Fatal("expected heartbeat timestamp to be recorded")
	}
}

func TestSyncOnceRefreshesCacheAndAppliesSnapshot(t *testing.T) {
	setupTestDB(t)
	writeNodeSyncSettings(t, "worker-refresh")

	cachePath := filepath.Join(t.TempDir(), "shared-cache.json")
	wantSnapshot := &SharedAccountsSnapshot{
		Version: 12,
		Inbounds: []*model.Inbound{
			{
				Id:             100,
				Enable:         true,
				Port:           8443,
				Protocol:       model.VLESS,
				Settings:       `{"clients":[]}`,
				StreamSettings: `{"network":"tcp","tlsSettings":{"settings":{"allowInsecure":true}}}`,
				Tag:            "in-100",
			},
		},
	}

	applyCalls := 0
	syncSvc := &NodeSyncService{
		cachePath:        cachePath,
		lastSeenVersion:  11,
		loadVersion:      func() (int64, error) { return 12, nil },
		loadSnapshot:     func() (*SharedAccountsSnapshot, error) { return wantSnapshot, nil },
		applySnapshot:    func(snapshot *SharedAccountsSnapshot) error { applyCalls++; return nil },
	}

	didSync, err := syncSvc.SyncOnce()
	if err != nil {
		t.Fatalf("SyncOnce error: %v", err)
	}
	if !didSync {
		t.Fatal("expected sync to run when version changes")
	}
	if applyCalls != 1 {
		t.Fatalf("expected applySnapshot to be called once, got %d", applyCalls)
	}
	if syncSvc.lastSeenVersion != 12 {
		t.Fatalf("expected lastSeenVersion to become 12, got %d", syncSvc.lastSeenVersion)
	}

	cached, err := LoadSharedAccountsSnapshot(cachePath)
	if err != nil {
		t.Fatalf("LoadSharedAccountsSnapshot error: %v", err)
	}
	if cached.Version != 12 {
		t.Fatalf("expected cached version 12, got %d", cached.Version)
	}

	state := loadNodeState(t, "worker-refresh")
	if state.LastSeenVersion != 12 {
		t.Fatalf("expected last seen version 12, got %d", state.LastSeenVersion)
	}
	if state.LastSyncAt == 0 {
		t.Fatal("expected LastSyncAt to be recorded after successful sync")
	}
	if state.LastError != "" {
		t.Fatalf("expected empty LastError, got %q", state.LastError)
	}
}

func TestSyncOncePreservesLastSyncAtWhenVersionUnchanged(t *testing.T) {
	setupTestDB(t)
	writeNodeSyncSettings(t, "worker-preserve")

	if err := database.UpsertNodeState(database.GetDB(), &model.NodeState{
		NodeID:          "worker-preserve",
		NodeRole:        "worker",
		LastSyncAt:      12345,
		LastHeartbeatAt: 12345,
		LastSeenVersion: 8,
	}); err != nil {
		t.Fatalf("UpsertNodeState error: %v", err)
	}

	syncSvc := &NodeSyncService{
		cachePath:       filepath.Join(t.TempDir(), "shared-cache.json"),
		lastSeenVersion: 8,
		loadVersion:     func() (int64, error) { return 8, nil },
		loadSnapshot:    func() (*SharedAccountsSnapshot, error) { return nil, nil },
		applySnapshot:   func(*SharedAccountsSnapshot) error { return nil },
	}

	didSync, err := syncSvc.SyncOnce()
	if err != nil {
		t.Fatalf("SyncOnce error: %v", err)
	}
	if didSync {
		t.Fatal("expected unchanged version to skip sync")
	}

	state := loadNodeState(t, "worker-preserve")
	if state.LastSyncAt != 12345 {
		t.Fatalf("expected LastSyncAt to remain 12345, got %d", state.LastSyncAt)
	}
}

func TestBootstrapFromCacheAppliesCachedSnapshot(t *testing.T) {
	setupTestDB(t)
	writeNodeSyncSettings(t, "worker-bootstrap")

	cachePath := filepath.Join(t.TempDir(), "shared-cache.json")
	cached := &SharedAccountsSnapshot{
		Version: 77,
		Inbounds: []*model.Inbound{
			{
				Id:       77,
				Enable:   true,
				Port:     10077,
				Protocol: model.VLESS,
				Settings: `{"clients":[]}`,
				Tag:      "cache-77",
			},
		},
	}
	if err := SaveSharedAccountsSnapshot(cachePath, cached); err != nil {
		t.Fatalf("SaveSharedAccountsSnapshot error: %v", err)
	}

	appliedVersion := int64(0)
	syncSvc := &NodeSyncService{
		cachePath: cachePath,
		applySnapshot: func(snapshot *SharedAccountsSnapshot) error {
			appliedVersion = snapshot.Version
			return nil
		},
	}

	if err := syncSvc.BootstrapFromCache(); err != nil {
		t.Fatalf("BootstrapFromCache error: %v", err)
	}
	if appliedVersion != 77 {
		t.Fatalf("expected cached version 77 to be applied, got %d", appliedVersion)
	}
}
