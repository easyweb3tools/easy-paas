package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Fill is L6: execution fill record.
type Fill struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	PlanID    uint64 `gorm:"not null;index"`
	TokenID   string `gorm:"type:varchar(100);not null;index"`
	Direction string `gorm:"type:varchar(10);not null"`

	FilledSize decimal.Decimal  `gorm:"type:numeric(30,10);not null"`
	AvgPrice   decimal.Decimal  `gorm:"type:numeric(20,10);not null"`
	Fee        decimal.Decimal  `gorm:"type:numeric(30,10);not null;default:0"`
	Slippage   *decimal.Decimal `gorm:"type:numeric(20,10)"`

	FilledAt  time.Time `gorm:"type:timestamptz;not null;index"`
	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
}

func (Fill) TableName() string {
	return "fills"
}
