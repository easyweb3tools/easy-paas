package models

import (
	"time"

	"gorm.io/datatypes"
)

type RawWSEvent struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement;comment:事件ID"`
	TokenID    *string        `gorm:"type:text;index;comment:合约ID"`
	EventType  string         `gorm:"type:text;not null;comment:事件类型"`
	Sequence   *int64         `gorm:"comment:事件序号"`
	ReceivedAt time.Time      `gorm:"type:timestamptz;not null;comment:接收时间"`
	Payload    datatypes.JSON `gorm:"type:jsonb;not null;comment:原始载荷"`
}

func (RawWSEvent) TableName() string {
	return "raw_ws_events"
}
