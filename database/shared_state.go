package database

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"gorm.io/gorm"
)

const SharedAccountsVersionKey = "shared_accounts_version"

func txOrDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return GetDB()
}

func seedSharedAccountsVersion(tx *gorm.DB) error {
	state := &model.SharedState{
		Key: SharedAccountsVersionKey,
	}
	return txOrDB(tx).
		Attrs(&model.SharedState{
			Version:   0,
			UpdatedAt: time.Now().Unix(),
		}).
		FirstOrCreate(state).Error
}

func GetSharedAccountsVersion(tx *gorm.DB) (int64, error) {
	state := &model.SharedState{
		Key: SharedAccountsVersionKey,
	}
	if err := txOrDB(tx).First(state).Error; err != nil {
		return 0, err
	}
	return state.Version, nil
}

func BumpSharedAccountsVersion(tx *gorm.DB) error {
	return txOrDB(tx).Model(&model.SharedState{}).
		Where(&model.SharedState{Key: SharedAccountsVersionKey}).
		Updates(map[string]any{
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now().Unix(),
		}).Error
}

func UpsertNodeState(tx *gorm.DB, state *model.NodeState) error {
	state.UpdatedAt = time.Now().Unix()
	return txOrDB(tx).Save(state).Error
}

// GetNodeStates returns all node_state records ordered by node_id.
func GetNodeStates() ([]model.NodeState, error) {
	var states []model.NodeState
	err := GetDB().Order("node_id").Find(&states).Error
	return states, err
}
