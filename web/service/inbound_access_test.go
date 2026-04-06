package service

import (
	"errors"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/gorm"
)

func TestGetInboundForUser_DeniesOtherUsers(t *testing.T) {
	setupTestDB(t)

	svc := &InboundService{}
	inbound := mustCreateInboundWithClients(t, svc, model.Inbound{
		UserId:   2,
		Port:     13001,
		Protocol: model.VLESS,
		Tag:      "owned-by-user-2",
	}, model.Client{
		ID:     "client-1",
		Email:  "user2@example.com",
		Enable: false,
	})

	_, err := svc.GetInboundForUser(1, false, inbound.Id)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}

	got, err := svc.GetInboundForUser(2, false, inbound.Id)
	if err != nil {
		t.Fatalf("expected owner to fetch inbound: %v", err)
	}
	if got.Id != inbound.Id {
		t.Fatalf("expected inbound %d, got %d", inbound.Id, got.Id)
	}
}

func TestDelInboundForUser_DeniesOtherUsers(t *testing.T) {
	setupTestDB(t)

	svc := &InboundService{}
	inbound := mustCreateInboundWithClients(t, svc, model.Inbound{
		UserId:   2,
		Port:     13002,
		Protocol: model.VLESS,
		Tag:      "delete-owned-by-user-2",
	}, model.Client{
		ID:     "client-1",
		Email:  "user2@example.com",
		Enable: false,
	})

	_, err := svc.DelInboundForUser(1, false, inbound.Id)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}

	if _, err := svc.GetInbound(inbound.Id); err != nil {
		t.Fatalf("expected inbound to remain after denied delete: %v", err)
	}
}
