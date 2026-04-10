package service

import (
	"context"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
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

func (s *TrafficFlushService) Collect(clientTraffics []*xray.ClientTraffic) error {
	deltas := make([]TrafficDelta, 0, len(clientTraffics))
	for _, traffic := range clientTraffics {
		if traffic == nil || (traffic.Up == 0 && traffic.Down == 0) {
			continue
		}
		deltas = append(deltas, TrafficDelta{
			InboundID: traffic.InboundId,
			Email:     traffic.Email,
			UpDelta:   traffic.Up,
			DownDelta: traffic.Down,
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
			if err := tx.Model(&model.Inbound{}).
				Where("id = ?", delta.InboundID).
				Updates(map[string]any{
					"up":       gorm.Expr("up + ?", delta.UpDelta),
					"down":     gorm.Expr("down + ?", delta.DownDelta),
					"all_time": gorm.Expr("COALESCE(all_time, 0) + ?", delta.UpDelta+delta.DownDelta),
				}).Error; err != nil {
				return err
			}

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
