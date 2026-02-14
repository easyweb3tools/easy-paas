package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

// Opportunity is L5: normalized opportunity output for all strategies.
type Opportunity struct {
	ID         uint64 `gorm:"primaryKey;autoIncrement"`
	StrategyID uint64 `gorm:"not null;index"`
	Strategy   Strategy

	Status  string  `gorm:"type:varchar(20);not null;index;default:'active'"`
	EventID *string `gorm:"type:varchar(100);index"`
	// PrimaryMarketID is used to deduplicate opportunities that are scoped to a single market.
	PrimaryMarketID *string `gorm:"type:varchar(100);index"`

	MarketIDs datatypes.JSON `gorm:"type:jsonb"`

	// Core metrics. Store money-like values as numeric to avoid float errors.
	EdgePct decimal.Decimal `gorm:"type:numeric(20,10);not null"`
	EdgeUSD decimal.Decimal `gorm:"type:numeric(30,10);not null"`
	MaxSize decimal.Decimal `gorm:"type:numeric(30,10);not null"`

	Confidence float64 `gorm:"not null"`
	RiskScore  float64 `gorm:"not null"`

	DecayType string     `gorm:"type:varchar(20)"`
	ExpiresAt *time.Time `gorm:"type:timestamptz;index"`

	Legs      datatypes.JSON `gorm:"type:jsonb;not null"`
	SignalIDs datatypes.JSON `gorm:"type:jsonb"`
	Reasoning string         `gorm:"type:text"`
	DataAgeMs int            `gorm:"not null"`
	Warnings  datatypes.JSON `gorm:"type:jsonb"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime;index"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (Opportunity) TableName() string {
	return "opportunities"
}
