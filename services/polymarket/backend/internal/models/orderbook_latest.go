package models

import (
	"time"

	"gorm.io/datatypes"
)

type OrderbookLatest struct {
	TokenID        string         `gorm:"primaryKey;type:text;comment:合约ID"`
	SnapshotTS     time.Time      `gorm:"type:timestamptz;not null;comment:快照时间"`
	BidsJSON       datatypes.JSON `gorm:"type:jsonb;not null;comment:买盘前N档"`
	AsksJSON       datatypes.JSON `gorm:"type:jsonb;not null;comment:卖盘前N档"`
	BestBid        *float64       `gorm:"type:numeric;comment:最优买价"`
	BestAsk        *float64       `gorm:"type:numeric;comment:最优卖价"`
	Mid            *float64       `gorm:"type:numeric;comment:中间价"`
	Source         *string        `gorm:"type:text;comment:数据来源"`
	DataAgeSeconds int            `gorm:"not null;comment:数据新鲜度秒数"`
	UpdatedAt      time.Time      `gorm:"type:timestamptz;not null;comment:更新时间"`
}

func (OrderbookLatest) TableName() string {
	return "orderbook_latest"
}
