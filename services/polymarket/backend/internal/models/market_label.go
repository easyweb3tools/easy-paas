package models

import "time"

// MarketLabel is L5: market labeling for strategy filtering.
type MarketLabel struct {
	ID       uint64  `gorm:"primaryKey;autoIncrement"`
	MarketID string  `gorm:"type:varchar(100);not null;uniqueIndex:uniq_market_label"`
	Label    string  `gorm:"type:varchar(50);not null;uniqueIndex:uniq_market_label"`
	SubLabel *string `gorm:"type:varchar(50)"`

	AutoLabeled bool    `gorm:"default:false"`
	Confidence  float64 `gorm:"default:1.0"`

	CreatedAt time.Time `gorm:"type:timestamptz;autoCreateTime"`
}

func (MarketLabel) TableName() string {
	return "market_labels"
}
