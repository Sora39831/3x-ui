package sub

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// SubClashService handles Clash YAML subscription generation.
type SubClashService struct {
	template string
	inboundService service.InboundService
	SubService     *SubService
}

// NewSubClashService creates a new Clash subscription service with the given template.
func NewSubClashService(template string, subService *SubService) *SubClashService {
	return &SubClashService{
		template:   template,
		SubService: subService,
	}
}

// GetClash generates a Clash YAML configuration for the given subscription ID.
func (s *SubClashService) GetClash(subId string) (string, string, error) {
	if s.template == "" {
		return "", "", fmt.Errorf("clash template is empty")
	}

	inbounds, err := s.SubService.getInboundsBySubId(subId)
	if err != nil || len(inbounds) == 0 {
		return "", "", err
	}

	var header string
	var traffic xray.ClientTraffic
	var clientTraffics []xray.ClientTraffic
	var proxies []string

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
			if client.Enable && client.SubID == subId {
				clientTraffics = append(clientTraffics, s.SubService.getClientTraffics(inbound.ClientStats, client.Email))
				newProxies := s.getProxy(inbound, client)
				proxies = append(proxies, newProxies...)
			}
		}
	}

	if len(proxies) == 0 {
		return "", "", nil
	}

	// Aggregate traffic stats
	for index, clientTraffic := range clientTraffics {
		if index == 0 {
			traffic.Up = clientTraffic.Up
			traffic.Down = clientTraffic.Down
			traffic.Total = clientTraffic.Total
			if clientTraffic.ExpiryTime > 0 {
				traffic.ExpiryTime = clientTraffic.ExpiryTime
			}
		} else {
			traffic.Up += clientTraffic.Up
			traffic.Down += clientTraffic.Down
			if traffic.Total == 0 || clientTraffic.Total == 0 {
				traffic.Total = 0
			} else {
				traffic.Total += clientTraffic.Total
			}
			if clientTraffic.ExpiryTime != traffic.ExpiryTime {
				traffic.ExpiryTime = 0
			}
		}
	}

	// Build proxies YAML block
	proxiesYaml := ""
	for _, p := range proxies {
		proxiesYaml += "  - " + p + "\n"
	}

	// Inject proxies into template by replacing "proxies: []"
	result := strings.Replace(s.template, "proxies: []", "proxies:\n"+proxiesYaml, 1)

	header = fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d", traffic.Up, traffic.Down, traffic.Total, traffic.ExpiryTime/1000)
	return result, header, nil
}

// getProxy generates Clash proxy entries for a client.
func (s *SubClashService) getProxy(inbound *model.Inbound, client model.Client) []string {
	var proxies []string
	var stream map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
		logger.Warning("SubClashService - failed to parse StreamSettings for inbound", inbound.Tag, ":", err)
	}

	// Resolve address
	var address string
	if inbound.Listen == "" || inbound.Listen == "0.0.0.0" || inbound.Listen == "::" || inbound.Listen == "::0" {
		address = s.SubService.address
	} else {
		address = inbound.Listen
	}

	// Get remark
	remark := s.SubService.genRemark(inbound, client.Email, "")

	// Parse stream settings
	network, _ := stream["network"].(string)
	security, _ := stream["security"].(string)

	// Handle external proxies
	externalProxies, ok := stream["externalProxy"].([]any)
	if !ok || len(externalProxies) == 0 {
		externalProxies = []any{
			map[string]any{
				"forceTls": "same",
			},
		}
	}

	for _, ep := range externalProxies {
		externalProxy, _ := ep.(map[string]any)
		destAddress := address
		destPort := inbound.Port

		if dest, ok := externalProxy["dest"].(string); ok && dest != "" {
			destAddress = dest
		}
		if port, ok := externalProxy["port"].(float64); ok && port > 0 {
			destPort = int(port)
		}

		forceTls, _ := externalProxy["forceTls"].(string)
		tlsEnabled := false
		switch forceTls {
		case "tls":
			tlsEnabled = true
		case "none":
			tlsEnabled = false
		default: // "same"
			tlsEnabled = security == "tls" || security == "reality"
		}

		remarkExtra := remark
		if customRemark, ok := externalProxy["remark"].(string); ok && customRemark != "" {
			remarkExtra = customRemark
		}

		proxy := s.buildProxyEntry(inbound, client, destAddress, destPort, network, security, tlsEnabled, remarkExtra, stream)
		proxies = append(proxies, proxy)
	}

	return proxies
}

// buildProxyEntry builds a single Clash proxy entry as an inline YAML map string.
func (s *SubClashService) buildProxyEntry(inbound *model.Inbound, client model.Client, address string, port int, network, security string, tlsEnabled bool, remark string, stream map[string]any) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("name: %q", remark))

	// Protocol-specific fields
	switch inbound.Protocol {
	case model.VMESS:
		parts = append(parts, "type: vmess")
		parts = append(parts, fmt.Sprintf("server: %q", address))
		parts = append(parts, fmt.Sprintf("port: %d", port))
		parts = append(parts, fmt.Sprintf("uuid: %q", client.ID))
		parts = append(parts, "alterId: 0")
		parts = append(parts, "cipher: auto")

	case model.VLESS:
		parts = append(parts, "type: vless")
		parts = append(parts, fmt.Sprintf("server: %q", address))
		parts = append(parts, fmt.Sprintf("port: %d", port))
		parts = append(parts, fmt.Sprintf("uuid: %q", client.ID))
		if client.Flow != "" {
			parts = append(parts, fmt.Sprintf("flow: %q", client.Flow))
		}

	case model.Trojan:
		parts = append(parts, "type: trojan")
		parts = append(parts, fmt.Sprintf("server: %q", address))
		parts = append(parts, fmt.Sprintf("port: %d", port))
		parts = append(parts, fmt.Sprintf("password: %q", client.Password))

	case model.Shadowsocks:
		parts = append(parts, "type: ss")
		parts = append(parts, fmt.Sprintf("server: %q", address))
		parts = append(parts, fmt.Sprintf("port: %d", port))
		cipher, password := s.parseShadowsocksSettings(client)
		parts = append(parts, fmt.Sprintf("cipher: %q", cipher))
		parts = append(parts, fmt.Sprintf("password: %q", password))
		parts = append(parts, "udp: true")
		return strings.Join(parts, "\n  ")
	}

	// TLS settings
	if tlsEnabled {
		parts = append(parts, "tls: true")
		if security == "reality" {
			realitySetting, _ := stream["realitySettings"].(map[string]any)
			if publicKey, ok := realitySetting["publicKey"].(string); ok && publicKey != "" {
				realityOpts := fmt.Sprintf("reality-opts:\n    public-key: %q", publicKey)
				if shortId, ok := realitySetting["shortId"].(string); ok && shortId != "" {
					realityOpts += fmt.Sprintf("\n    short-id: %q", shortId)
				}
				parts = append(parts, realityOpts)
			}
			// Reality server names
			serverNames, _ := realitySetting["serverNames"].([]any)
			if len(serverNames) > 0 {
				sni := fmt.Sprintf("%v", serverNames[0])
				parts = append(parts, fmt.Sprintf("sni: %q", sni))
			}
		} else {
			// TLS settings
			tlsSetting, _ := stream["tlsSettings"].(map[string]any)
			if serverName, ok := tlsSetting["serverName"].(string); ok && serverName != "" {
				parts = append(parts, fmt.Sprintf("sni: %q", serverName))
			}
			if alpn, ok := tlsSetting["alpn"].([]any); ok && len(alpn) > 0 {
				alpnStrs := make([]string, len(alpn))
				for i, a := range alpn {
					alpnStrs[i] = fmt.Sprintf("%v", a)
				}
				parts = append(parts, fmt.Sprintf("alpn: [%s]", strings.Join(alpnStrs, ", ")))
			}
		}
		// Fingerprint
		if fp, ok := stream["fingerprint"].(string); ok && fp != "" {
			parts = append(parts, fmt.Sprintf("client-fingerprint: %q", fp))
		}
	} else {
		parts = append(parts, "tls: false")
	}

	parts = append(parts, "udp: true")

	// Network-specific settings
	switch network {
	case "ws":
		ws, _ := stream["wsSettings"].(map[string]any)
		if path, ok := ws["path"].(string); ok && path != "" {
			wsOpts := fmt.Sprintf("ws-opts:\n    path: %q", path)
			if host, ok := ws["host"].(string); ok && host != "" {
				wsOpts += fmt.Sprintf("\n    headers:\n      Host: %q", host)
			} else {
				headers, _ := ws["headers"].(map[string]any)
				if h, ok := headers["Host"].(string); ok && h != "" {
					wsOpts += fmt.Sprintf("\n    headers:\n      Host: %q", h)
				}
			}
			parts = append(parts, wsOpts)
		}

	case "grpc":
		grpc, _ := stream["grpcSettings"].(map[string]any)
		if serviceName, ok := grpc["serviceName"].(string); ok && serviceName != "" {
			parts = append(parts, fmt.Sprintf("grpc-opts:\n    grpc-service-name: %q", serviceName))
		}

	case "h2":
		h2, _ := stream["h2Settings"].(map[string]any)
		if path, ok := h2["path"].(string); ok && path != "" {
			h2Opts := fmt.Sprintf("h2-opts:\n    path: %q", path)
			if host, ok := h2["host"].([]any); ok && len(host) > 0 {
				hostStrs := make([]string, len(host))
				for i, h := range host {
					hostStrs[i] = fmt.Sprintf("%q", fmt.Sprintf("%v", h))
				}
				h2Opts += fmt.Sprintf("\n    host: [%s]", strings.Join(hostStrs, ", "))
			}
			parts = append(parts, h2Opts)
		}

	case "tcp":
		tcp, _ := stream["tcpSettings"].(map[string]any)
		header, _ := tcp["header"].(map[string]any)
		if typeStr, ok := header["type"].(string); ok && typeStr == "http" {
			request, _ := header["request"].(map[string]any)
			httpOpts := "http-opts:"
			if path, ok := request["path"].([]any); ok && len(path) > 0 {
				httpOpts += fmt.Sprintf("\n    path:\n      - %q", fmt.Sprintf("%v", path[0]))
			}
			if headers, ok := request["headers"].(map[string]any); ok && len(headers) > 0 {
				httpOpts += "\n    headers:"
				for k, v := range headers {
					if vals, ok := v.([]any); ok && len(vals) > 0 {
						httpOpts += fmt.Sprintf("\n      %s:\n        - %q", k, fmt.Sprintf("%v", vals[0]))
					}
				}
			}
			parts = append(parts, httpOpts)
		}

	case "httpupgrade":
		hu, _ := stream["httpupgradeSettings"].(map[string]any)
		if path, ok := hu["path"].(string); ok && path != "" {
			huOpts := fmt.Sprintf("httpupgrade-opts:\n    path: %q", path)
			if host, ok := hu["host"].(string); ok && host != "" {
				huOpts += fmt.Sprintf("\n    host: %q", host)
			} else {
				headers, _ := hu["headers"].(map[string]any)
				if h, ok := headers["Host"].(string); ok && h != "" {
					huOpts += fmt.Sprintf("\n    host: %q", h)
				}
			}
			parts = append(parts, huOpts)
		}
	}

	return strings.Join(parts, "\n  ")
}

// parseShadowsocksSettings extracts cipher and password from shadowsocks client settings.
func (s *SubClashService) parseShadowsocksSettings(client model.Client) (string, string) {
	// Default cipher
	cipher := "aes-128-gcm"
	password := client.Password

	// Try to parse the method from the client's settings
	// Shadowsocks protocol stores method in client.ID for some configurations
	if client.Security != "" {
		cipher = client.Security
	}

	return cipher, password
}
