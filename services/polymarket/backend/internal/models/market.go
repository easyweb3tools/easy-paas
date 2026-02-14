package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
)

type Market struct {
	ID                string           `gorm:"primaryKey;type:text;comment:市场唯一标识"`
	EventID           string           `gorm:"type:text;index;not null;comment:关联事件ID"`
	Slug              *string          `gorm:"type:text;uniqueIndex;comment:URL友好标识"`
	Question          string           `gorm:"type:text;not null;comment:市场问题"`
	ConditionID       string           `gorm:"type:text;index;not null;comment:链上条件ID"`
	MarketAddress     *string          `gorm:"type:text;comment:合约地址"`
	TickSize          decimal.Decimal  `gorm:"type:numeric(20,10);not null;comment:最小价格单位"`
	Volume            *decimal.Decimal `gorm:"type:numeric(30,10);comment:交易量"`
	Liquidity         *decimal.Decimal `gorm:"type:numeric(30,10);comment:流动性"`
	Active            bool             `gorm:"not null;default:true;comment:是否活跃"`
	Closed            bool             `gorm:"not null;default:false;comment:是否已关闭"`
	NegRisk           *bool            `gorm:"default:null;comment:是否为负风险市场"`
	Status            *string          `gorm:"type:text;comment:市场状态"`
	ExternalCreatedAt *time.Time       `gorm:"type:timestamptz;comment:外部创建时间"`
	ExternalUpdatedAt *time.Time       `gorm:"type:timestamptz;index;comment:外部更新时间"`
	LastSeenAt        time.Time        `gorm:"type:timestamptz;not null;comment:最近同步时间"`
	RawJSON           datatypes.JSON   `gorm:"type:jsonb;not null;comment:原始数据"`
}

func (Market) TableName() string {
	return "catalog_markets"
}
