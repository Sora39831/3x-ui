package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
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
