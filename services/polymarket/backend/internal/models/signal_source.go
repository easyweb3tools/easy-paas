package models

import (
	"time"

	"gorm.io/datatypes"
)

// SignalSource stores external/internal collector configuration and health state.
type SignalSource struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement"`
	Name         string         `gorm:"type:varchar(50);uniqueIndex;not null"`
	SourceType   string         `gorm:"type:varchar(30);not null"`
	Endpoint     string         `gorm:"type:varchar(500)"`
	PollInterval string         `gorm:"type:varchar(20)"`
	Enabled      bool           `gorm:"default:true"`
	LastPollAt   *time.Time     `gorm:"type:timestamptz"`
	LastError    *string        `gorm:"type:text"`
	HealthStatus string         `gorm:"type:varchar(20);default:'unknown'"`
	Config       datatypes.JSON `gorm:"type:jsonb"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (SignalSource) TableName() string {
	return "signal_sources"
}
