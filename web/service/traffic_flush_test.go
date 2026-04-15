package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
	"gorm.io/gorm"
)

func TestTrafficPendingStoreMerge(t *testing.T) {
	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))

	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 7}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", DownDelta: 9}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected one merged delta, got %d", len(deltas))
	}
	if deltas[0].UpDelta != 7 || deltas[0].DownDelta != 9 {
		t.Fatalf("unexpected merged delta: %+v", deltas[0])
	}
}

func TestCollectPersistsInboundOnlyDeltaFromDifference(t *testing.T) {
	setupTestDB(t)
	if err := database.GetDB().Create(&model.Inbound{Id: 1, Tag: "inbound-443", Enable: true}).Error; err != nil {
		t.Fatalf("seed inbound failed: %v", err)
	}

	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	svc := NewTrafficFlushService(store)

	err := svc.Collect(
		[]*xray.Traffic{{Tag: "inbound-443", IsInbound: true, Up: 100, Down: 50}},
		[]*xray.ClientTraffic{{InboundId: 1, Email: "alice@example.com", Up: 70, Down: 20}},
	)
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas, got %d", len(deltas))
	}

	var clientDelta *TrafficDelta
	var inboundOnlyDelta *TrafficDelta
	for i := range deltas {
		switch deltas[i].Kind {
		case TrafficDeltaKindClient:
			clientDelta = &deltas[i]
		case TrafficDeltaKindInboundOnly:
			inboundOnlyDelta = &deltas[i]
		}
	}

	if clientDelta == nil {
		t.Fatal("expected client delta to be persisted")
	}
	if clientDelta.InboundID != 1 || clientDelta.Email != "alice@example.com" || clientDelta.UpDelta != 70 || clientDelta.DownDelta != 20 {
		t.Fatalf("unexpected client delta: %+v", *clientDelta)
	}

	if inboundOnlyDelta == nil {
		t.Fatal("expected inbound-only delta to be persisted")
	}
	if inboundOnlyDelta.InboundID != 1 || inboundOnlyDelta.Email != "" || inboundOnlyDelta.UpDelta != 30 || inboundOnlyDelta.DownDelta != 30 {
		t.Fatalf("unexpected inbound-only delta: %+v", *inboundOnlyDelta)
	}
}

func TestFlushOnceClearsPendingOnSuccess(t *testing.T) {
	setupTestDB(t)

	if err := database.GetDB().Create(&model.Inbound{Id: 1, Tag: "inbound-443", Enable: true}).Error; err != nil {
		t.Fatalf("seed inbound failed: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: 1, Email: "alice@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("seed client traffic failed: %v", err)
	}

	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 7, DownDelta: 9}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	svc := NewTrafficFlushService(store)
	if err := svc.FlushOnce(); err != nil {
		t.Fatalf("FlushOnce error: %v", err)
	}

	var clientTraffic xray.ClientTraffic
	if err := database.GetDB().First(&clientTraffic, "inbound_id = ? AND email = ?", 1, "alice@example.com").Error; err != nil {
		t.Fatalf("lookup client traffic failed: %v", err)
	}
	if clientTraffic.Up != 7 || clientTraffic.Down != 9 {
		t.Fatalf("unexpected flushed traffic: %+v", clientTraffic)
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 0 {
		t.Fatalf("expected pending deltas to be cleared, got %+v", deltas)
	}
}

func TestFlushOnceAppliesInboundOnlyDeltaWithoutCreatingClientTraffic(t *testing.T) {
	setupTestDB(t)

	if err := database.GetDB().Create(&model.Inbound{Id: 1, Tag: "inbound-443", Enable: true}).Error; err != nil {
		t.Fatalf("seed inbound failed: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: 1, Email: "alice@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("seed client traffic failed: %v", err)
	}

	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	if err := store.Merge([]TrafficDelta{
		{Kind: TrafficDeltaKindClient, InboundID: 1, Email: "alice@example.com", UpDelta: 7, DownDelta: 9},
		{Kind: TrafficDeltaKindInboundOnly, InboundID: 1, UpDelta: 3, DownDelta: 4},
	}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	svc := NewTrafficFlushService(store)
	if err := svc.FlushOnce(); err != nil {
		t.Fatalf("FlushOnce error: %v", err)
	}

	var inbound model.Inbound
	if err := database.GetDB().First(&inbound, "id = ?", 1).Error; err != nil {
		t.Fatalf("lookup inbound failed: %v", err)
	}
	if inbound.Up != 10 || inbound.Down != 13 || inbound.AllTime != 23 {
		t.Fatalf("unexpected inbound totals: %+v", inbound)
	}

	var clientTraffic xray.ClientTraffic
	if err := database.GetDB().First(&clientTraffic, "inbound_id = ? AND email = ?", 1, "alice@example.com").Error; err != nil {
		t.Fatalf("lookup client traffic failed: %v", err)
	}
	if clientTraffic.Up != 7 || clientTraffic.Down != 9 {
		t.Fatalf("unexpected flushed client traffic: %+v", clientTraffic)
	}

	var count int64
	if err := database.GetDB().Model(&xray.ClientTraffic{}).Where("inbound_id = ? AND email = ?", 1, "").Count(&count).Error; err != nil {
		t.Fatalf("count inbound-only client rows failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no client_traffics row for inbound-only delta, got %d", count)
	}
}

func TestCollectClampsNegativeResidualAndLogsDetailedWarning(t *testing.T) {
	setupTestDB(t)
	if err := database.GetDB().Create(&model.Inbound{Id: 1, Tag: "inbound-443", Enable: true}).Error; err != nil {
		t.Fatalf("seed inbound failed: %v", err)
	}

	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	svc := NewTrafficFlushService(store)

	err := svc.Collect(
		[]*xray.Traffic{{Tag: "inbound-443", IsInbound: true, Up: 10, Down: 5}},
		[]*xray.ClientTraffic{{InboundId: 1, Email: "alice@example.com", Up: 12, Down: 7}},
	)
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected only client delta after clamping negative residual, got %d deltas: %+v", len(deltas), deltas)
	}
	if deltas[0].Kind != TrafficDeltaKindClient {
		t.Fatalf("expected remaining delta to be client kind, got %+v", deltas[0])
	}

	logs := logger.GetLogs(200, "WARNING")
	joined := strings.Join(logs, "\n")
	for _, want := range []string{
		"shared traffic residual below zero",
		"tag=inbound-443",
		"inbound_id=1",
		"inbound_up=10",
		"inbound_down=5",
		"client_up=12",
		"client_down=7",
		"residual_up=-2",
		"residual_down=-2",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected warning logs to contain %q, logs=%s", want, joined)
		}
	}
}

func TestFlushOnceKeepsPendingOnFailure(t *testing.T) {
	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 3}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	svc := NewTrafficFlushService(store)
	svc.flushFn = func([]TrafficDelta) error { return errors.New("boom") }

	if err := svc.FlushOnce(); err == nil {
		t.Fatal("expected flush failure")
	}

	deltas, err := store.Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected pending delta to remain, got %+v", deltas)
	}
}

func TestFlushOnceMarksRestartWhenReconciliationRequiresIt(t *testing.T) {
	setupTestDB(t)

	if err := database.GetDB().Create(&model.Inbound{Id: 1, Tag: "inbound-443", Enable: true}).Error; err != nil {
		t.Fatalf("seed inbound failed: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{InboundId: 1, Email: "alice@example.com", Enable: true}).Error; err != nil {
		t.Fatalf("seed client traffic failed: %v", err)
	}

	store := NewTrafficPendingStore(filepath.Join(t.TempDir(), "traffic-pending.json"))
	if err := store.Merge([]TrafficDelta{{InboundID: 1, Email: "alice@example.com", UpDelta: 1}}); err != nil {
		t.Fatalf("Merge error: %v", err)
	}

	restartMarked := false
	svc := NewTrafficFlushService(store)
	svc.reconcileFn = func(*gorm.DB) (bool, error) { return true, nil }
	svc.markRestart = func() { restartMarked = true }

	if err := svc.FlushOnce(); err != nil {
		t.Fatalf("FlushOnce error: %v", err)
	}
	if !restartMarked {
		t.Fatal("expected flush to mark restart when reconciliation requires it")
	}
}
