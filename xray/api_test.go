package xray

import (
	"testing"
)

func TestProcessTraffic_Inbound(t *testing.T) {
	matches := []string{
		"inbound>>>vmess-in>>>traffic>>>uplink",
		"inbound",
		"vmess-in",
		"uplink",
	}
	trafficMap := make(map[string]*Traffic)
	processTraffic(matches, 1024, trafficMap)

	tr, ok := trafficMap["vmess-in"]
	if !ok {
		t.Fatal("should have vmess-in entry")
	}
	if !tr.IsInbound {
		t.Error("should be inbound")
	}
	if tr.IsOutbound {
		t.Error("should not be outbound")
	}
	if tr.Tag != "vmess-in" {
		t.Errorf("tag should be vmess-in, got %q", tr.Tag)
	}
	if tr.Up != 1024 {
		t.Errorf("up should be 1024, got %d", tr.Up)
	}
	if tr.Down != 0 {
		t.Errorf("down should be 0, got %d", tr.Down)
	}
}

func TestProcessTraffic_Outbound(t *testing.T) {
	matches := []string{
		"outbound>>>direct>>>traffic>>>downlink",
		"outbound",
		"direct",
		"downlink",
	}
	trafficMap := make(map[string]*Traffic)
	processTraffic(matches, 2048, trafficMap)

	tr, ok := trafficMap["direct"]
	if !ok {
		t.Fatal("should have direct entry")
	}
	if tr.IsOutbound != true {
		t.Error("should be outbound")
	}
	if tr.IsInbound != false {
		t.Error("should not be inbound")
	}
	if tr.Down != 2048 {
		t.Errorf("down should be 2048, got %d", tr.Down)
	}
}

func TestProcessTraffic_ApiTagSkipped(t *testing.T) {
	matches := []string{
		"inbound>>>api>>>traffic>>>uplink",
		"inbound",
		"api",
		"uplink",
	}
	trafficMap := make(map[string]*Traffic)
	processTraffic(matches, 1024, trafficMap)

	if _, ok := trafficMap["api"]; ok {
		t.Error("api tag should be skipped")
	}
}

func TestProcessTraffic_Aggregates(t *testing.T) {
	trafficMap := make(map[string]*Traffic)

	// First: uplink
	processTraffic([]string{"", "inbound", "test-tag", "uplink"}, 100, trafficMap)
	// Second: downlink on same tag
	processTraffic([]string{"", "inbound", "test-tag", "downlink"}, 200, trafficMap)

	tr := trafficMap["test-tag"]
	if tr.Up != 100 {
		t.Errorf("expected up=100, got %d", tr.Up)
	}
	if tr.Down != 200 {
		t.Errorf("expected down=200, got %d", tr.Down)
	}
}

func TestProcessClientTraffic(t *testing.T) {
	clientMap := make(map[string]*ClientTraffic)

	processClientTraffic([]string{"", "user@example.com", "uplink"}, 500, clientMap)
	processClientTraffic([]string{"", "user@example.com", "downlink"}, 1500, clientMap)

	ct, ok := clientMap["user@example.com"]
	if !ok {
		t.Fatal("should have client entry")
	}
	if ct.Email != "user@example.com" {
		t.Errorf("email should be user@example.com, got %q", ct.Email)
	}
	if ct.Up != 500 {
		t.Errorf("up should be 500, got %d", ct.Up)
	}
	if ct.Down != 1500 {
		t.Errorf("down should be 1500, got %d", ct.Down)
	}
}

func TestProcessClientTraffic_MultipleClients(t *testing.T) {
	clientMap := make(map[string]*ClientTraffic)

	processClientTraffic([]string{"", "user1@test.com", "uplink"}, 100, clientMap)
	processClientTraffic([]string{"", "user2@test.com", "uplink"}, 200, clientMap)

	if len(clientMap) != 2 {
		t.Errorf("expected 2 clients, got %d", len(clientMap))
	}
	if clientMap["user1@test.com"].Up != 100 {
		t.Error("user1 up mismatch")
	}
	if clientMap["user2@test.com"].Up != 200 {
		t.Error("user2 up mismatch")
	}
}

func TestMapToSlice_Empty(t *testing.T) {
	m := make(map[string]*Traffic)
	result := mapToSlice(m)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got length %d", len(result))
	}
}

func TestMapToSlice_Nil(t *testing.T) {
	var m map[string]*Traffic
	result := mapToSlice(m)
	if len(result) != 0 {
		t.Errorf("expected empty slice for nil map, got length %d", len(result))
	}
}

func TestMapToSlice_Multiple(t *testing.T) {
	m := map[string]*Traffic{
		"a": {Tag: "a", Up: 1},
		"b": {Tag: "b", Up: 2},
		"c": {Tag: "c", Up: 3},
	}
	result := mapToSlice(m)
	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
}

func TestXrayAPI_Init_InvalidPort(t *testing.T) {
	api := &XrayAPI{}
	if err := api.Init(0); err == nil {
		t.Error("Init with port 0 should return error")
	}
	if err := api.Init(-1); err == nil {
		t.Error("Init with negative port should return error")
	}
	if err := api.Init(70000); err == nil {
		t.Error("Init with port > 65535 should return error")
	}
}
