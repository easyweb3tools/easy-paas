package models

import (
	"time"

	"gorm.io/datatypes"
)

type RawRESTSnapshot struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement;comment:快照ID"`
	TokenID      *string        `gorm:"type:text;index;comment:合约ID"`
	SnapshotType string         `gorm:"type:text;not null;comment:快照类型"`
	FetchedAt    time.Time      `gorm:"type:timestamptz;not null;comment:获取时间"`
	Payload      datatypes.JSON `gorm:"type:jsonb;not null;comment:原始载荷"`
}

func (RawRESTSnapshot) TableName() string {
	return "raw_rest_snapshots"
}
