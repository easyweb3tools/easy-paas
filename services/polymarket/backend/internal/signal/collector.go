package signal

import (
	"context"
	"time"

	"polymarket/internal/models"
)

type HealthStatus struct {
	Status     string
	LastPollAt *time.Time
	LastError  *string
	Details    map[string]any
}

type SourceInfo struct {
	SourceType   string
	Endpoint     string
	PollInterval time.Duration
}

type SignalSourceInfo interface {
	SourceInfo() SourceInfo
}

// SignalCollector produces L4 signals.
type SignalCollector interface {
	Name() string
	Start(ctx context.Context, out chan<- models.Signal) error
	Stop() error
	Health() HealthStatus
}
