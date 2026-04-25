package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XUI_DEBUG", "")
	t.Setenv("XUI_DB_FOLDER", tmpDir)
	dbPath := filepath.Join(tmpDir, "test.db")
	if err := database.InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	t.Cleanup(func() {
		database.CloseDB()
	})
}

func TestGetFirstUser(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	user, err := svc.GetFirstUser()
	if err != nil {
		t.Fatalf("GetFirstUser error: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", user.Username)
	}
}

func TestCheckUser_ValidCredentials(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	user, err := svc.CheckUser("admin", "admin", "")
	if err != nil {
		t.Fatalf("CheckUser error: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("expected username 'admin', got %q", user.Username)
	}
}

func TestCheckUser_WrongPassword(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	_, err := svc.CheckUser("admin", "wrongpassword", "")
	if err == nil {
		t.Error("CheckUser should fail with wrong password")
	}
}

func TestCheckUser_NonExistentUser(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	_, err := svc.CheckUser("nonexistent", "password", "")
	if err == nil {
		t.Error("CheckUser should fail for non-existent user")
	}
}

func TestUpdateFirstUser(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	err := svc.UpdateFirstUser("newadmin", "newpassword123")
	if err != nil {
		t.Fatalf("UpdateFirstUser error: %v", err)
	}

	// Verify login with new credentials
	user, err := svc.CheckUser("newadmin", "newpassword123", "")
	if err != nil {
		t.Fatalf("CheckUser with new credentials error: %v", err)
	}
	if user.Username != "newadmin" {
		t.Errorf("expected username 'newadmin', got %q", user.Username)
	}
}

func TestUpdateFirstUser_EmptyUsername(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	err := svc.UpdateFirstUser("", "password")
	if err == nil {
		t.Error("UpdateFirstUser should fail with empty username")
	}
}

func TestUpdateFirstUser_EmptyPassword(t *testing.T) {
	setupTestDB(t)

	svc := &UserService{}
	err := svc.UpdateFirstUser("admin", "")
	if err == nil {
		t.Error("UpdateFirstUser should fail with empty password")
	}
}

func TestUpdateFirstUser_CreateWhenNone(t *testing.T) {
	// Use a fresh temp dir so no users table data exists
	tmpDir := t.TempDir()
	os.Setenv("XUI_DEBUG", "")
	os.Setenv("XUI_DB_FOLDER", tmpDir)
	defer func() {
		os.Unsetenv("XUI_DEBUG")
		os.Unsetenv("XUI_DB_FOLDER")
	}()

	dbPath := filepath.Join(tmpDir, "empty.db")
	if err := database.InitDBWithPath(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.CloseDB()

	// Delete all users to simulate empty table
	database.GetDB().Where("1 = 1").Delete(&model.User{})

	svc := &UserService{}
	err := svc.UpdateFirstUser("firstadmin", "firstpass")
	if err != nil {
		t.Fatalf("UpdateFirstUser should create user when table is empty: %v", err)
	}

	user, err := svc.GetFirstUser()
	if err != nil {
		t.Fatalf("GetFirstUser error: %v", err)
	}
	if user.Username != "firstadmin" {
		t.Errorf("expected username 'firstadmin', got %q", user.Username)
	}
	if !crypto.CheckPasswordHash(user.Password, "firstpass") {
		t.Error("password hash should match 'firstpass'")
	}
}

func TestDeleteUser_RemovesClientsFromAllInbounds(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	userSvc := &UserService{}
	inboundSvc := &InboundService{}

	hashedPassword, err := crypto.HashPasswordAsBcrypt("password123")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	managedUser := &model.User{
		Username: "managed_user",
		Password: hashedPassword,
		Role:     "user",
	}
	if err := db.Create(managedUser).Error; err != nil {
		t.Fatalf("create managed user failed: %v", err)
	}

	settings1Bytes, err := json.Marshal(map[string]any{
		"clients": []map[string]any{
			{"id": "client-1", "email": "managed_user", "enable": false},
			{"id": "client-2", "email": "keep_user", "enable": false},
		},
	})
	if err != nil {
		t.Fatalf("marshal inbound settings 1 failed: %v", err)
	}
	settings2Bytes, err := json.Marshal(map[string]any{
		"clients": []map[string]any{
			{"id": "client-3", "email": "managed_user", "enable": false},
			{"id": "client-4", "email": "another_user", "enable": false},
		},
	})
	if err != nil {
		t.Fatalf("marshal inbound settings 2 failed: %v", err)
	}

	inbound1 := &model.Inbound{
		UserId:   1,
		Port:     21001,
		Protocol: model.VLESS,
		Tag:      "user-delete-sync-1",
		Settings: string(settings1Bytes),
	}
	inbound2 := &model.Inbound{
		UserId:   1,
		Port:     21002,
		Protocol: model.VLESS,
		Tag:      "user-delete-sync-2",
		Settings: string(settings2Bytes),
	}
	if err := db.Create(inbound1).Error; err != nil {
		t.Fatalf("create inbound1 failed: %v", err)
	}
	if err := db.Create(inbound2).Error; err != nil {
		t.Fatalf("create inbound2 failed: %v", err)
	}

	if err := db.Create(&xray.ClientTraffic{InboundId: inbound1.Id, Email: "managed_user"}).Error; err != nil {
		t.Fatalf("create traffic for inbound1 failed: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound2.Id, Email: "managed_user"}).Error; err != nil {
		t.Fatalf("create traffic for inbound2 failed: %v", err)
	}
	if err := db.Create(&xray.ClientTraffic{InboundId: inbound1.Id, Email: "keep_user"}).Error; err != nil {
		t.Fatalf("create keep_user traffic failed: %v", err)
	}
	if err := db.Create(&model.InboundClientIps{ClientEmail: "managed_user", Ips: "[\"1.1.1.1\"]"}).Error; err != nil {
		t.Fatalf("create inbound client ips failed: %v", err)
	}

	if err := userSvc.DeleteUser(managedUser.Id, 1, inboundSvc); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	var usersCount int64
	if err := db.Model(&model.User{}).Where("id = ?", managedUser.Id).Count(&usersCount).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if usersCount != 0 {
		t.Fatalf("expected managed user to be deleted, remaining=%d", usersCount)
	}

	checkInboundHasNoManagedUser := func(inboundID int) {
		t.Helper()
		var inbound model.Inbound
		if err := db.First(&inbound, inboundID).Error; err != nil {
			t.Fatalf("load inbound %d failed: %v", inboundID, err)
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			t.Fatalf("unmarshal inbound settings failed: %v", err)
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			t.Fatalf("invalid clients format in inbound %d", inboundID)
		}
		for _, clientRaw := range clients {
			clientMap, ok := clientRaw.(map[string]any)
			if !ok {
				continue
			}
			email, _ := clientMap["email"].(string)
			if email == "managed_user" {
				t.Fatalf("managed_user still exists in inbound %d clients", inboundID)
			}
		}
	}
	checkInboundHasNoManagedUser(inbound1.Id)
	checkInboundHasNoManagedUser(inbound2.Id)

	var managedTrafficCount int64
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "managed_user").Count(&managedTrafficCount).Error; err != nil {
		t.Fatalf("count managed user traffic failed: %v", err)
	}
	if managedTrafficCount != 0 {
		t.Fatalf("expected managed_user traffic to be deleted, remaining=%d", managedTrafficCount)
	}

	var keepTrafficCount int64
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "keep_user").Count(&keepTrafficCount).Error; err != nil {
		t.Fatalf("count keep_user traffic failed: %v", err)
	}
	if keepTrafficCount != 1 {
		t.Fatalf("expected keep_user traffic to remain, got=%d", keepTrafficCount)
	}

	var ipsCount int64
	if err := db.Model(&model.InboundClientIps{}).Where("client_email = ?", "managed_user").Count(&ipsCount).Error; err != nil {
		t.Fatalf("count inbound client ips failed: %v", err)
	}
	if ipsCount != 0 {
		t.Fatalf("expected managed_user inbound_client_ips to be deleted, remaining=%d", ipsCount)
	}
}

func TestRegisterUser_AutoFillFlowForEligibleVlessInbound(t *testing.T) {
	setupTestDB(t)

	db := database.GetDB()
	userSvc := &UserService{}
	inboundSvc := &InboundService{}

	vlessSettingsBytes, err := json.Marshal(map[string]any{
		"clients": []map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal vless settings failed: %v", err)
	}

	vlessInbound := &model.Inbound{
		UserId:         1,
		Port:           21011,
		Protocol:       model.VLESS,
		Tag:            "register-flow-vless",
		Settings:       string(vlessSettingsBytes),
		StreamSettings: `{"network":"tcp","security":"tls"}`,
	}
	if err := db.Create(vlessInbound).Error; err != nil {
		t.Fatalf("create vless inbound failed: %v", err)
	}

	vmessSettingsBytes, err := json.Marshal(map[string]any{
		"clients": []map[string]any{},
	})
	if err != nil {
		t.Fatalf("marshal vmess settings failed: %v", err)
	}
	vmessInbound := &model.Inbound{
		UserId:         1,
		Port:           21012,
		Protocol:       model.VMESS,
		Tag:            "register-flow-vmess",
		Settings:       string(vmessSettingsBytes),
		StreamSettings: `{"network":"tcp","security":"tls"}`,
	}
	if err := db.Create(vmessInbound).Error; err != nil {
		t.Fatalf("create vmess inbound failed: %v", err)
	}

	if err := userSvc.RegisterUser("flow_user", "password123", inboundSvc); err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}

	assertClientFlow := func(inboundID int, expectedFlow string) {
		t.Helper()
		var inbound model.Inbound
		if err := db.First(&inbound, inboundID).Error; err != nil {
			t.Fatalf("load inbound %d failed: %v", inboundID, err)
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			t.Fatalf("unmarshal inbound settings failed: %v", err)
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			t.Fatalf("invalid clients format in inbound %d", inboundID)
		}
		for _, clientRaw := range clients {
			clientMap, ok := clientRaw.(map[string]any)
			if !ok {
				continue
			}
			email, _ := clientMap["email"].(string)
			if email != "flow_user" {
				continue
			}
			flow, _ := clientMap["flow"].(string)
			if flow != expectedFlow {
				t.Fatalf("unexpected flow for inbound %d: expected %q, got %q", inboundID, expectedFlow, flow)
			}
			return
		}
		t.Fatalf("flow_user not found in inbound %d", inboundID)
	}

	assertClientFlow(vlessInbound.Id, "xtls-rprx-vision")
	assertClientFlow(vmessInbound.Id, "")
}
