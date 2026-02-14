package models

import "time"

type LastTradePrice struct {
	TokenID   string     `gorm:"primaryKey;type:text;comment:合约ID"`
	Price     float64    `gorm:"type:numeric;not null;comment:最新成交价"`
	TradeTS   *time.Time `gorm:"type:timestamptz;comment:成交时间"`
	Source    *string    `gorm:"type:text;comment:数据来源"`
	UpdatedAt time.Time  `gorm:"type:timestamptz;not null;comment:更新时间"`
}

func (LastTradePrice) TableName() string {
	return "last_trade_prices"
}
