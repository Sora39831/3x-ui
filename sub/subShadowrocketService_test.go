package sub

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

const testShadowrocketVlessLink = "vless://34b56216-1d5c-4606-a418-dbf2caa6fe60@1.2.3.4:443" +
	"?type=tcp&security=reality&encryption=mlkem768x25519plus.native.0rtt.ABC" +
	"&flow=xtls-rprx-vision&sni=www.amazon.com&fp=chrome" +
	"&pbk=Iq4VerqAVOxwyOmrpe21yQg44kENr8xZZsjWFWyENxk&sid=5bb395" +
	"&support-x25519mlkem768=true&spx=%2Fabc" +
	"#%F0%9F%87%AF%F0%9F%87%B5_Softbank"

func TestIsShadowrocketRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		ua     string
		target string
		want   bool
	}{
		{name: "shadowrocket ua", ua: "Shadowrocket/2.2.65 (iPhone; iOS 18.0)", want: true},
		{name: "lowercase ua", ua: "shadowrocket", want: true},
		{name: "plain curl", ua: "curl/8.7.1", target: "/sub/abc", want: false},
		{name: "format override", ua: "curl/8.7.1", target: "/sub/abc?format=sr", want: true},
		{name: "sr override", ua: "curl/8.7.1", target: "/sub/abc?sr=1", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := tc.target
			if target == "" {
				target = "/sub/abc"
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("GET", target, nil)
			c.Request.Header.Set("User-Agent", tc.ua)

			if got := IsShadowrocketRequest(c); got != tc.want {
				t.Fatalf("IsShadowrocketRequest() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildShadowrocketConfigVlessReality(t *testing.T) {
	service := NewSubService(false, "ieo")

	out, err := service.BuildShadowrocketConfig([]string{testShadowrocketVlessLink})
	if err != nil {
		t.Fatalf("BuildShadowrocketConfig() error = %v", err)
	}

	var config struct {
		Servers []ShadowrocketServer `json:"servers"`
	}
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("unmarshal output error = %v, output = %s", err, out)
	}
	if len(config.Servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(config.Servers))
	}

	srv := config.Servers[0]
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"add", srv.Add, "1.2.3.4"},
		{"host", srv.Host, "1.2.3.4"},
		{"port", srv.Port, "443"},
		{"password", srv.Password, "34b56216-1d5c-4606-a418-dbf2caa6fe60"},
		{"type", srv.Type, "VLESS"},
		{"tls", srv.TLS, true},
		{"udp", srv.UDP, 1},
		{"obfs", srv.Obfs, "none"},
		{"peer", srv.Peer, "www.amazon.com"},
		{"publicKey", srv.PublicKey, "Iq4VerqAVOxwyOmrpe21yQg44kENr8xZZsjWFWyENxk"},
		{"shortId", srv.ShortID, "5bb395"},
		{"xtls", srv.XTLS, 2},
		{"tlsProfile", srv.TLSProfile, shadowrocketTLSProfile},
		{"method", srv.Method, "mlkem768x25519plus.native.0rtt.ABC"},
		{"flag", srv.Flag, "JP"},
		{"title", srv.Title, "🇯🇵_Softbank"},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %#v, want %#v", check.name, check.got, check.want)
		}
	}
}

func TestBuildShadowrocketConfigDeduplicates(t *testing.T) {
	service := NewSubService(false, "ieo")

	out, err := service.BuildShadowrocketConfig([]string{
		testShadowrocketVlessLink,
		testShadowrocketVlessLink + "\n" + testShadowrocketVlessLink,
	})
	if err != nil {
		t.Fatalf("BuildShadowrocketConfig() error = %v", err)
	}

	var config struct {
		Servers []ShadowrocketServer `json:"servers"`
	}
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		t.Fatalf("unmarshal output error = %v", err)
	}
	if len(config.Servers) != 1 {
		t.Fatalf("got %d servers, want 1 after dedupe", len(config.Servers))
	}
}

func TestBuildShadowrocketConfigSkipsUnsupportedLinks(t *testing.T) {
	service := NewSubService(false, "ieo")

	out, err := service.BuildShadowrocketConfig([]string{"", "ss://not-supported", "garbage"})
	if err != nil {
		t.Fatalf("BuildShadowrocketConfig() error = %v", err)
	}
	if out != `{"servers":[]}` {
		t.Fatalf("unexpected output: %s", out)
	}
}
