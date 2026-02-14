package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

// ExecutionPlan is L6: plan produced from an opportunity, optionally after risk preflight.
type ExecutionPlan struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement"`
	OpportunityID uint64 `gorm:"not null;index"`

	Status       string `gorm:"type:varchar(20);not null;default:'draft';index"`
	StrategyName string `gorm:"type:varchar(50);not null;index"`

	PlannedSizeUSD decimal.Decimal `gorm:"type:numeric(30,10);not null"`
	MaxLossUSD     decimal.Decimal `gorm:"type:numeric(30,10);not null"`
	KellyFraction  *float64

	Params          datatypes.JSON `gorm:"type:jsonb"`
	PreflightResult datatypes.JSON `gorm:"type:jsonb"`
	Legs            datatypes.JSON `gorm:"type:jsonb;not null"`

	ExecutedAt *time.Time `gorm:"type:timestamptz;index"`
	CreatedAt  time.Time  `gorm:"type:timestamptz;autoCreateTime"`
	UpdatedAt  time.Time  `gorm:"type:timestamptz;autoUpdateTime"`
}

func (ExecutionPlan) TableName() string {
	return "execution_plans"
}
