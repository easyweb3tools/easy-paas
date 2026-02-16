package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"

	polymarketgamma "polymarket/internal/client/polymarket/gamma"
	"polymarket/internal/config"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

// SettlementIngestService attempts to auto-ingest market resolution outcomes into market_settlement_history.
//
// Notes:
//   - Public Gamma responses do not guarantee a resolved outcome field. This ingestor is best-effort:
//     if it cannot extract a YES/NO outcome from the raw market JSON, it skips the market.
//   - This is intentionally disabled by default (see config).
type SettlementIngestService struct {
	Repo   repository.Repository
	Gamma  *polymarketgamma.Client
	Config config.SettlementIngestConfig
	Logger *zap.Logger
	Flags  *SystemSettingsService
}

func (s *SettlementIngestService) Run(ctx context.Context) error {
	if s == nil || s.Repo == nil || s.Gamma == nil {
		return nil
	}
	interval := s.Config.ScanInterval
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	// Run once on start.
	_ = s.runOnceIfEnabled(ctx)

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			_ = s.runOnceIfEnabled(ctx)
		}
	}
}

func (s *SettlementIngestService) runOnceIfEnabled(ctx context.Context) error {
	if s != nil && s.Flags != nil && !s.Flags.IsEnabled(ctx, FeatureSettlementIngest, false) {
		return nil
	}
	return s.RunOnce(ctx)
}

func (s *SettlementIngestService) RunOnce(ctx context.Context) error {
	if s == nil || s.Repo == nil || s.Gamma == nil {
		return nil
	}
	now := time.Now().UTC()
	lookback := s.Config.LookbackDays
	if lookback <= 0 {
		lookback = 14
	}
	batch := s.Config.BatchSize
	if batch <= 0 || batch > 1000 {
		batch = 200
	}
	cutoff := now.Add(-time.Duration(lookback) * 24 * time.Hour)

	closed := true
	offset := 0
	for {
		markets, err := s.Repo.ListMarkets(ctx, repository.ListMarketsParams{
			Limit:   batch,
			Offset:  offset,
			Closed:  &closed,
			OrderBy: "external_updated_at",
			Asc:     boolPtr(false),
		})
		if err != nil {
			s.logWarn("settlement ingest list markets failed", err)
			return err
		}
		if len(markets) == 0 {
			return nil
		}
		marketIDs := make([]string, 0, len(markets))
		for _, m := range markets {
			if strings.TrimSpace(m.ID) == "" {
				continue
			}
			if m.ExternalUpdatedAt != nil && m.ExternalUpdatedAt.Before(cutoff) {
				// Because we sort by external_updated_at desc, once we hit older than cutoff we can stop.
				return nil
			}
			marketIDs = append(marketIDs, m.ID)
		}
		if len(marketIDs) == 0 {
			return nil
		}
		existing, _ := s.Repo.ListMarketSettlementHistoryByMarketIDs(ctx, marketIDs)
		exists := map[string]struct{}{}
		for _, row := range existing {
			if strings.TrimSpace(row.MarketID) != "" {
				exists[strings.TrimSpace(row.MarketID)] = struct{}{}
			}
		}

		for _, mkt := range markets {
			marketID := strings.TrimSpace(mkt.ID)
			if marketID == "" {
				continue
			}
			if _, ok := exists[marketID]; ok {
				continue
			}
			raw, err := s.Gamma.GetMarketRawByID(ctx, marketID, nil)
			if err != nil {
				// Skip noisy errors; try again next loop.
				s.logWarn("gamma market fetch failed", err, zap.String("market_id", marketID))
				continue
			}
			outcome, settledAt, initialYes, finalYes, err := extractBinarySettlement(raw)
			if err != nil {
				continue
			}
			if outcome != "YES" && outcome != "NO" {
				continue
			}
			if settledAt.IsZero() {
				settledAt = now
			}

			labelsJSON := datatypes.JSON([]byte(`[]`))
			mid := marketID
			labels, _ := s.Repo.ListMarketLabels(ctx, repository.ListMarketLabelsParams{
				Limit:    2000,
				Offset:   0,
				MarketID: &mid,
				OrderBy:  "created_at",
				Asc:      boolPtr(false),
			})
			seen := map[string]struct{}{}
			labelNames := make([]string, 0, len(labels))
			for _, l := range labels {
				val := strings.TrimSpace(l.Label)
				if val == "" {
					continue
				}
				if _, ok := seen[val]; ok {
					continue
				}
				seen[val] = struct{}{}
				labelNames = append(labelNames, val)
			}
			if raw, err := json.Marshal(labelNames); err == nil {
				labelsJSON = datatypes.JSON(raw)
			}

			item := &models.MarketSettlementHistory{
				MarketID:        marketID,
				EventID:         mkt.EventID,
				Question:        mkt.Question,
				Outcome:         outcome,
				Category:        "",
				Labels:          labelsJSON,
				InitialYesPrice: initialYes,
				FinalYesPrice:   finalYes,
				SettledAt:       settledAt,
				CreatedAt:       now,
			}
			if err := s.Repo.UpsertMarketSettlementHistory(ctx, item); err != nil {
				s.logWarn("upsert settlement history failed", err, zap.String("market_id", marketID))
			}
		}

		if len(markets) < batch {
			return nil
		}
		offset += batch
	}
}

// extractBinarySettlement tries to decode a YES/NO settlement from raw Gamma market JSON.
// This is best-effort: it returns an error if it cannot find a usable outcome.
func extractBinarySettlement(raw []byte) (outcome string, settledAt time.Time, initialYes *decimal.Decimal, finalYes *decimal.Decimal, err error) {
	var obj map[string]any
	if len(raw) == 0 {
		return "", time.Time{}, nil, nil, errors.New("empty")
	}
	if e := json.Unmarshal(raw, &obj); e != nil {
		return "", time.Time{}, nil, nil, e
	}
	// Common candidates across various APIs/versions.
	for _, key := range []string{"resolution", "resolvedOutcome", "resolved_outcome", "outcome", "answer", "result", "winningOutcome", "winning_outcome"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				switch strings.ToUpper(strings.TrimSpace(s)) {
				case "YES", "Y", "TRUE", "1", "YES ":
					outcome = "YES"
				case "NO", "N", "FALSE", "0", "NO ":
					outcome = "NO"
				default:
					// Sometimes "Yes"/"No"
					if strings.EqualFold(strings.TrimSpace(s), "yes") {
						outcome = "YES"
					}
					if strings.EqualFold(strings.TrimSpace(s), "no") {
						outcome = "NO"
					}
				}
			}
		}
		if outcome == "YES" || outcome == "NO" {
			break
		}
	}
	if outcome == "" {
		return "", time.Time{}, nil, nil, errors.New("no outcome")
	}

	// Timestamps (best-effort).
	for _, key := range []string{"resolvedAt", "resolved_at", "settledAt", "settled_at", "closedTime", "closed_time", "updatedAt", "updated_at"} {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				if ts, e := time.Parse(time.RFC3339, strings.TrimSpace(s)); e == nil {
					settledAt = ts.UTC()
					break
				}
			}
		}
	}

	// Prices (optional).
	initialYes = parseDecimalFromAny(obj["initialYesPrice"])
	finalYes = parseDecimalFromAny(obj["finalYesPrice"])

	return outcome, settledAt, initialYes, finalYes, nil
}

func parseDecimalFromAny(v any) *decimal.Decimal {
	switch x := v.(type) {
	case float64:
		d := decimal.NewFromFloat(x)
		return &d
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		if d, err := decimal.NewFromString(s); err == nil {
			return &d
		}
	}
	return nil
}

func (s *SettlementIngestService) logWarn(msg string, err error, fields ...zap.Field) {
	if s == nil || s.Logger == nil {
		return
	}
	s.Logger.Warn(msg, append(fields, zap.Error(err))...)
}
