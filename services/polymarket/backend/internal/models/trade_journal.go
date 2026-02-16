package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

// TradeJournal stores decision-chain snapshots for post-trade review.
type TradeJournal struct {
	ID uint64 `gorm:"primaryKey;autoIncrement"`

	ExecutionPlanID uint64 `gorm:"not null;uniqueIndex"`
	OpportunityID   uint64 `gorm:"not null;index"`
	StrategyName    string `gorm:"type:varchar(50);not null;index"`

	EntryReasoning string         `gorm:"type:text"`
	SignalSnapshot datatypes.JSON `gorm:"type:jsonb"`
	MarketSnapshot datatypes.JSON `gorm:"type:jsonb"`
	EntryParams    datatypes.JSON `gorm:"type:jsonb"`

	ExitReasoning   string           `gorm:"type:text"`
	OutcomeSnapshot datatypes.JSON   `gorm:"type:jsonb"`
	Outcome         string           `gorm:"type:varchar(20);index"`
	PnLUSD          *decimal.Decimal `gorm:"column:pnl_usd;type:numeric(30,10)"`
	ROI             *decimal.Decimal `gorm:"type:numeric(20,10)"`

	Notes      string         `gorm:"type:text"`
	Tags       datatypes.JSON `gorm:"type:jsonb"`
	ReviewedAt *time.Time     `gorm:"type:timestamptz"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime;index"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (TradeJournal) TableName() string {
	return "trade_journals"
}
