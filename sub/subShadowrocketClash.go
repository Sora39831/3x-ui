package sub

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// BuildShadowrocketConfig renders a subscription as Shadowrocket JSON using the
// same server list as the Clash subscription, so both formats expose an
// identical node set (otherwise Shadowrocket would only see the panel host's
// local inbound while Clash clients see every configured server).
func (s *SubClashService) BuildShadowrocketConfig(subId string) (string, string, error) {
	if len(s.servers) == 0 {
		return "", "", nil
	}
	inbounds, err := s.SubService.getInboundsBySubId(subId)
	if err != nil || len(inbounds) == 0 {
		return "", "", err
	}

	var clientTraffics []xray.ClientTraffic
	servers := make([]ShadowrocketServer, 0, len(inbounds)*len(s.servers))
	seen := make(map[string]struct{}, len(inbounds)*len(s.servers))

	for _, inbound := range inbounds {
		clients, err := s.inboundService.GetClients(inbound)
		if err != nil {
			logger.Error("SubClashService - GetClients: Unable to get clients from inbound")
		}
		if clients == nil {
			continue
		}
		if len(inbound.Listen) > 0 && inbound.Listen[0] == '@' {
			listen, port, streamSettings, err := s.SubService.getFallbackMaster(inbound.Listen, inbound.StreamSettings)
			if err == nil {
				inbound.Listen = listen
				inbound.Port = port
				inbound.StreamSettings = streamSettings
			}
		}

		for _, client := range clients {
			if client.SubID != subId {
				continue
			}
			clientTraffics = append(clientTraffics, s.SubService.getClientTraffics(inbound.ClientStats, client.Email))
			for _, server := range s.servers {
				srv, ok := buildShadowrocketServer(inbound, client, server.Server, server.Name)
				if !ok {
					continue
				}
				key := srv.Add + "|" + srv.Port + "|" + srv.Password
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				servers = append(servers, srv)
			}
		}
	}

	if len(servers) == 0 {
		return "", "", nil
	}

	payload, err := json.Marshal(shadowrocketConfig{Servers: servers})
	if err != nil {
		return "", "", err
	}

	traffic := aggregateClientTraffics(clientTraffics)
	header := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return string(payload), header, nil
}

// buildShadowrocketServer converts a single inbound/client pair into a
// Shadowrocket node hosted at the given address and display name.
func buildShadowrocketServer(inbound *model.Inbound, client model.Client, add, title string) (ShadowrocketServer, bool) {
	var stream map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		logger.Warning("SubShadowrocketService - failed to parse StreamSettings for inbound", inbound.Tag, ":", err)
		return ShadowrocketServer{}, false
	}

	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)

	srv := ShadowrocketServer{
		Add:      add,
		Host:     add,
		Port:     strconv.Itoa(inbound.Port),
		Type:     "VLESS",
		Title:    title,
		TLS:      security == "tls" || security == "reality",
		UDP:      1,
		Obfs:     "none",
		Flag:     flagFromTitle(title),
		Proto:    "auto",
		Reserved: []any{},
	}

	switch inbound.Protocol {
	case model.VLESS:
		srv.Type = "VLESS"
		srv.Password = client.ID
	case model.VMESS:
		srv.Type = "VMess"
		srv.Password = client.ID
	case model.Trojan:
		srv.Type = "Trojan"
		srv.Password = client.Password
	default:
		// Shadowsocks and other protocols are left to the regular link
		// subscription; their Shadowrocket mapping differs per cipher.
		return ShadowrocketServer{}, false
	}

	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		logger.Warning("SubShadowrocketService - failed to parse Settings for inbound", inbound.Tag, ":", err)
	}
	if srv.Type == "VLESS" {
		if enc, ok := settings["encryption"].(string); ok && enc != "" && !strings.EqualFold(enc, "none") {
			srv.Method = enc
		}
	}

	if strings.Contains(client.Flow, "xtls-rprx-vision") {
		srv.XTLS = 2
	}

	switch security {
	case "reality":
		realitySetting, _ := stream["realitySettings"].(map[string]any)
		if realitySetting != nil {
			if names, ok := realitySetting["serverNames"].([]any); ok && len(names) > 0 {
				srv.Peer = fmt.Sprintf("%v", names[0])
			}
			if shortIds, ok := realitySetting["shortIds"].([]any); ok && len(shortIds) > 0 {
				srv.ShortID = fmt.Sprintf("%v", shortIds[0])
			}
			if inner, ok := realitySetting["settings"].(map[string]any); ok {
				if pk, ok := inner["publicKey"].(string); ok {
					srv.PublicKey = pk
				}
			}
			srv.TLSProfile = shadowrocketTLSProfile
		}
	case "tls":
		tlsSetting, _ := stream["tlsSettings"].(map[string]any)
		if tlsSetting != nil {
			if sni, ok := tlsSetting["serverName"].(string); ok {
				srv.Peer = sni
			}
			if fp, ok := tlsSetting["fingerprint"].(string); ok && fp != "" {
				srv.TLSProfile = shadowrocketTLSProfile
			}
		}
	}

	switch network {
	case "ws":
		srv.Obfs = "ws"
		if ws, ok := stream["wsSettings"].(map[string]any); ok {
			if path, ok := ws["path"].(string); ok {
				srv.Path = path
			}
			if host, ok := ws["host"].(string); ok {
				srv.ObfsParam = host
			}
		}
	case "httpupgrade":
		srv.Obfs = "ws"
		if up, ok := stream["httpupgradeSettings"].(map[string]any); ok {
			if path, ok := up["path"].(string); ok {
				srv.Path = path
			}
			if host, ok := up["host"].(string); ok {
				srv.ObfsParam = host
			}
		}
	case "grpc":
		srv.Obfs = "grpc"
		if grpc, ok := stream["grpcSettings"].(map[string]any); ok {
			if name, ok := grpc["serviceName"].(string); ok {
				srv.ObfsParam = name
			}
		}
	}

	return srv, true
}

// aggregateClientTraffics merges per-client traffic counters the same way the
// link and Clash subscriptions do.
func aggregateClientTraffics(list []xray.ClientTraffic) xray.ClientTraffic {
	var traffic xray.ClientTraffic
	for index, ct := range list {
		if index == 0 {
			traffic.Up = ct.Up
			traffic.Down = ct.Down
			traffic.Total = ct.Total
			if ct.ExpiryTime > 0 {
				traffic.ExpiryTime = ct.ExpiryTime
			}
			continue
		}
		traffic.Up += ct.Up
		traffic.Down += ct.Down
		if traffic.Total == 0 || ct.Total == 0 {
			traffic.Total = 0
		} else {
			traffic.Total += ct.Total
		}
		if ct.ExpiryTime != traffic.ExpiryTime {
			traffic.ExpiryTime = 0
		}
	}
	return traffic
}
