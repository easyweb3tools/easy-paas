package models

import (
	"time"

	"gorm.io/datatypes"
)

type SyncState struct {
	Scope         string         `gorm:"primaryKey;type:text;comment:同步范围标识"`
	Cursor        *string        `gorm:"type:text;comment:分页游标"`
	WatermarkTS   *time.Time     `gorm:"type:timestamptz;comment:更新时间水位"`
	LastSuccessAt *time.Time     `gorm:"type:timestamptz;comment:最近成功时间"`
	LastAttemptAt *time.Time     `gorm:"type:timestamptz;comment:最近尝试时间"`
	LastError     *string        `gorm:"type:text;comment:最近错误信息"`
	StatsJSON     datatypes.JSON `gorm:"type:jsonb;comment:本轮统计JSON"`
}

func (SyncState) TableName() string {
	return "sync_state"
}
