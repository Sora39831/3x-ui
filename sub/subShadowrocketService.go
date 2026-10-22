package sub

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// shadowrocketTLSProfile is the uTLS fingerprint profile handed to Shadowrocket.
// Chrome140 is the first profile verified to work against Xray-core v26.10+
// REALITY servers that require an X25519MLKEM768 key share in ClientHello.
const shadowrocketTLSProfile = "Chrome140"

// ShadowrocketServer is a single node in a Shadowrocket JSON subscription.
// The field set mirrors the object Shadowrocket itself exports, so that a
// subscription generated here can be imported and edited identically.
type ShadowrocketServer struct {
	Add        string `json:"add"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	Password   string `json:"password"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	TLS        bool   `json:"tls"`
	UDP        int    `json:"udp"`
	Obfs       string `json:"obfs"`
	Flag       string `json:"flag"`
	Peer       string `json:"peer,omitempty"`
	Method     string `json:"method,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	ShortID    string `json:"shortId,omitempty"`
	XTLS       int    `json:"xtls,omitempty"`
	TLSProfile string `json:"tlsProfile,omitempty"`
	Path       string `json:"path,omitempty"`
	ObfsParam  string `json:"obfsParam,omitempty"`
	Proto      string `json:"proto,omitempty"`
	Chain      string `json:"chain,omitempty"`
	Plugin     string `json:"plugin,omitempty"`
	Cert       string `json:"cert,omitempty"`
	Reserved   []any  `json:"reserved"`
}

type shadowrocketConfig struct {
	Servers []ShadowrocketServer `json:"servers"`
}

// IsShadowrocketRequest reports whether the request should receive the
// Shadowrocket JSON subscription instead of the standard link list.
// Detection is user-agent based, with ?format=sr / ?sr=1 as a manual override
// for testing from a browser or curl.
func IsShadowrocketRequest(c *gin.Context) bool {
	ua := strings.ToLower(c.GetHeader("User-Agent"))
	if strings.Contains(ua, "shadowrocket") {
		return true
	}
	if c.Query("sr") == "1" {
		return true
	}
	switch strings.ToLower(c.Query("format")) {
	case "sr", "shadowrocket":
		return true
	}
	return false
}

// BuildShadowrocketConfig renders the link list returned by GetSubs into a
// Shadowrocket JSON subscription ({"servers": [...]}).
// Links are re-parsed rather than re-generated so every protocol-specific
// detail (REALITY keys, ML-KEM encryption string, flow, transport options)
// stays in sync with the regular subscription output.
func (s *SubService) BuildShadowrocketConfig(subs []string) (string, error) {
	servers := make([]ShadowrocketServer, 0, len(subs))
	seen := make(map[string]struct{}, len(subs))

	for _, raw := range subs {
		for _, link := range strings.Split(strings.TrimSpace(raw), "\n") {
			link = strings.TrimSpace(link)
			if link == "" {
				continue
			}
			server, ok := parseShadowrocketLink(link)
			if !ok {
				continue
			}
			key := server.Add + "|" + server.Port + "|" + server.Password
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			servers = append(servers, server)
		}
	}

	payload, err := json.Marshal(shadowrocketConfig{Servers: servers})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func parseShadowrocketLink(link string) (ShadowrocketServer, bool) {
	switch {
	case strings.HasPrefix(link, "vless://"):
		return parseShadowrocketVless(link)
	case strings.HasPrefix(link, "trojan://"):
		return parseShadowrocketTrojan(link)
	case strings.HasPrefix(link, "vmess://"):
		return parseShadowrocketVmess(link)
	}
	return ShadowrocketServer{}, false
}

func parseShadowrocketVless(link string) (ShadowrocketServer, bool) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" || u.User == nil {
		return ShadowrocketServer{}, false
	}
	q := u.Query()

	srv := ShadowrocketServer{
		Add:      u.Hostname(),
		Host:     u.Hostname(),
		Port:     u.Port(),
		Password: u.User.Username(),
		Type:     "VLESS",
		Title:    u.Fragment,
		TLS:      false,
		UDP:      1,
		Obfs:     "none",
		Flag:     flagFromTitle(u.Fragment),
		Proto:    "auto",
		Reserved: []any{},
	}

	switch q.Get("security") {
	case "tls":
		srv.TLS = true
		srv.Peer = q.Get("sni")
		if q.Get("fp") != "" {
			srv.TLSProfile = shadowrocketTLSProfile
		}
	case "reality":
		srv.TLS = true
		srv.Peer = q.Get("sni")
		srv.PublicKey = q.Get("pbk")
		srv.ShortID = q.Get("sid")
		srv.TLSProfile = shadowrocketTLSProfile
	}

	if enc := q.Get("encryption"); enc != "" && !strings.EqualFold(enc, "none") {
		srv.Method = enc
	}
	if strings.Contains(q.Get("flow"), "xtls-rprx-vision") {
		srv.XTLS = 2
	}

	switch q.Get("type") {
	case "ws", "httpupgrade":
		srv.Obfs = "ws"
		srv.Path = q.Get("path")
		if host := q.Get("host"); host != "" {
			srv.ObfsParam = host
		}
	case "grpc":
		srv.Obfs = "grpc"
		srv.ObfsParam = q.Get("serviceName")
	case "h2", "http":
		srv.Obfs = "h2"
		srv.Path = q.Get("path")
		if host := q.Get("host"); host != "" {
			srv.ObfsParam = host
		}
	}

	return srv, true
}

func parseShadowrocketTrojan(link string) (ShadowrocketServer, bool) {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" || u.User == nil {
		return ShadowrocketServer{}, false
	}
	q := u.Query()

	srv := ShadowrocketServer{
		Add:        u.Hostname(),
		Host:       u.Hostname(),
		Port:       u.Port(),
		Password:   u.User.Username(),
		Type:       "Trojan",
		Title:      u.Fragment,
		TLS:        true,
		UDP:        1,
		Obfs:       "none",
		Flag:       flagFromTitle(u.Fragment),
		Peer:       q.Get("sni"),
		TLSProfile: shadowrocketTLSProfile,
		Proto:      "auto",
		Reserved:   []any{},
	}

	switch q.Get("type") {
	case "ws":
		srv.Obfs = "ws"
		srv.Path = q.Get("path")
		if host := q.Get("host"); host != "" {
			srv.ObfsParam = host
		}
	case "grpc":
		srv.Obfs = "grpc"
		srv.ObfsParam = q.Get("serviceName")
	}

	return srv, true
}

// parseShadowrocketVmess decodes the base64 JSON used by vmess:// links.
func parseShadowrocketVmess(link string) (ShadowrocketServer, bool) {
	raw := strings.TrimPrefix(link, "vmess://")
	if idx := strings.IndexAny(raw, "#"); idx >= 0 {
		raw = raw[:idx]
	}
	data, err := decodeVmessBase64(raw)
	if err != nil {
		return ShadowrocketServer{}, false
	}

	var v struct {
		Add  string `json:"add"`
		Port any    `json:"port"`
		ID   string `json:"id"`
		Net  string `json:"net"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
		Host string `json:"host"`
		Path string `json:"path"`
		PS   string `json:"ps"`
		Scy  string `json:"scy"`
	}
	if err := json.Unmarshal(data, &v); err != nil || v.Add == "" || v.ID == "" {
		return ShadowrocketServer{}, false
	}

	srv := ShadowrocketServer{
		Add:      v.Add,
		Host:     v.Add,
		Port:     portToString(v.Port),
		Password: v.ID,
		Type:     "VMess",
		Title:    v.PS,
		TLS:      strings.EqualFold(v.TLS, "tls"),
		UDP:      1,
		Obfs:     "none",
		Flag:     flagFromTitle(v.PS),
		Proto:    "auto",
		Reserved: []any{},
	}
	if srv.TLS {
		srv.Peer = v.SNI
		srv.TLSProfile = shadowrocketTLSProfile
	}
	switch v.Net {
	case "ws":
		srv.Obfs = "ws"
		srv.Path = v.Path
		srv.ObfsParam = v.Host
	case "grpc":
		srv.Obfs = "grpc"
		srv.ObfsParam = v.Path
	}
	if v.Scy != "" && v.Scy != "auto" {
		srv.Method = v.Scy
	}
	return srv, true
}

func decodeVmessBase64(s string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	if data, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return data, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

func portToString(port any) string {
	switch p := port.(type) {
	case string:
		return p
	case float64:
		return strconv.Itoa(int(p))
	case int:
		return strconv.Itoa(p)
	default:
		return ""
	}
}

// flagFromTitle extracts a two-letter country flag from a title that starts
// with (or contains) a regional-indicator emoji pair, e.g. "🇯🇵_Softbank" -> "JP".
func flagFromTitle(title string) string {
	runes := []rune(title)
	for i := 0; i+1 < len(runes); i++ {
		a, b := runes[i], runes[i+1]
		if isRegionalIndicator(a) && isRegionalIndicator(b) {
			return string(rune('A'+a-0x1F1E6)) + string(rune('A'+b-0x1F1E6))
		}
	}
	return ""
}

func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}
