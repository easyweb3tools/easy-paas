package models

import (
	"time"

	"gorm.io/datatypes"
)

// SystemSetting stores runtime-configurable settings in DB for agent control.
type SystemSetting struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`

	Key string `gorm:"type:varchar(120);not null;uniqueIndex"`

	// JSON value, e.g. true/false for switches, or object for richer settings.
	Value datatypes.JSON `gorm:"type:jsonb;not null"`

	Description string    `gorm:"type:text"`
	CreatedAt   time.Time `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;autoUpdateTime;index"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}
