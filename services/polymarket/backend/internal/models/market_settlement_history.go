package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

// MarketSettlementHistory is L6: historical settlements to support systematic strategies.
type MarketSettlementHistory struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement"`
	MarketID string `gorm:"type:varchar(100);not null;uniqueIndex"`
	EventID  string `gorm:"type:varchar(100);not null;index"`
	Question string `gorm:"type:text"`

	Outcome  string         `gorm:"type:varchar(10);not null;index"`
	Category string         `gorm:"type:varchar(50);index"`
	Labels   datatypes.JSON `gorm:"type:jsonb"`

	InitialYesPrice *decimal.Decimal `gorm:"type:numeric(20,10)"`
	FinalYesPrice   *decimal.Decimal `gorm:"type:numeric(20,10)"`

	SettledAt time.Time `gorm:"type:timestamptz;not null;index"`
	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
}

func (MarketSettlementHistory) TableName() string {
	return "market_settlement_history"
}
