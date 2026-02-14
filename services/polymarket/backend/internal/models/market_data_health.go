package models

import "time"

type MarketDataHealth struct {
	TokenID          string     `gorm:"primaryKey;type:text;comment:合约ID"`
	WSConnected      bool       `gorm:"not null;comment:WS连接状态"`
	LastWSTS         *time.Time `gorm:"column:last_ws_ts;type:timestamptz;comment:最后WS时间"`
	LastRESTTS       *time.Time `gorm:"column:last_rest_ts;type:timestamptz;comment:最后REST时间"`
	DataAgeSeconds   int        `gorm:"not null;comment:数据新鲜度秒数"`
	Stale            bool       `gorm:"not null;comment:是否过期"`
	NeedsResync      bool       `gorm:"not null;comment:是否需要纠偏"`
	LastResyncTS     *time.Time `gorm:"type:timestamptz;comment:最后纠偏时间"`
	Spread           *float64   `gorm:"type:numeric;comment:买卖价差"`
	SpreadBps        *float64   `gorm:"type:numeric;comment:价差bps"`
	PriceJumpBps     *float64   `gorm:"type:numeric;comment:价格跳变幅度bps"`
	LastBookChangeTS *time.Time `gorm:"type:timestamptz;comment:最近盘口更新时间"`
	Reason           *string    `gorm:"type:text;comment:原因说明"`
	UpdatedAt        time.Time  `gorm:"type:timestamptz;not null;comment:更新时间"`
}

func (MarketDataHealth) TableName() string {
	return "market_data_health"
}
