package models

import (
	"time"

	"gorm.io/datatypes"
)

// Strategy is L5: strategy config and state.
type Strategy struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:varchar(50);uniqueIndex;not null"`
	DisplayName string `gorm:"type:varchar(100);not null"`
	Description string `gorm:"type:text"`
	Category    string `gorm:"type:varchar(30);not null;index"`

	Enabled  bool `gorm:"default:false;index"`
	Priority int  `gorm:"default:0;index"`

	Params          datatypes.JSON `gorm:"type:jsonb;not null"`
	RequiredSignals datatypes.JSON `gorm:"type:jsonb"`
	Stats           datatypes.JSON `gorm:"type:jsonb"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (Strategy) TableName() string {
	return "strategies"
}
