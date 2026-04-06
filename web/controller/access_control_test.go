package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/global"
	"github.com/mhsanaei/3x-ui/v2/web/session"
	"github.com/mhsanaei/3x-ui/v2/xray"
	"github.com/robfig/cron/v3"
)

type testWebServer struct {
	cron *cron.Cron
}

func (s *testWebServer) GetCron() *cron.Cron     { return s.cron }
func (s *testWebServer) GetCtx() context.Context { return context.Background() }
func (s *testWebServer) GetWSHub() any           { return nil }

func setupControllerTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DEBUG", "")
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	dbPath := filepath.Join(tmpDir, "controller-test.db")
	if err := database.InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		database.CloseDB()
	})
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	store := cookie.NewStore([]byte("test-secret"))
	r.Use(sessions.Sessions("3x-ui", store))
	r.Use(func(c *gin.Context) {
		c.Set("base_path", "/")
		role := c.GetHeader("X-Test-Role")
		if role == "" {
			return
		}
		user := &model.User{
			Id:       1,
			Username: c.GetHeader("X-Test-Username"),
			Role:     role,
		}
		if user.Username == "" {
			user.Username = "tester@example.com"
		}
		session.SetLoginUser(c, user)
	})
	return r
}

func TestXUIController_SettingsPageRequiresAdmin(t *testing.T) {
	r := newTestRouter(t)
	NewXUIController(r.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/panel/settings", nil)
	req.Header.Set("X-Test-Role", "user")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}
	if got := w.Header().Get("Location"); got != "/panel/user" {
		t.Fatalf("expected redirect to /panel/user, got %q", got)
	}
}

func TestAPIController_AdminEndpointsRequireAdmin(t *testing.T) {
	global.SetWebServer(&testWebServer{cron: cron.New()})

	r := newTestRouter(t)
	NewAPIController(r.Group("/"))

	for _, path := range []string{
		"/panel/api/inbounds/list",
		"/panel/api/server/status",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Test-Role", "user")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: expected %d, got %d", path, http.StatusForbidden, w.Code)
		}
	}
}

func TestAPIController_UserInfoRemainsAvailableToLoggedInUser(t *testing.T) {
	setupControllerTestDB(t)
	global.SetWebServer(&testWebServer{cron: cron.New()})

	inboundSettings, err := json.Marshal(map[string]any{
		"clients": []map[string]any{
			{"id": "client-1", "email": "tester@example.com", "enable": true, "subId": "sub-1"},
		},
	})
	if err != nil {
		t.Fatalf("marshal inbound settings failed: %v", err)
	}

	inbound := &model.Inbound{
		UserId:   1,
		Port:     12001,
		Protocol: model.VLESS,
		Tag:      "controller-user-info",
		Settings: string(inboundSettings),
	}
	if err := database.GetDB().Create(inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}
	if err := database.GetDB().Create(&xray.ClientTraffic{
		InboundId: inbound.Id,
		Email:     "tester@example.com",
		Enable:    true,
	}).Error; err != nil {
		t.Fatalf("create client traffic failed: %v", err)
	}

	r := newTestRouter(t)
	NewAPIController(r.Group("/"))

	req := httptest.NewRequest(http.MethodGet, "/panel/api/inbounds/userInfo", nil)
	req.Header.Set("X-Test-Role", "user")
	req.Header.Set("X-Test-Username", "tester@example.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, w.Code)
	}
}
