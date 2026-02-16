package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Position struct {
	ID       uint64 `gorm:"primaryKey;autoIncrement"`
	TokenID  string `gorm:"type:varchar(100);not null;uniqueIndex"`
	MarketID string `gorm:"type:varchar(100);not null;index"`
	EventID  string `gorm:"type:varchar(100);index"`

	Direction string `gorm:"type:varchar(10);not null"`

	Quantity      decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
	AvgEntryPrice decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
	CurrentPrice  decimal.Decimal `gorm:"type:numeric(20,10);not null;default:0"`
	CostBasis     decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`
	UnrealizedPnL decimal.Decimal `gorm:"column:unrealized_pnl;type:numeric(30,10);not null;default:0"`
	RealizedPnL   decimal.Decimal `gorm:"column:realized_pnl;type:numeric(30,10);not null;default:0"`

	Status       string     `gorm:"type:varchar(20);not null;default:'open';index"`
	StrategyName string     `gorm:"type:varchar(50);index"`
	OpenedAt     time.Time  `gorm:"type:timestamptz;not null"`
	ClosedAt     *time.Time `gorm:"type:timestamptz"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime;index"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (Position) TableName() string {
	return "positions"
}
