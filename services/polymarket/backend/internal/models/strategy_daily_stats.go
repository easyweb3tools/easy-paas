package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type StrategyDailyStats struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	StrategyName string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_strategy_daily;index"`
	Date         time.Time `gorm:"type:date;not null;uniqueIndex:idx_strategy_daily;index"`

	TradesCount int `gorm:"not null;default:0"`
	WinCount    int `gorm:"not null;default:0"`
	LossCount   int `gorm:"not null;default:0"`

	PnLUSD         decimal.Decimal `gorm:"column:pnl_usd;type:numeric(30,10);not null;default:0"`
	AvgEdgePct     decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
	AvgSlippageBps decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
	AvgHoldHours   decimal.Decimal `gorm:"type:numeric(20,4);not null;default:0"`
	MaxDrawdownUSD decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
	CumulativePnL  decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (StrategyDailyStats) TableName() string {
	return "strategy_daily_stats"
}
