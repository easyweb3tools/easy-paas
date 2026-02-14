package models

import (
	"time"

	"gorm.io/datatypes"
)

type Series struct {
	ID                string         `gorm:"primaryKey;type:text;comment:系列唯一标识"`
	Title             string         `gorm:"type:text;not null;comment:系列名称"`
	Slug              *string        `gorm:"type:text;uniqueIndex;comment:URL友好标识"`
	Image             *string        `gorm:"type:text;comment:图片URL"`
	ExternalUpdatedAt *time.Time     `gorm:"type:timestamptz;index;comment:外部更新时间"`
	LastSeenAt        time.Time      `gorm:"type:timestamptz;not null;comment:最近同步时间"`
	RawJSON           datatypes.JSON `gorm:"type:jsonb;not null;comment:原始数据"`
}

func (Series) TableName() string {
	return "catalog_series"
}
