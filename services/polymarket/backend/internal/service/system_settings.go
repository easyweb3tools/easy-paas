package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/datatypes"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

const (
	FeatureCatalogSync        = "feature.catalog_sync"
	FeatureCLOBStream         = "feature.clob_stream"
	FeatureStrategyEngine     = "feature.strategy_engine"
	FeatureLabeler            = "feature.labeler"
	FeatureSettlementIngest   = "feature.settlement_ingest"
	FeatureAutoExecutor       = "feature.auto_executor"
	FeaturePositionSync       = "feature.position_sync"
	FeaturePortfolioSnapshot  = "feature.portfolio_snapshot"
	FeaturePositionManager    = "feature.position_manager"
	FeatureDailyStats         = "feature.daily_stats"
	FeatureMarketReview       = "feature.market_review"
	FeatureSignalBinanceWS    = "feature.signal.binance_ws"
	FeatureSignalBinancePrice = "feature.signal.binance_price"
	FeatureSignalWeatherAPI   = "feature.signal.weather_api"
	FeatureSignalPriceChange  = "feature.signal.price_change"
	FeatureSignalOrderbook    = "feature.signal.orderbook_pattern"
	FeatureSignalCertainty    = "feature.signal.certainty_sweep"
)

func DefaultFeatureSwitches() map[string]bool {
	return map[string]bool{
		FeatureCatalogSync:        true,
		FeatureCLOBStream:         true,
		FeatureStrategyEngine:     true,
		FeatureLabeler:            true,
		FeatureSettlementIngest:   true,
		FeatureAutoExecutor:       false,
		FeaturePositionSync:       true,
		FeaturePortfolioSnapshot:  true,
		FeaturePositionManager:    false,
		FeatureDailyStats:         true,
		FeatureMarketReview:       true,
		FeatureSignalBinanceWS:    false,
		FeatureSignalBinancePrice: false,
		FeatureSignalWeatherAPI:   false,
		FeatureSignalPriceChange:  true,  // internal DB poller — feeds news_alpha, volatility_spread
		FeatureSignalOrderbook:    true,  // internal DB poller — feeds fear_spike, mm_inventory_skew
		FeatureSignalCertainty:    true,  // internal DB poller — feeds certainty_sweep
	}
}

type SystemSettingsService struct {
	Repo repository.Repository
}

func (s *SystemSettingsService) EnsureDefaultSwitches(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	now := time.Now().UTC()
	for key, enabled := range DefaultFeatureSwitches() {
		existing, err := s.Repo.GetSystemSettingByKey(ctx, key)
		if err != nil {
			return err
		}
		if existing != nil {
			// Upgrade OFF → ON: if the default is now true but the stored
			// value is false, update it. Never turn an ON switch OFF.
			if enabled {
				var current bool
				if err := json.Unmarshal(existing.Value, &current); err == nil && !current {
					raw, _ := json.Marshal(true)
					existing.Value = datatypes.JSON(raw)
					existing.UpdatedAt = now
					if err := s.Repo.UpsertSystemSetting(ctx, existing); err != nil {
						return err
					}
				}
			}
			continue
		}
		raw, _ := json.Marshal(enabled)
		item := &models.SystemSetting{
			Key:         key,
			Value:       datatypes.JSON(raw),
			Description: "feature switch",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.Repo.UpsertSystemSetting(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *SystemSettingsService) IsEnabled(ctx context.Context, key string, fallback bool) bool {
	if s == nil || s.Repo == nil {
		return fallback
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fallback
	}
	item, err := s.Repo.GetSystemSettingByKey(ctx, key)
	if err != nil || item == nil || len(item.Value) == 0 {
		return fallback
	}
	var enabled bool
	if err := json.Unmarshal(item.Value, &enabled); err != nil {
		return fallback
	}
	return enabled
}

func (s *SystemSettingsService) SetEnabled(ctx context.Context, key string, enabled bool) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	raw, _ := json.Marshal(enabled)
	item := &models.SystemSetting{
		Key:         key,
		Value:       datatypes.JSON(raw),
		Description: "feature switch",
		UpdatedAt:   time.Now().UTC(),
	}
	return s.Repo.UpsertSystemSetting(ctx, item)
}
