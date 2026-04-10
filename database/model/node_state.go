package model

type NodeState struct {
	NodeID          string `json:"nodeId" gorm:"primaryKey"`
	NodeRole        string `json:"nodeRole" gorm:"not null"`
	LastSyncAt      int64  `json:"lastSyncAt"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
	LastSeenVersion int64  `json:"lastSeenVersion"`
	LastError       string `json:"lastError"`
	UpdatedAt       int64  `json:"updatedAt"`
}
