package model

type SharedState struct {
	Key       string `json:"key" gorm:"primaryKey"`
	Version   int64  `json:"version" gorm:"not null;default:0"`
	UpdatedAt int64  `json:"updatedAt"`
}
