package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// ExecutionRule controls whether a strategy can be auto-executed.
type ExecutionRule struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`

	StrategyName string `gorm:"type:varchar(50);not null;uniqueIndex"`
	AutoExecute  bool   `gorm:"not null;default:false"`

	MinConfidence float64         `gorm:"not null;default:0.8"`
	MinEdgePct    decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0.05"`

	StopLossPct    decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0.10"`
	TakeProfitPct  decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0.20"`
	MaxHoldHours   int             `gorm:"not null;default:72"`
	MaxDailyTrades int             `gorm:"not null;default:10"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (ExecutionRule) TableName() string {
	return "execution_rules"
}
