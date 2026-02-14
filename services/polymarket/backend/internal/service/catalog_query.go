package service

import (
	"context"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type CatalogQueryService struct {
	Repo repository.CatalogRepository
}

type CatalogEventsResult struct {
	Items []models.Event
	Total int64
}

type CatalogMarketsResult struct {
	Items []models.Market
	Total int64
}

type CatalogTokensResult struct {
	Items []models.Token
	Total int64
}

func (s *CatalogQueryService) ListEvents(ctx context.Context, params repository.ListEventsParams) (CatalogEventsResult, error) {
	total, err := s.Repo.CountEvents(ctx, params)
	if err != nil {
		return CatalogEventsResult{}, err
	}
	items, err := s.Repo.ListEvents(ctx, params)
	if err != nil {
		return CatalogEventsResult{}, err
	}
	return CatalogEventsResult{Items: items, Total: total}, nil
}

func (s *CatalogQueryService) ListMarkets(ctx context.Context, params repository.ListMarketsParams) (CatalogMarketsResult, error) {
	total, err := s.Repo.CountMarkets(ctx, params)
	if err != nil {
		return CatalogMarketsResult{}, err
	}
	items, err := s.Repo.ListMarkets(ctx, params)
	if err != nil {
		return CatalogMarketsResult{}, err
	}
	return CatalogMarketsResult{Items: items, Total: total}, nil
}

func (s *CatalogQueryService) ListTokens(ctx context.Context, params repository.ListTokensParams) (CatalogTokensResult, error) {
	total, err := s.Repo.CountTokens(ctx, params)
	if err != nil {
		return CatalogTokensResult{}, err
	}
	items, err := s.Repo.ListTokens(ctx, params)
	if err != nil {
		return CatalogTokensResult{}, err
	}
	return CatalogTokensResult{Items: items, Total: total}, nil
}
