package service

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/xray"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TrafficFlushService struct {
	store       *TrafficPendingStore
	inbounds    InboundService
	xrayService XrayService
	flushFn     func([]TrafficDelta) error
	reconcileFn func(*gorm.DB) (bool, error)
	markRestart func()
}

func NewTrafficFlushService(store *TrafficPendingStore) *TrafficFlushService {
	svc := &TrafficFlushService{store: store}
	svc.flushFn = svc.flushToDatabase
	svc.reconcileFn = svc.inbounds.ReconcileSharedTrafficState
	svc.markRestart = svc.xrayService.SetToNeedRestart
	return svc
}

func (s *TrafficFlushService) Collect(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) error {
	// Resolve email → InboundId mapping from database, since Xray API only
	// returns email for client traffic without the InboundId.
	emailToInboundID := map[string]int{}
	if len(clientTraffics) > 0 {
		emails := make([]string, 0, len(clientTraffics))
		for _, ct := range clientTraffics {
			if ct != nil && ct.Email != "" {
				emails = append(emails, ct.Email)
			}
		}
		if len(emails) > 0 {
			var rows []xray.ClientTraffic
			if err := database.GetDB().Model(&xray.ClientTraffic{}).
				Select("inbound_id, email").
				Where("email IN (?)", emails).
				Find(&rows).Error; err != nil {
				logger.Warning("resolve email to inbound_id failed:", err)
			}
			for _, r := range rows {
				emailToInboundID[r.Email] = r.InboundId
			}
		}
	}

	deltas := make([]TrafficDelta, 0, len(clientTraffics)+len(inboundTraffics))
	clientTotals := map[int]TrafficDelta{}

	for _, traffic := range clientTraffics {
		if traffic == nil || (traffic.Up == 0 && traffic.Down == 0) {
			continue
		}
		resolvedID := emailToInboundID[traffic.Email]
		if resolvedID == 0 {
			logger.Warningf("skip client traffic for unknown email %q (no inbound_id in DB)", traffic.Email)
			continue
		}
		delta := TrafficDelta{
			Kind:      TrafficDeltaKindClient,
			InboundID: resolvedID,
			Email:     traffic.Email,
			UpDelta:   traffic.Up,
			DownDelta: traffic.Down,
		}
		deltas = append(deltas, delta)
		total := clientTotals[resolvedID]
		total.UpDelta += traffic.Up
		total.DownDelta += traffic.Down
		clientTotals[resolvedID] = total
	}

	for _, traffic := range inboundTraffics {
		if traffic == nil || !traffic.IsInbound || (traffic.Up == 0 && traffic.Down == 0) {
			continue
		}

		var inbound model.Inbound
		if err := database.GetDB().Select("id").First(&inbound, "tag = ?", traffic.Tag).Error; err != nil {
			logger.Warning("resolve inbound tag for shared traffic failed:", err)
			continue
		}

		clientTotal := clientTotals[inbound.Id]
		residualUp := traffic.Up - clientTotal.UpDelta
		residualDown := traffic.Down - clientTotal.DownDelta
		if residualUp < 0 || residualDown < 0 {
			logger.Warningf(
				"shared traffic residual below zero: tag=%s inbound_id=%d inbound_up=%d inbound_down=%d client_up=%d client_down=%d residual_up=%d residual_down=%d",
				traffic.Tag,
				inbound.Id,
				traffic.Up,
				traffic.Down,
				clientTotal.UpDelta,
				clientTotal.DownDelta,
				residualUp,
				residualDown,
			)
			if residualUp < 0 {
				residualUp = 0
			}
			if residualDown < 0 {
				residualDown = 0
			}
		}
		if residualUp == 0 && residualDown == 0 {
			continue
		}

		deltas = append(deltas, TrafficDelta{
			Kind:      TrafficDeltaKindInboundOnly,
			InboundID: inbound.Id,
			UpDelta:   residualUp,
			DownDelta: residualDown,
		})
	}

	if len(deltas) == 0 {
		return nil
	}
	return s.store.Merge(deltas)
}

func (s *TrafficFlushService) flushToDatabase(deltas []TrafficDelta) error {
	now := time.Now().UnixMilli()

	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, delta := range deltas {
			kind := delta.Kind
			if kind == "" {
				kind = TrafficDeltaKindClient
			}

			if err := tx.Model(&model.Inbound{}).
				Where("id = ?", delta.InboundID).
				Updates(map[string]any{
					"up":       gorm.Expr("up + ?", delta.UpDelta),
					"down":     gorm.Expr("down + ?", delta.DownDelta),
					"all_time": gorm.Expr("COALESCE(all_time, 0) + ?", delta.UpDelta+delta.DownDelta),
				}).Error; err != nil {
				return err
			}

			if kind == TrafficDeltaKindClient {
				row := xray.ClientTraffic{
					InboundId:  delta.InboundID,
					Email:      delta.Email,
					Enable:     true,
					Up:         delta.UpDelta,
					Down:       delta.DownDelta,
					AllTime:    delta.UpDelta + delta.DownDelta,
					LastOnline: now,
				}
				if err := tx.Clauses(clause.OnConflict{
					Columns: []clause.Column{{Name: "inbound_id"}, {Name: "email"}},
					DoUpdates: clause.Assignments(map[string]any{
						"up":          gorm.Expr("up + ?", delta.UpDelta),
						"down":        gorm.Expr("down + ?", delta.DownDelta),
						"all_time":    gorm.Expr("COALESCE(all_time, 0) + ?", delta.UpDelta+delta.DownDelta),
						"last_online": now,
					}),
				}).Create(&row).Error; err != nil {
					return err
				}
			}
		}

		if IsMaster() {
			needRestart, err := s.reconcileFn(tx)
			if err != nil {
				return err
			}
			if needRestart && s.markRestart != nil {
				s.markRestart()
			}
		}
		return nil
	})
}

func (s *TrafficFlushService) FlushOnce() error {
	deltas, err := s.store.Take()
	if err != nil || len(deltas) == 0 {
		return err
	}
	if err := s.flushFn(deltas); err != nil {
		return common.Combine(err, s.store.Merge(deltas))
	}
	return nil
}

func (s *TrafficFlushService) Run(ctx context.Context) {
	interval := time.Duration(config.GetNodeConfigFromJSON().TrafficFlushSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = s.FlushOnce()
			return
		case <-ticker.C:
			_ = s.FlushOnce()
		}
	}
}
