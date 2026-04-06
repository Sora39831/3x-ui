package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

func mustMarshalInboundSettings(t *testing.T, clients ...model.Client) string {
	t.Helper()

	settings := map[string]any{
		"clients": clients,
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal inbound settings failed: %v", err)
	}
	return string(data)
}

func mustCreateInboundWithClients(t *testing.T, svc *InboundService, inbound model.Inbound, clients ...model.Client) *model.Inbound {
	t.Helper()

	inbound.Settings = mustMarshalInboundSettings(t, clients...)
	if err := database.GetDB().Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}

	for i := range clients {
		if clients[i].Email == "" {
			continue
		}
		if err := svc.AddClientStat(database.GetDB(), inbound.Id, &clients[i]); err != nil {
			t.Fatalf("create client traffic failed: %v", err)
		}
	}

	return &inbound
}

func countClientTraffic(t *testing.T, inboundID int, email string) int64 {
	t.Helper()

	var count int64
	query := database.GetDB().Model(&xray.ClientTraffic{}).Where("email = ?", email)
	if inboundID > 0 {
		query = query.Where("inbound_id = ?", inboundID)
	}
	if err := query.Count(&count).Error; err != nil {
		t.Fatalf("count client traffic failed: %v", err)
	}
	return count
}

func TestDelInboundClientByEmail_ScopedToInbound(t *testing.T) {
	setupTestDB(t)

	svc := &InboundService{}
	duplicateEmail := "shared@example.com"

	inbound1 := mustCreateInboundWithClients(t, svc, model.Inbound{
		UserId:   1,
		Port:     10001,
		Protocol: model.VLESS,
		Tag:      "inbound-test-1",
	}, model.Client{
		ID:     "client-1",
		Email:  duplicateEmail,
		Enable: false,
	}, model.Client{
		ID:     "client-2",
		Email:  "unique-1@example.com",
		Enable: false,
	})

	inbound2 := mustCreateInboundWithClients(t, svc, model.Inbound{
		UserId:   1,
		Port:     10002,
		Protocol: model.VLESS,
		Tag:      "inbound-test-2",
	}, model.Client{
		ID:     "client-3",
		Email:  duplicateEmail,
		Enable: false,
	}, model.Client{
		ID:     "client-4",
		Email:  "unique-2@example.com",
		Enable: false,
	})

	if got := countClientTraffic(t, 0, duplicateEmail); got != 2 {
		t.Fatalf("expected 2 traffic rows before deletion, got %d", got)
	}

	if _, err := svc.DelInboundClientByEmail(inbound1.Id, duplicateEmail); err != nil {
		t.Fatalf("first delete failed: %v", err)
	}
	if got := countClientTraffic(t, inbound1.Id, duplicateEmail); got != 0 {
		t.Fatalf("expected inbound1 traffic to be deleted, got %d", got)
	}
	if got := countClientTraffic(t, inbound2.Id, duplicateEmail); got != 1 {
		t.Fatalf("expected inbound2 traffic to remain, got %d", got)
	}

	if _, err := svc.DelInboundClientByEmail(inbound2.Id, duplicateEmail); err != nil {
		t.Fatalf("second delete failed: %v", err)
	}
	if got := countClientTraffic(t, 0, duplicateEmail); got != 0 {
		t.Fatalf("expected all duplicate-email traffics to be deleted, got %d", got)
	}
}

func TestUpdateInboundClient_DoesNotUpdateOtherInboundTraffic(t *testing.T) {
	setupTestDB(t)

	p = xray.NewProcess(&xray.Config{})
	svc := &InboundService{}
	duplicateEmail := "shared@example.com"
	renamedEmail := "renamed@example.com"

	inbound1 := mustCreateInboundWithClients(t, svc, model.Inbound{
		UserId:   1,
		Port:     11001,
		Protocol: model.VLESS,
		Tag:      "inbound-edit-1",
	}, model.Client{
		ID:         "client-1",
		Email:      duplicateEmail,
		Enable:     false,
		TotalGB:    10,
		ExpiryTime: 111,
	}, model.Client{
		ID:     "client-2",
		Email:  "unique-1@example.com",
		Enable: false,
	})

	inbound2 := mustCreateInboundWithClients(t, svc, model.Inbound{
		UserId:   1,
		Port:     11002,
		Protocol: model.VLESS,
		Tag:      "inbound-edit-2",
	}, model.Client{
		ID:         "client-3",
		Email:      duplicateEmail,
		Enable:     false,
		TotalGB:    20,
		ExpiryTime: 222,
	}, model.Client{
		ID:     "client-4",
		Email:  "unique-2@example.com",
		Enable: false,
	})

	updatePayload := &model.Inbound{
		Id: inbound1.Id,
		Settings: mustMarshalInboundSettings(t, model.Client{
			ID:         "client-1",
			Email:      renamedEmail,
			Enable:     false,
			TotalGB:    30,
			ExpiryTime: 333,
		}),
	}

	if _, err := svc.UpdateInboundClient(updatePayload, "client-1"); err != nil {
		t.Fatalf("update inbound client failed: %v", err)
	}

	var inbound1Traffic xray.ClientTraffic
	if err := database.GetDB().
		Where("inbound_id = ? AND email = ?", inbound1.Id, renamedEmail).
		First(&inbound1Traffic).Error; err != nil {
		t.Fatalf("expected updated inbound1 traffic row: %v", err)
	}
	if inbound1Traffic.Total != 30 || inbound1Traffic.ExpiryTime != 333 {
		t.Fatalf("unexpected inbound1 traffic values: %+v", inbound1Traffic)
	}

	var inbound2Traffic xray.ClientTraffic
	if err := database.GetDB().
		Where("inbound_id = ? AND email = ?", inbound2.Id, duplicateEmail).
		First(&inbound2Traffic).Error; err != nil {
		t.Fatalf("expected inbound2 traffic row to remain unchanged: %v", err)
	}
	if inbound2Traffic.Total != 20 || inbound2Traffic.ExpiryTime != 222 {
		t.Fatalf("unexpected inbound2 traffic values: %+v", inbound2Traffic)
	}
	if got := countClientTraffic(t, inbound2.Id, renamedEmail); got != 0 {
		t.Fatalf("expected renamed email to stay isolated to inbound1, got %d rows in inbound2", got)
	}
}

func TestMigrationRequirements_RollsBackOnAddClientStatFailure(t *testing.T) {
	setupTestDB(t)

	svc := &InboundService{}
	inbound := model.Inbound{
		UserId:   1,
		Port:     12001,
		Protocol: model.VLESS,
		Tag:      "rollback-test",
		Up:       10,
		Down:     20,
		Settings: mustMarshalInboundSettings(t, model.Client{
			ID:         "client-rollback",
			Email:      "rollback@example.com",
			Enable:     true,
			TotalGB:    100,
			ExpiryTime: 200,
		}),
	}
	if err := database.GetDB().Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}

	if err := database.GetDB().Exec(`
		CREATE TRIGGER fail_client_traffic_insert
		BEFORE INSERT ON client_traffics
		BEGIN
			SELECT RAISE(FAIL, 'boom');
		END;
	`).Error; err != nil {
		t.Fatalf("create trigger failed: %v", err)
	}

	err := svc.MigrationRequirements()
	if err == nil {
		t.Fatalf("expected migration requirements to return an error when client traffic insert fails")
	}

	var refreshed model.Inbound
	if err := database.GetDB().First(&refreshed, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound failed: %v", err)
	}
	if refreshed.AllTime != 0 {
		t.Fatalf("expected inbound all_time rollback to keep 0, got %d", refreshed.AllTime)
	}

	var traffic xray.ClientTraffic
	err = database.GetDB().
		Where("inbound_id = ? AND email = ?", inbound.Id, "rollback@example.com").
		First(&traffic).Error
	if err == nil {
		t.Fatalf("expected client traffic insert to roll back, but row exists: %+v", traffic)
	}
	if !database.IsNotFound(err) {
		t.Fatalf("reload client traffic failed: %v", err)
	}
}
