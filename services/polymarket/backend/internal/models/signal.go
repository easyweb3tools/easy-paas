package models

import (
	"time"

	"gorm.io/datatypes"
)

// Signal is L4 (Signals): raw signals produced by collectors.
type Signal struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	SignalType string `gorm:"type:varchar(50);not null;index"`
	Source     string `gorm:"type:varchar(50);not null;index"`

	MarketID *string `gorm:"type:varchar(100);index"`
	EventID  *string `gorm:"type:varchar(100);index"`
	TokenID  *string `gorm:"type:varchar(100);index"`

	Strength  float64        `gorm:"not null"`
	Direction string         `gorm:"type:varchar(10)"`
	Payload   datatypes.JSON `gorm:"type:jsonb"`

	ExpiresAt *time.Time `gorm:"type:timestamptz;index"`
	CreatedAt time.Time  `gorm:"type:timestamptz;autoCreateTime;index"`
}

func (Signal) TableName() string {
	return "signals"
}
