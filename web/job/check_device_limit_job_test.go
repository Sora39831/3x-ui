package job

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

func resetDeviceLimitJobGlobals() {
	activeClientsLock.Lock()
	activeClientIPs = make(map[string]map[string]time.Time)
	activeClientsLock.Unlock()

	clientStatusMu.Lock()
	clientStatus = make(map[string]bool)
	clientStatusMu.Unlock()
}

func TestCheckDeviceLimitJob_Run_SkipWhenAlreadyRunning(t *testing.T) {
	resetDeviceLimitJobGlobals()

	j := NewCheckDeviceLimitJob(nil)
	j.running.Store(true)
	j.isXrayRunning = func() bool {
		t.Fatal("Run should skip execution when already running")
		return true
	}

	j.Run()
}

func TestCheckDeviceLimitJob_UnbanWhenEnforcementDisabled(t *testing.T) {
	resetDeviceLimitJobGlobals()

	activeClientsLock.Lock()
	activeClientIPs["alice@example.com"] = map[string]time.Time{
		"1.2.3.4": time.Now(),
	}
	activeClientsLock.Unlock()

	clientStatusMu.Lock()
	clientStatus["alice@example.com"] = true
	clientStatusMu.Unlock()

	j := NewCheckDeviceLimitJob(nil)
	j.getAPIPort = func() int { return 10085 }
	j.apiInit = func(int) error { return nil }
	j.apiClose = func() {}
	j.sleep = func(time.Duration) {}
	j.loadAllInbounds = func() ([]*model.Inbound, error) {
		return []*model.Inbound{
			{
				Id:          1,
				Enable:      false, // Enforcement disabled
				DeviceLimit: 0,
				Tag:         "inbound-10001",
				Protocol:    model.VLESS,
			},
		}, nil
	}
	j.getClientTraffic = func(email string) (*xray.ClientTraffic, error) {
		return &xray.ClientTraffic{InboundId: 1, Email: email}, nil
	}
	j.getClientByEmail = func(email string) (*xray.ClientTraffic, *model.Client, error) {
		return &xray.ClientTraffic{InboundId: 1, Email: email}, &model.Client{ID: "orig-id", Email: email}, nil
	}

	removeCalls := 0
	addCalls := 0
	j.removeUser = func(inboundTag, email string) error {
		removeCalls++
		return nil
	}
	j.addUser = func(protocol, inboundTag string, user map[string]any) error {
		addCalls++
		return nil
	}

	j.checkAllClientsLimit()

	if removeCalls != 1 || addCalls != 1 {
		t.Fatalf("expected one restore cycle, got remove=%d add=%d", removeCalls, addCalls)
	}
	if j.isClientBanned("alice@example.com") {
		t.Fatal("expected client ban flag to be cleared when enforcement is disabled")
	}
}

func TestCheckDeviceLimitJob_ClearStaleBanWhenInboundMissing(t *testing.T) {
	resetDeviceLimitJobGlobals()

	clientStatusMu.Lock()
	clientStatus["ghost@example.com"] = true
	clientStatusMu.Unlock()

	j := NewCheckDeviceLimitJob(nil)
	j.getAPIPort = func() int { return 10085 }
	j.apiInit = func(int) error { return nil }
	j.apiClose = func() {}
	j.sleep = func(time.Duration) {}
	j.loadAllInbounds = func() ([]*model.Inbound, error) {
		return []*model.Inbound{
			{Id: 2, Enable: true, DeviceLimit: 1, Tag: "inbound-10002", Protocol: model.VLESS},
		}, nil
	}
	j.getClientTraffic = func(email string) (*xray.ClientTraffic, error) {
		return &xray.ClientTraffic{InboundId: 999, Email: email}, nil
	}
	j.getClientByEmail = func(email string) (*xray.ClientTraffic, *model.Client, error) {
		t.Fatal("GetClientByEmail should not be called when inbound is missing")
		return nil, nil, nil
	}
	j.removeUser = func(inboundTag, email string) error {
		t.Fatal("RemoveUser should not be called when inbound is missing")
		return nil
	}
	j.addUser = func(protocol, inboundTag string, user map[string]any) error {
		t.Fatal("AddUser should not be called when inbound is missing")
		return nil
	}

	j.checkAllClientsLimit()

	if j.isClientBanned("ghost@example.com") {
		t.Fatal("expected stale banned status to be cleared when inbound no longer exists")
	}
}
