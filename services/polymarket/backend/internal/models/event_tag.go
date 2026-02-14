package models

type EventTag struct {
	EventID string `gorm:"primaryKey;type:text;comment:事件ID"`
	TagID   string `gorm:"primaryKey;type:text;comment:标签ID"`
}

func (EventTag) TableName() string {
	return "catalog_event_tags"
}
