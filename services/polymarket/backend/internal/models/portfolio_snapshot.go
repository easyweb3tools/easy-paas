package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type PortfolioSnapshot struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	SnapshotAt time.Time `gorm:"type:timestamptz;not null;uniqueIndex"`

	TotalPositions int `gorm:"not null"`

	TotalCostBasis decimal.Decimal `gorm:"type:numeric(30,10);not null"`
	TotalMarketVal decimal.Decimal `gorm:"type:numeric(30,10);not null"`
	UnrealizedPnL  decimal.Decimal `gorm:"column:unrealized_pnl;type:numeric(30,10);not null"`
	RealizedPnL    decimal.Decimal `gorm:"column:realized_pnl;type:numeric(30,10);not null"`
	NetLiquidation decimal.Decimal `gorm:"type:numeric(30,10);not null"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
}

func (PortfolioSnapshot) TableName() string {
	return "portfolio_snapshots"
}
