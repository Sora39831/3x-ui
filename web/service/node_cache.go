package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type SharedAccountsSnapshot struct {
	Version  int64            `json:"version"`
	Inbounds []*model.Inbound `json:"inbounds"`
}

func LoadSharedAccountsSnapshot(path string) (*SharedAccountsSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	snapshot := &SharedAccountsSnapshot{}
	if err := json.Unmarshal(data, snapshot); err != nil {
		return nil, err
	}
	if snapshot.Inbounds == nil {
		snapshot.Inbounds = []*model.Inbound{}
	}
	return snapshot, nil
}

func SaveSharedAccountsSnapshot(path string, snapshot *SharedAccountsSnapshot) error {
	if snapshot == nil {
		return errors.New("shared snapshot is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
