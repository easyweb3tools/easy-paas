package models

import (
	"time"

	"gorm.io/datatypes"
)

type Token struct {
	ID                string         `gorm:"primaryKey;type:text;comment:合约唯一标识"`
	MarketID          string         `gorm:"type:text;index;not null;comment:关联市场ID"`
	Outcome           string         `gorm:"type:text;not null;comment:结果名称(Yes/No)"`
	Side              *string        `gorm:"type:text;comment:结果方向标识"`
	ExternalCreatedAt *time.Time     `gorm:"type:timestamptz;comment:外部创建时间"`
	ExternalUpdatedAt *time.Time     `gorm:"type:timestamptz;index;comment:外部更新时间"`
	LastSeenAt        time.Time      `gorm:"type:timestamptz;not null;comment:最近同步时间"`
	RawJSON           datatypes.JSON `gorm:"type:jsonb;not null;comment:原始数据"`
}

func (Token) TableName() string {
	return "catalog_tokens"
}
