package logging

import (
	"encoding/json"
	"time"
)

type OperationLog struct {
	ID         string          `json:"id"`
	ProjectID  string          `json:"project"`
	Agent      string          `json:"agent"`
	Action     string          `json:"action"`
	Level      string          `json:"level"`
	Details    json.RawMessage `json:"details"`
	SessionKey string          `json:"session_key"`
	CreatedAt  time.Time       `json:"created_at"`
	Metadata   json.RawMessage `json:"metadata"`
}

type ListFilter struct {
	ProjectID string
	Action    string
	Level     string
	From      *time.Time
	To        *time.Time
	Limit     int
}
