package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type MarketReview struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	MarketID  string `gorm:"type:varchar(100);not null;uniqueIndex;index"`
	EventID   string `gorm:"type:varchar(100);index"`
	OurAction string `gorm:"type:varchar(20);not null;index"`

	OpportunityID *uint64 `gorm:"index"`
	StrategyName  string  `gorm:"type:varchar(50);index"`

	EdgeAtEntry     *decimal.Decimal `gorm:"type:numeric(20,10)"`
	FinalOutcome    string           `gorm:"type:varchar(10)"`
	FinalPrice      *decimal.Decimal `gorm:"type:numeric(20,10)"`
	HypotheticalPnL decimal.Decimal  `gorm:"column:hypothetical_pnl;type:numeric(30,10);not null;default:0"`
	ActualPnL       decimal.Decimal  `gorm:"column:actual_pnl;type:numeric(30,10);not null;default:0"`

	LessonTags datatypes.JSON `gorm:"type:jsonb"`
	Notes      string         `gorm:"type:text"`

	SettledAt time.Time `gorm:"type:timestamptz;index"`
	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (MarketReview) TableName() string {
	return "market_reviews"
}
