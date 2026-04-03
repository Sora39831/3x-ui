package xray

import (
	"testing"
)

func TestInboundConfig_Equals_Equal(t *testing.T) {
	a := &InboundConfig{
		Listen:         []byte(`"0.0.0.0"`),
		Port:           443,
		Protocol:       "vless",
		Settings:       []byte(`{}`),
		StreamSettings: []byte(`{"network":"tcp"}`),
		Tag:            "inbound-443",
		Sniffing:       []byte(`{"enabled":true}`),
	}
	b := &InboundConfig{
		Listen:         []byte(`"0.0.0.0"`),
		Port:           443,
		Protocol:       "vless",
		Settings:       []byte(`{}`),
		StreamSettings: []byte(`{"network":"tcp"}`),
		Tag:            "inbound-443",
		Sniffing:       []byte(`{"enabled":true}`),
	}
	if !a.Equals(b) {
		t.Error("identical InboundConfigs should be equal")
	}
}

func TestInboundConfig_Equals_DifferentPort(t *testing.T) {
	a := &InboundConfig{Port: 443, Protocol: "vless"}
	b := &InboundConfig{Port: 8443, Protocol: "vless"}
	if a.Equals(b) {
		t.Error("InboundConfigs with different ports should not be equal")
	}
}

func TestInboundConfig_Equals_DifferentProtocol(t *testing.T) {
	a := &InboundConfig{Port: 443, Protocol: "vless"}
	b := &InboundConfig{Port: 443, Protocol: "trojan"}
	if a.Equals(b) {
		t.Error("InboundConfigs with different protocols should not be equal")
	}
}

func TestInboundConfig_Equals_DifferentTag(t *testing.T) {
	a := &InboundConfig{Port: 443, Protocol: "vless", Tag: "tag-a"}
	b := &InboundConfig{Port: 443, Protocol: "vless", Tag: "tag-b"}
	if a.Equals(b) {
		t.Error("InboundConfigs with different tags should not be equal")
	}
}

func TestInboundConfig_Equals_NilRawMessages(t *testing.T) {
	a := &InboundConfig{Port: 443, Protocol: "vless", Listen: nil, Settings: nil}
	b := &InboundConfig{Port: 443, Protocol: "vless", Listen: nil, Settings: nil}
	if !a.Equals(b) {
		t.Error("InboundConfigs with nil RawMessages should be equal")
	}
}

func TestInboundConfig_Equals_DifferentListen(t *testing.T) {
	a := &InboundConfig{Listen: []byte(`"0.0.0.0"`), Port: 443}
	b := &InboundConfig{Listen: []byte(`"127.0.0.1"`), Port: 443}
	if a.Equals(b) {
		t.Error("InboundConfigs with different Listen should not be equal")
	}
}

func TestConfig_Equals_Equal(t *testing.T) {
	a := &Config{
		LogConfig:    []byte(`{"loglevel":"info"}`),
		RouterConfig: []byte(`{}`),
		InboundConfigs: []InboundConfig{
			{Port: 443, Protocol: "vless"},
		},
	}
	b := &Config{
		LogConfig:    []byte(`{"loglevel":"info"}`),
		RouterConfig: []byte(`{}`),
		InboundConfigs: []InboundConfig{
			{Port: 443, Protocol: "vless"},
		},
	}
	if !a.Equals(b) {
		t.Error("identical Configs should be equal")
	}
}

func TestConfig_Equals_DifferentInboundCount(t *testing.T) {
	a := &Config{
		InboundConfigs: []InboundConfig{{Port: 443}},
	}
	b := &Config{
		InboundConfigs: []InboundConfig{},
	}
	if a.Equals(b) {
		t.Error("Configs with different inbound counts should not be equal")
	}
}

func TestConfig_Equals_DifferentLogConfig(t *testing.T) {
	a := &Config{LogConfig: []byte(`{"loglevel":"info"}`)}
	b := &Config{LogConfig: []byte(`{"loglevel":"debug"}`)}
	if a.Equals(b) {
		t.Error("Configs with different LogConfig should not be equal")
	}
}

func TestConfig_Equals_EmptyConfigs(t *testing.T) {
	a := &Config{}
	b := &Config{}
	if !a.Equals(b) {
		t.Error("two empty Configs should be equal")
	}
}

func TestConfig_Equals_NilVsEmpty(t *testing.T) {
	a := &Config{}
	b := &Config{InboundConfigs: []InboundConfig{}}
	if !a.Equals(b) {
		t.Error("nil and empty slice InboundConfigs should be equal")
	}
}
