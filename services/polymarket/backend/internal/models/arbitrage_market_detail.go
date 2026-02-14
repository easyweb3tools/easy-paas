package models

import "github.com/shopspring/decimal"

// ArbitrageMarketDetail 套利机会中每个市场的详情
type ArbitrageMarketDetail struct {
	OpportunityID string          `gorm:"primaryKey;type:text"`
	MarketID      string          `gorm:"primaryKey;type:text"`
	TokenID       string          `gorm:"type:text;not null"`
	Outcome       string          `gorm:"type:text;not null;comment:市场问题/选项名称"`
	BestBid       decimal.Decimal `gorm:"type:numeric(20,10)"`
	BestAsk       decimal.Decimal `gorm:"type:numeric(20,10)"`
	Mid           decimal.Decimal `gorm:"type:numeric(20,10)"`
	SpreadBps     decimal.Decimal `gorm:"type:numeric(10,2)"`
	BidDepthUSD   decimal.Decimal `gorm:"type:numeric(20,6)"`
	AskDepthUSD   decimal.Decimal `gorm:"type:numeric(20,6)"`
}

func (ArbitrageMarketDetail) TableName() string {
	return "arbitrage_market_details"
}
