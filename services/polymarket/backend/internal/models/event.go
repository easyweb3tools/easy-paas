package models

import (
	"time"

	"gorm.io/datatypes"
)

type Event struct {
	ID                string         `gorm:"primaryKey;type:text;comment:事件唯一标识"`
	Slug              string         `gorm:"type:text;uniqueIndex;not null;comment:URL友好标识"`
	Title             string         `gorm:"type:text;not null;comment:事件标题"`
	Description       *string        `gorm:"type:text;comment:事件描述"`
	Active            bool           `gorm:"not null;default:true;comment:是否活跃"`
	Closed            bool           `gorm:"not null;default:false;comment:是否已关闭"`
	NegRisk           *bool          `gorm:"default:null;comment:是否为负风险事件"`
	StartTime         *time.Time     `gorm:"type:timestamptz;comment:开始时间"`
	EndTime           *time.Time     `gorm:"type:timestamptz;comment:结束时间"`
	SeriesID          *string        `gorm:"type:text;index;comment:关联系列ID"`
	ExternalCreatedAt *time.Time     `gorm:"type:timestamptz;comment:外部创建时间"`
	ExternalUpdatedAt *time.Time     `gorm:"type:timestamptz;index;comment:外部更新时间"`
	LastSeenAt        time.Time      `gorm:"type:timestamptz;not null;comment:最近同步时间"`
	RawJSON           datatypes.JSON `gorm:"type:jsonb;not null;comment:原始数据"`
}

func (Event) TableName() string {
	return "catalog_events"
}
