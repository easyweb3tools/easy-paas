package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/repository"
)

type DailyStatsService struct {
	Repo   repository.Repository
	Logger *zap.Logger
	Flags  *SystemSettingsService
}

func (s *DailyStatsService) Run(ctx context.Context, interval time.Duration) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := s.RunOnce(ctx); err != nil && s.Logger != nil {
			s.Logger.Warn("daily stats run failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (s *DailyStatsService) RunOnce(ctx context.Context) error {
	if s == nil || s.Repo == nil {
		return nil
	}
	if s.Flags != nil && !s.Flags.IsEnabled(ctx, FeatureDailyStats, true) {
		return nil
	}
	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)
	_, err := s.Repo.RebuildStrategyDailyStats(ctx, &since, nil)
	return err
}
