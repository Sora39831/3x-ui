package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRedirectMiddleware_XUIToPanel(t *testing.T) {
	r := gin.New()
	r.Use(RedirectMiddleware("/"))
	r.GET("/panel/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "panel")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/xui/settings", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/panel/settings" {
		t.Errorf("expected redirect to /panel/settings, got %q", loc)
	}
}

func TestRedirectMiddleware_XUIAPIToPanelAPI(t *testing.T) {
	r := gin.New()
	r.Use(RedirectMiddleware("/"))
	r.GET("/panel/api/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "api")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/xui/API/inbounds", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/panel/api/inbounds" {
		t.Errorf("expected redirect to /panel/api/inbounds, got %q", loc)
	}
}

func TestRedirectMiddleware_PanelAPICase(t *testing.T) {
	r := gin.New()
	r.Use(RedirectMiddleware("/"))
	r.GET("/panel/api/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "api")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panel/API/list", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/panel/api/list" {
		t.Errorf("expected redirect to /panel/api/list, got %q", loc)
	}
}

func TestRedirectMiddleware_NoRedirect(t *testing.T) {
	r := gin.New()
	r.Use(RedirectMiddleware("/"))
	r.GET("/panel/settings", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panel/settings", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRedirectMiddleware_WithBasePath(t *testing.T) {
	r := gin.New()
	r.Use(RedirectMiddleware("/base/"))
	r.GET("/base/panel/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/base/xui/settings", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/base/panel/settings" {
		t.Errorf("expected redirect to /base/panel/settings, got %q", loc)
	}
}

func TestDomainValidatorMiddleware_MatchingDomain(t *testing.T) {
	r := gin.New()
	r.Use(DomainValidatorMiddleware("example.com"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "example.com"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDomainValidatorMiddleware_MatchingDomainWithPort(t *testing.T) {
	r := gin.New()
	r.Use(DomainValidatorMiddleware("example.com"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "example.com:8443"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for matching domain with port, got %d", w.Code)
	}
}

func TestDomainValidatorMiddleware_NonMatchingDomain(t *testing.T) {
	r := gin.New()
	r.Use(DomainValidatorMiddleware("example.com"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "evil.com"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestDomainValidatorMiddleware_Subdomain(t *testing.T) {
	r := gin.New()
	r.Use(DomainValidatorMiddleware("example.com"))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Host = "sub.example.com"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for subdomain, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_FirstRequest(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(5, time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for first request, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_WithinLimit(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(3, time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for i := range 3 {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitMiddleware_ExceedsLimit(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(2, time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// First 2 should pass
	for range 2 {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.3:12345"
		r.ServeHTTP(w, req)
	}

	// 3rd should be rate limited
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.3:12345"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_CFConnectingIP(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(2, time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for range 2 {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("CF-Connecting-IP", "10.0.0.1")
		r.ServeHTTP(w, req)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "10.0.0.1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 with CF-Connecting-IP, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_RemoteAddr(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(2, time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	for range 2 {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.2:12345"
		r.ServeHTTP(w, req)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 with RemoteAddr, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_DifferentIPsIndependent(t *testing.T) {
	r := gin.New()
	r.Use(RateLimitMiddleware(1, time.Minute))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Exhaust limit for IP 1
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "10.0.0.10")
	r.ServeHTTP(w, req)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "10.0.0.10")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("IP 1 second request should be 429, got %d", w.Code)
	}

	// IP 2 should still be allowed
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/test", nil)
	req.Header.Set("CF-Connecting-IP", "10.0.0.20")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("different IP should get 200, got %d", w.Code)
	}
}
