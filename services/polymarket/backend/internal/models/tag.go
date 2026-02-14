package models

import (
	"time"

	"gorm.io/datatypes"
)

type Tag struct {
	ID                string         `gorm:"primaryKey;type:text;comment:标签唯一标识"`
	Label             string         `gorm:"type:text;not null;comment:标签名称"`
	Slug              string         `gorm:"type:text;uniqueIndex;not null;comment:URL友好标识"`
	ExternalUpdatedAt *time.Time     `gorm:"type:timestamptz;index;comment:外部更新时间"`
	LastSeenAt        time.Time      `gorm:"type:timestamptz;not null;comment:最近同步时间"`
	RawJSON           datatypes.JSON `gorm:"type:jsonb;not null;comment:原始数据"`
}

func (Tag) TableName() string {
	return "catalog_tags"
}
