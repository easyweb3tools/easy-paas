package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// PnLRecord is L6: post-trade analytics record.
type PnLRecord struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement"`
	PlanID       uint64 `gorm:"not null;uniqueIndex"`
	StrategyName string `gorm:"type:varchar(50);not null;index"`

	ExpectedEdge decimal.Decimal  `gorm:"type:numeric(20,10);not null"`
	// Use explicit column names because default GORM naming turns "PnL" into "pn_l".
	RealizedPnL  *decimal.Decimal `gorm:"column:realized_pnl;type:numeric(30,10)"`
	RealizedROI  *decimal.Decimal `gorm:"column:realized_roi;type:numeric(20,10)"`
	SlippageLoss *decimal.Decimal `gorm:"type:numeric(30,10)"`

	Outcome       string  `gorm:"type:varchar(20);index"`
	FailureReason *string `gorm:"type:varchar(50);index"`

	SettledAt *time.Time `gorm:"type:timestamptz;index"`
	Notes     *string    `gorm:"type:text"`
	CreatedAt time.Time  `gorm:"type:timestamptz;autoCreateTime"`
}

func (PnLRecord) TableName() string {
	return "pnl_records"
}
