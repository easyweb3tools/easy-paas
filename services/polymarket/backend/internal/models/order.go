package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	PlanID      uint64 `gorm:"not null;index"`
	ClobOrderID string `gorm:"type:varchar(100);index"`
	TokenID     string `gorm:"type:varchar(100);not null;index"`

	Side      string `gorm:"type:varchar(10);not null"`
	OrderType string `gorm:"type:varchar(20);not null;default:'limit'"`

	Price     decimal.Decimal `gorm:"type:numeric(20,10);not null"`
	SizeUSD   decimal.Decimal `gorm:"type:numeric(30,10);not null"`
	FilledUSD decimal.Decimal `gorm:"type:numeric(30,10);not null;default:0"`

	Status        string `gorm:"type:varchar(20);not null;default:'pending';index"`
	FailureReason string `gorm:"type:text"`

	SubmittedAt *time.Time `gorm:"type:timestamptz"`
	FilledAt    *time.Time `gorm:"type:timestamptz"`
	CancelledAt *time.Time `gorm:"type:timestamptz"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime;index"`
	UpdatedAt time.Time `gorm:"type:timestamptz;autoUpdateTime"`
}

func (Order) TableName() string {
	return "orders"
}
