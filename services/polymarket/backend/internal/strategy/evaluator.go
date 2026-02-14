package strategy

import (
	"context"
	"encoding/json"

	"polymarket/internal/models"
)

type StrategyEvaluator interface {
	Name() string
	RequiredSignals() []string
	Evaluate(ctx context.Context, signals []models.Signal) ([]models.Opportunity, error)
	DefaultParams() json.RawMessage
}

// SignalSubscriber is satisfied by signal.SignalHub.
type SignalSubscriber interface {
	Subscribe(signalType string, buf int) <-chan models.Signal
}
