package gormrepository

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).Transaction(fn)
}

// --- L4-L6 (V2) -------------------------------------------------------------

func (s *Store) InsertSignal(ctx context.Context, item *models.Signal) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) ListSignals(ctx context.Context, params repository.ListSignalsParams) ([]models.Signal, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Signal{})
	if params.Type != nil && strings.TrimSpace(*params.Type) != "" {
		query = query.Where("signal_type = ?", strings.TrimSpace(*params.Type))
	}
	if params.Source != nil && strings.TrimSpace(*params.Source) != "" {
		query = query.Where("source = ?", strings.TrimSpace(*params.Source))
	}
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("created_at >= ?", *params.Since)
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.Signal
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) DeleteExpiredSignals(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if before.IsZero() {
		before = time.Now().UTC()
	}
	res := s.db.WithContext(ctx).
		Where("expires_at IS NOT NULL").
		Where("expires_at < ?", before).
		Delete(&models.Signal{})
	return res.RowsAffected, res.Error
}

func (s *Store) UpsertSignalSource(ctx context.Context, item *models.SignalSource) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	if strings.TrimSpace(item.Name) == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_type",
			"endpoint",
			"poll_interval",
			"enabled",
			"last_poll_at",
			"last_error",
			"health_status",
			"config",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) ListSignalSources(ctx context.Context) ([]models.SignalSource, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var items []models.SignalSource
	if err := s.db.WithContext(ctx).
		Model(&models.SignalSource{}).
		Order("name asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListMarketDataHealthCandidates(ctx context.Context, limit int, minSpreadBps float64) ([]models.MarketDataHealth, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	limit = normalizeLimit(limit, 200)
	query := s.db.WithContext(ctx).
		Model(&models.MarketDataHealth{}).
		Where("ws_connected = ?", true).
		Where("stale = ?", false).
		Where("needs_resync = ?", false).
		Where("spread_bps IS NOT NULL")
	if minSpreadBps > 0 {
		query = query.Where("spread_bps >= ?", minSpreadBps)
	}
	var items []models.MarketDataHealth
	if err := query.Order("spread_bps desc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListYesTokenJumpCandidates(ctx context.Context, limit int, minJumpBps float64, maxSpreadBps float64) ([]repository.TokenJumpCandidate, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	limit = normalizeLimit(limit, 200)
	query := s.db.WithContext(ctx).
		Table("market_data_health AS h").
		Select(`
			h.token_id AS token_id,
			t.market_id AS market_id,
			COALESCE(h.price_jump_bps,0) AS price_jump_bps,
			COALESCE(h.spread_bps,0) AS spread_bps,
			h.updated_at AS updated_at
		`).
		Joins("JOIN catalog_tokens AS t ON t.id = h.token_id").
		Where("h.ws_connected = ?", true).
		Where("h.stale = ?", false).
		Where("h.needs_resync = ?", false).
		Where("h.price_jump_bps IS NOT NULL").
		Where("LOWER(t.outcome) = 'yes'")
	if minJumpBps > 0 {
		query = query.Where("h.price_jump_bps >= ?", minJumpBps)
	}
	if maxSpreadBps > 0 {
		query = query.Where("h.spread_bps IS NOT NULL AND h.spread_bps <= ?", maxSpreadBps)
	}
	var rows []repository.TokenJumpCandidate
	if err := query.Order("h.price_jump_bps desc").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) UpsertStrategy(ctx context.Context, item *models.Strategy) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	if strings.TrimSpace(item.Name) == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"display_name",
			"description",
			"category",
			"enabled",
			"priority",
			"params",
			"required_signals",
			"stats",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) GetStrategyByName(ctx context.Context, name string) (*models.Strategy, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	var item models.Strategy
	err := s.db.WithContext(ctx).Model(&models.Strategy{}).Where("name = ?", name).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListStrategies(ctx context.Context) ([]models.Strategy, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var items []models.Strategy
	if err := s.db.WithContext(ctx).
		Model(&models.Strategy{}).
		Order("priority asc, name asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) SetStrategyEnabled(ctx context.Context, name string, enabled bool) error {
	if s == nil || s.db == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&models.Strategy{}).
		Where("name = ?", name).
		Updates(map[string]any{"enabled": enabled, "updated_at": time.Now().UTC()}).
		Error
}

func (s *Store) UpdateStrategyParams(ctx context.Context, name string, params []byte) error {
	if s == nil || s.db == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&models.Strategy{}).
		Where("name = ?", name).
		Updates(map[string]any{"params": params, "updated_at": time.Now().UTC()}).
		Error
}

func (s *Store) UpdateStrategyStats(ctx context.Context, name string, stats []byte) error {
	if s == nil || s.db == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&models.Strategy{}).
		Where("name = ?", name).
		Updates(map[string]any{"stats": stats, "updated_at": time.Now().UTC()}).
		Error
}

func (s *Store) InsertOpportunity(ctx context.Context, item *models.Opportunity) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) UpsertActiveOpportunity(ctx context.Context, item *models.Opportunity) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	if item.StrategyID == 0 {
		return s.InsertOpportunity(ctx, item)
	}

	keyEventID := ""
	if item.EventID != nil {
		keyEventID = strings.TrimSpace(*item.EventID)
	}
	keyMarketID := ""
	if item.PrimaryMarketID != nil {
		keyMarketID = strings.TrimSpace(*item.PrimaryMarketID)
	}
	if keyEventID == "" && keyMarketID == "" {
		return s.InsertOpportunity(ctx, item)
	}

	var existing models.Opportunity
	query := s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("strategy_id = ?", item.StrategyID).
		Where("status = ?", "active")
	if keyEventID != "" {
		query = query.Where("event_id = ?", keyEventID)
	} else {
		query = query.Where("primary_market_id = ?", keyMarketID)
	}
	err := query.Order("created_at desc").First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		return s.InsertOpportunity(ctx, item)
	}
	// Update core fields in-place, keep status/strategy/event stable.
	updates := map[string]any{
		"primary_market_id": item.PrimaryMarketID,
		"market_ids":        item.MarketIDs,
		"edge_pct":          item.EdgePct,
		"edge_usd":          item.EdgeUSD,
		"max_size":          item.MaxSize,
		"confidence":        item.Confidence,
		"risk_score":        item.RiskScore,
		"decay_type":        item.DecayType,
		"expires_at":        item.ExpiresAt,
		"legs":              item.Legs,
		"signal_ids":        item.SignalIDs,
		"reasoning":         item.Reasoning,
		"data_age_ms":       item.DataAgeMs,
		"warnings":          item.Warnings,
		"updated_at":        time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("id = ?", existing.ID).
		Updates(updates).Error
}

func (s *Store) GetOpportunityByID(ctx context.Context, id uint64) (*models.Opportunity, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if id == 0 {
		return nil, nil
	}
	var item models.Opportunity
	err := s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Preload("Strategy").
		Where("id = ?", id).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListOpportunities(ctx context.Context, params repository.ListOpportunitiesParams) ([]models.Opportunity, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Opportunity{}).Preload("Strategy")
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	needsStratJoin := (params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "") ||
		(params.Category != nil && strings.TrimSpace(*params.Category) != "")
	if needsStratJoin {
		query = query.Joins("JOIN strategies ON strategies.id = opportunities.strategy_id").
			Where("1 = 1")
		if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
			query = query.Where("strategies.name = ?", strings.TrimSpace(*params.StrategyName))
		}
		if params.Category != nil && strings.TrimSpace(*params.Category) != "" {
			query = query.Where("strategies.category = ?", strings.TrimSpace(*params.Category))
		}
	}
	if params.MinEdgePct != nil {
		query = query.Where("edge_pct >= ?", *params.MinEdgePct)
	}
	if params.MinConfidence != nil {
		query = query.Where("confidence >= ?", *params.MinConfidence)
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.Opportunity
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountOpportunities(ctx context.Context, params repository.ListOpportunitiesParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Opportunity{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	needsStratJoin := (params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "") ||
		(params.Category != nil && strings.TrimSpace(*params.Category) != "")
	if needsStratJoin {
		query = query.Joins("JOIN strategies ON strategies.id = opportunities.strategy_id").
			Where("1 = 1")
		if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
			query = query.Where("strategies.name = ?", strings.TrimSpace(*params.StrategyName))
		}
		if params.Category != nil && strings.TrimSpace(*params.Category) != "" {
			query = query.Where("strategies.category = ?", strings.TrimSpace(*params.Category))
		}
	}
	if params.MinEdgePct != nil {
		query = query.Where("edge_pct >= ?", *params.MinEdgePct)
	}
	if params.MinConfidence != nil {
		query = query.Where("confidence >= ?", *params.MinConfidence)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) UpdateOpportunityStatus(ctx context.Context, id uint64, status string) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 || strings.TrimSpace(status) == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": strings.TrimSpace(status), "updated_at": time.Now().UTC()}).
		Error
}

func (s *Store) ExpireDueOpportunities(ctx context.Context, now time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	res := s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("status = ?", "active").
		Where("expires_at IS NOT NULL").
		Where("expires_at < ?", now).
		Updates(map[string]any{"status": "expired", "updated_at": time.Now().UTC()})
	return res.RowsAffected, res.Error
}

func (s *Store) CountActiveOpportunities(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var total int64
	err := s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("status = ?", "active").
		Count(&total).Error
	return total, err
}

func (s *Store) ListOldestActiveOpportunityIDs(ctx context.Context, limit int) ([]uint64, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	limit = normalizeLimit(limit, 200)
	var ids []uint64
	err := s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("status = ?", "active").
		Order("created_at asc").
		Limit(limit).
		Pluck("id", &ids).Error
	return ids, err
}

func (s *Store) BulkUpdateOpportunityStatus(ctx context.Context, ids []uint64, status string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if len(ids) == 0 || strings.TrimSpace(status) == "" {
		return 0, nil
	}
	res := s.db.WithContext(ctx).
		Model(&models.Opportunity{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"status": strings.TrimSpace(status), "updated_at": time.Now().UTC()})
	return res.RowsAffected, res.Error
}

func (s *Store) UpsertMarketLabel(ctx context.Context, item *models.MarketLabel) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	if strings.TrimSpace(item.MarketID) == "" || strings.TrimSpace(item.Label) == "" {
		return nil
	}
	// Uniqueness is enforced by uniq_market_label (market_id, label).
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "market_id"}, {Name: "label"}},
		DoNothing: true,
	}).Create(item).Error
}

func (s *Store) ListMarketLabels(ctx context.Context, params repository.ListMarketLabelsParams) ([]models.MarketLabel, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.MarketLabel{})
	if params.MarketID != nil && strings.TrimSpace(*params.MarketID) != "" {
		query = query.Where("market_id = ?", strings.TrimSpace(*params.MarketID))
	}
	if params.Label != nil && strings.TrimSpace(*params.Label) != "" {
		query = query.Where("label = ?", strings.TrimSpace(*params.Label))
	}
	if params.SubLabel != nil && strings.TrimSpace(*params.SubLabel) != "" {
		query = query.Where("sub_label = ?", strings.TrimSpace(*params.SubLabel))
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
	limit := normalizeLimit(params.Limit, 500)
	offset := normalizeOffset(params.Offset)
	var items []models.MarketLabel
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) DeleteMarketLabel(ctx context.Context, marketID string, label string) error {
	if s == nil || s.db == nil {
		return nil
	}
	marketID = strings.TrimSpace(marketID)
	label = strings.TrimSpace(label)
	if marketID == "" || label == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Where("market_id = ? AND label = ?", marketID, label).
		Delete(&models.MarketLabel{}).Error
}

func (s *Store) ListTagsByEventIDs(ctx context.Context, eventIDs []string) (map[string][]models.Tag, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(eventIDs) == 0 {
		return map[string][]models.Tag{}, nil
	}
	type row struct {
		EventID string
		Tag     models.Tag
	}
	var rows []struct {
		EventID string
		ID      string
		Label   string
		Slug    string
	}
	if err := s.db.WithContext(ctx).
		Table("catalog_event_tags AS et").
		Select("et.event_id AS event_id, t.id AS id, t.label AS label, t.slug AS slug").
		Joins("JOIN catalog_tags AS t ON t.id = et.tag_id").
		Where("et.event_id IN ?", eventIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string][]models.Tag{}
	for _, r := range rows {
		out[r.EventID] = append(out[r.EventID], models.Tag{ID: r.ID, Label: r.Label, Slug: r.Slug})
	}
	return out, nil
}

// --- Execution & Analytics (L6) ---------------------------------------------

func (s *Store) InsertExecutionPlan(ctx context.Context, item *models.ExecutionPlan) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) GetExecutionPlanByID(ctx context.Context, id uint64) (*models.ExecutionPlan, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if id == 0 {
		return nil, nil
	}
	var item models.ExecutionPlan
	err := s.db.WithContext(ctx).Model(&models.ExecutionPlan{}).Where("id = ?", id).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListExecutionPlans(ctx context.Context, params repository.ListExecutionPlansParams) ([]models.ExecutionPlan, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.ExecutionPlan{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.ExecutionPlan
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountExecutionPlans(ctx context.Context, params repository.ListExecutionPlansParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.ExecutionPlan{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListExecutionPlansByStatuses(ctx context.Context, statuses []string, limit int) ([]models.ExecutionPlan, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	statuses = cleanStrings(statuses)
	if len(statuses) == 0 {
		return nil, nil
	}
	limit = normalizeLimit(limit, 5000)
	var items []models.ExecutionPlan
	if err := s.db.WithContext(ctx).
		Model(&models.ExecutionPlan{}).
		Where("status IN ?", statuses).
		Order("created_at desc").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) UpdateExecutionPlanStatus(ctx context.Context, id uint64, status string) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 || strings.TrimSpace(status) == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Model(&models.ExecutionPlan{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": strings.TrimSpace(status), "updated_at": time.Now().UTC()}).
		Error
}

func (s *Store) UpdateExecutionPlanPreflight(ctx context.Context, id uint64, status string, preflightResult []byte) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 || strings.TrimSpace(status) == "" {
		return nil
	}
	updates := map[string]any{
		"status":           strings.TrimSpace(status),
		"preflight_result": preflightResult,
		"updated_at":       time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Model(&models.ExecutionPlan{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) UpdateExecutionPlanExecutedAt(ctx context.Context, id uint64, status string, executedAt *time.Time) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 || strings.TrimSpace(status) == "" {
		return nil
	}
	updates := map[string]any{
		"status":      strings.TrimSpace(status),
		"executed_at": executedAt,
		"updated_at":  time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Model(&models.ExecutionPlan{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (s *Store) CountExecutionPlansByStrategySince(ctx context.Context, strategyName string, since time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	strategyName = strings.TrimSpace(strategyName)
	if strategyName == "" || since.IsZero() {
		return 0, nil
	}
	query := s.db.WithContext(ctx).
		Model(&models.ExecutionPlan{}).
		Where("strategy_name = ?", strategyName).
		Where("created_at >= ?", since.UTC())
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) InsertFill(ctx context.Context, item *models.Fill) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) ListFillsByPlanID(ctx context.Context, planID uint64) ([]models.Fill, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if planID == 0 {
		return nil, nil
	}
	var items []models.Fill
	if err := s.db.WithContext(ctx).
		Model(&models.Fill{}).
		Where("plan_id = ?", planID).
		Order("filled_at asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) UpsertPnLRecord(ctx context.Context, item *models.PnLRecord) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	if item.PlanID == 0 {
		return s.db.WithContext(ctx).Create(item).Error
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "plan_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"strategy_name", "expected_edge", "realized_pnl", "realized_roi", "slippage_loss", "outcome", "failure_reason", "settled_at", "notes"}),
	}).Create(item).Error
}

func (s *Store) GetPnLRecordByPlanID(ctx context.Context, planID uint64) (*models.PnLRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if planID == 0 {
		return nil, nil
	}
	var item models.PnLRecord
	err := s.db.WithContext(ctx).
		Where("plan_id = ?", planID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) SumRealizedPnLSince(ctx context.Context, since time.Time) (decimal.Decimal, error) {
	if s == nil || s.db == nil {
		return decimal.Zero, nil
	}
	if since.IsZero() {
		return decimal.Zero, nil
	}
	var out float64
	err := s.db.WithContext(ctx).
		Table("pnl_records").
		Select("COALESCE(SUM(COALESCE(realized_pnl,0)),0)").
		Where("created_at >= ?", since.UTC()).
		Scan(&out).Error
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromFloat(out), nil
}

func (s *Store) UpsertExecutionRule(ctx context.Context, item *models.ExecutionRule) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	item.StrategyName = strings.TrimSpace(item.StrategyName)
	if item.StrategyName == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "strategy_name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"auto_execute",
			"min_confidence",
			"min_edge_pct",
			"stop_loss_pct",
			"take_profit_pct",
			"max_hold_hours",
			"max_daily_trades",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) GetExecutionRuleByStrategyName(ctx context.Context, strategyName string) (*models.ExecutionRule, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	strategyName = strings.TrimSpace(strategyName)
	if strategyName == "" {
		return nil, nil
	}
	var item models.ExecutionRule
	err := s.db.WithContext(ctx).
		Model(&models.ExecutionRule{}).
		Where("strategy_name = ?", strategyName).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListExecutionRules(ctx context.Context) ([]models.ExecutionRule, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var items []models.ExecutionRule
	if err := s.db.WithContext(ctx).
		Model(&models.ExecutionRule{}).
		Order("strategy_name asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) DeleteExecutionRuleByStrategyName(ctx context.Context, strategyName string) error {
	if s == nil || s.db == nil {
		return nil
	}
	strategyName = strings.TrimSpace(strategyName)
	if strategyName == "" {
		return nil
	}
	return s.db.WithContext(ctx).
		Where("strategy_name = ?", strategyName).
		Delete(&models.ExecutionRule{}).Error
}

func (s *Store) InsertTradeJournal(ctx context.Context, item *models.TradeJournal) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) GetTradeJournalByPlanID(ctx context.Context, planID uint64) (*models.TradeJournal, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if planID == 0 {
		return nil, nil
	}
	var item models.TradeJournal
	err := s.db.WithContext(ctx).
		Model(&models.TradeJournal{}).
		Where("execution_plan_id = ?", planID).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) UpdateTradeJournalExit(ctx context.Context, planID uint64, updates map[string]any) error {
	if s == nil || s.db == nil {
		return nil
	}
	if planID == 0 || len(updates) == 0 {
		return nil
	}
	updates["updated_at"] = time.Now().UTC()
	return s.db.WithContext(ctx).
		Model(&models.TradeJournal{}).
		Where("execution_plan_id = ?", planID).
		Updates(updates).Error
}

func (s *Store) UpdateTradeJournalNotes(ctx context.Context, planID uint64, notes string, tags []byte, reviewedAt *time.Time) error {
	if s == nil || s.db == nil {
		return nil
	}
	if planID == 0 {
		return nil
	}
	updates := map[string]any{
		"notes":       strings.TrimSpace(notes),
		"tags":        tags,
		"reviewed_at": reviewedAt,
		"updated_at":  time.Now().UTC(),
	}
	return s.db.WithContext(ctx).
		Model(&models.TradeJournal{}).
		Where("execution_plan_id = ?", planID).
		Updates(updates).Error
}

func (s *Store) ListTradeJournals(ctx context.Context, params repository.ListTradeJournalParams) ([]models.TradeJournal, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.TradeJournal{})
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.Outcome != nil && strings.TrimSpace(*params.Outcome) != "" {
		query = query.Where("outcome = ?", strings.TrimSpace(*params.Outcome))
	}
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("created_at >= ?", params.Since.UTC())
	}
	if params.Until != nil && !params.Until.IsZero() {
		query = query.Where("created_at <= ?", params.Until.UTC())
	}
	for _, tag := range cleanStrings(params.Tags) {
		like := "%" + tag + "%"
		query = query.Where("CAST(tags AS TEXT) LIKE ?", like)
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.TradeJournal
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountTradeJournals(ctx context.Context, params repository.ListTradeJournalParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.TradeJournal{})
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.Outcome != nil && strings.TrimSpace(*params.Outcome) != "" {
		query = query.Where("outcome = ?", strings.TrimSpace(*params.Outcome))
	}
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("created_at >= ?", params.Since.UTC())
	}
	if params.Until != nil && !params.Until.IsZero() {
		query = query.Where("created_at <= ?", params.Until.UTC())
	}
	for _, tag := range cleanStrings(params.Tags) {
		like := "%" + tag + "%"
		query = query.Where("CAST(tags AS TEXT) LIKE ?", like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) UpsertSystemSetting(ctx context.Context, item *models.SystemSetting) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	item.Key = strings.TrimSpace(item.Key)
	if item.Key == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"value",
			"description",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) GetSystemSettingByKey(ctx context.Context, key string) (*models.SystemSetting, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	var item models.SystemSetting
	err := s.db.WithContext(ctx).Model(&models.SystemSetting{}).Where("key = ?", key).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListSystemSettings(ctx context.Context, params repository.ListSystemSettingsParams) ([]models.SystemSetting, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.SystemSetting{})
	if params.Prefix != nil && strings.TrimSpace(*params.Prefix) != "" {
		pattern := strings.TrimSpace(*params.Prefix) + "%"
		query = query.Where("key LIKE ?", pattern)
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "key")
	limit := normalizeLimit(params.Limit, 500)
	offset := normalizeOffset(params.Offset)
	var items []models.SystemSetting
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountSystemSettings(ctx context.Context, params repository.ListSystemSettingsParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.SystemSetting{})
	if params.Prefix != nil && strings.TrimSpace(*params.Prefix) != "" {
		pattern := strings.TrimSpace(*params.Prefix) + "%"
		query = query.Where("key LIKE ?", pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) UpsertPosition(ctx context.Context, item *models.Position) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	item.TokenID = strings.TrimSpace(item.TokenID)
	if item.TokenID == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"market_id",
			"event_id",
			"direction",
			"quantity",
			"avg_entry_price",
			"current_price",
			"cost_basis",
			"unrealized_pnl",
			"realized_pnl",
			"status",
			"strategy_name",
			"opened_at",
			"closed_at",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) GetPositionByID(ctx context.Context, id uint64) (*models.Position, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if id == 0 {
		return nil, nil
	}
	var item models.Position
	err := s.db.WithContext(ctx).Model(&models.Position{}).Where("id = ?", id).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) GetPositionByTokenID(ctx context.Context, tokenID string) (*models.Position, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, nil
	}
	var item models.Position
	err := s.db.WithContext(ctx).Model(&models.Position{}).Where("token_id = ?", tokenID).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListPositions(ctx context.Context, params repository.ListPositionsParams) ([]models.Position, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Position{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.MarketID != nil && strings.TrimSpace(*params.MarketID) != "" {
		query = query.Where("market_id = ?", strings.TrimSpace(*params.MarketID))
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "opened_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.Position
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountPositions(ctx context.Context, params repository.ListPositionsParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Position{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.MarketID != nil && strings.TrimSpace(*params.MarketID) != "" {
		query = query.Where("market_id = ?", strings.TrimSpace(*params.MarketID))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListOpenPositions(ctx context.Context) ([]models.Position, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var items []models.Position
	if err := s.db.WithContext(ctx).Model(&models.Position{}).
		Where("status = ?", "open").
		Order("opened_at asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ClosePosition(ctx context.Context, id uint64, realizedPnL decimal.Decimal, closedAt time.Time) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 {
		return nil
	}
	if closedAt.IsZero() {
		closedAt = time.Now().UTC()
	}
	return s.db.WithContext(ctx).Model(&models.Position{}).Where("id = ?", id).Updates(map[string]any{
		"status":         "closed",
		"closed_at":      &closedAt,
		"quantity":       decimal.Zero,
		"cost_basis":     decimal.Zero,
		"unrealized_pnl": decimal.Zero,
		"realized_pnl":   realizedPnL,
		"updated_at":     time.Now().UTC(),
	}).Error
}

func (s *Store) PositionsSummary(ctx context.Context) (repository.PositionsSummary, error) {
	if s == nil || s.db == nil {
		return repository.PositionsSummary{}, nil
	}
	var row struct {
		TotalOpen      int64
		TotalCostBasis float64
		TotalMarketVal float64
		UnrealizedPnL  float64
		RealizedPnL    float64
	}
	err := s.db.WithContext(ctx).
		Table("positions").
		Select(`
			COALESCE(SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END),0) AS total_open,
			COALESCE(SUM(CASE WHEN status = 'open' THEN cost_basis ELSE 0 END),0) AS total_cost_basis,
			COALESCE(SUM(CASE WHEN status = 'open' THEN (current_price * quantity) ELSE 0 END),0) AS total_market_val,
			COALESCE(SUM(CASE WHEN status = 'open' THEN unrealized_pnl ELSE 0 END),0) AS unrealized_pnl,
			COALESCE(SUM(realized_pnl),0) AS realized_pnl
		`).
		Scan(&row).Error
	if err != nil {
		return repository.PositionsSummary{}, err
	}
	return repository.PositionsSummary{
		TotalOpen:      row.TotalOpen,
		TotalCostBasis: row.TotalCostBasis,
		TotalMarketVal: row.TotalMarketVal,
		UnrealizedPnL:  row.UnrealizedPnL,
		RealizedPnL:    row.RealizedPnL,
		NetLiquidation: row.TotalMarketVal + row.RealizedPnL,
	}, nil
}

func (s *Store) InsertPortfolioSnapshot(ctx context.Context, item *models.PortfolioSnapshot) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "snapshot_at"}},
		DoUpdates: clause.AssignmentColumns([]string{"total_positions", "total_cost_basis", "total_market_val", "unrealized_pnl", "realized_pnl", "net_liquidation"}),
	}).Create(item).Error
}

func (s *Store) ListPortfolioSnapshots(ctx context.Context, params repository.ListPortfolioSnapshotsParams) ([]models.PortfolioSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.PortfolioSnapshot{})
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("snapshot_at >= ?", params.Since.UTC())
	}
	if params.Until != nil && !params.Until.IsZero() {
		query = query.Where("snapshot_at <= ?", params.Until.UTC())
	}
	limit := normalizeLimit(params.Limit, 500)
	offset := normalizeOffset(params.Offset)
	var items []models.PortfolioSnapshot
	if err := query.Order("snapshot_at desc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) InsertOrder(ctx context.Context, item *models.Order) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) GetOrderByID(ctx context.Context, id uint64) (*models.Order, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if id == 0 {
		return nil, nil
	}
	var item models.Order
	err := s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", id).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListOrders(ctx context.Context, params repository.ListOrdersParams) ([]models.Order, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Order{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	if params.PlanID != nil && *params.PlanID > 0 {
		query = query.Where("plan_id = ?", *params.PlanID)
	}
	if params.TokenID != nil && strings.TrimSpace(*params.TokenID) != "" {
		query = query.Where("token_id = ?", strings.TrimSpace(*params.TokenID))
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "created_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.Order
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountOrders(ctx context.Context, params repository.ListOrdersParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Order{})
	if params.Status != nil && strings.TrimSpace(*params.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(*params.Status))
	}
	if params.PlanID != nil && *params.PlanID > 0 {
		query = query.Where("plan_id = ?", *params.PlanID)
	}
	if params.TokenID != nil && strings.TrimSpace(*params.TokenID) != "" {
		query = query.Where("token_id = ?", strings.TrimSpace(*params.TokenID))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) UpdateOrderStatus(ctx context.Context, id uint64, status string, updates map[string]any) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 || strings.TrimSpace(status) == "" {
		return nil
	}
	next := map[string]any{
		"status":     strings.TrimSpace(status),
		"updated_at": time.Now().UTC(),
	}
	for k, v := range updates {
		next[k] = v
	}
	return s.db.WithContext(ctx).Model(&models.Order{}).Where("id = ?", id).Updates(next).Error
}

func (s *Store) UpsertStrategyDailyStats(ctx context.Context, item *models.StrategyDailyStats) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	item.StrategyName = strings.TrimSpace(item.StrategyName)
	if item.StrategyName == "" || item.Date.IsZero() {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "strategy_name"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"trades_count",
			"win_count",
			"loss_count",
			"pnl_usd",
			"avg_edge_pct",
			"avg_slippage_bps",
			"avg_hold_hours",
			"max_drawdown_usd",
			"cumulative_pnl",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) ListStrategyDailyStats(ctx context.Context, params repository.ListDailyStatsParams) ([]models.StrategyDailyStats, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.StrategyDailyStats{})
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("date >= ?", params.Since.UTC())
	}
	if params.Until != nil && !params.Until.IsZero() {
		query = query.Where("date <= ?", params.Until.UTC())
	}
	limit := normalizeLimit(params.Limit, 500)
	offset := normalizeOffset(params.Offset)
	var items []models.StrategyDailyStats
	if err := query.Order("date desc, strategy_name asc").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) AttributionByStrategy(ctx context.Context, strategyName string, since, until *time.Time) (repository.AttributionResult, error) {
	if s == nil || s.db == nil {
		return repository.AttributionResult{}, nil
	}
	strategyName = strings.TrimSpace(strategyName)
	if strategyName == "" {
		return repository.AttributionResult{}, nil
	}
	query := s.db.WithContext(ctx).Table("pnl_records").Where("strategy_name = ?", strategyName)
	if since != nil && !since.IsZero() {
		query = query.Where("created_at >= ?", since.UTC())
	}
	if until != nil && !until.IsZero() {
		query = query.Where("created_at <= ?", until.UTC())
	}
	var row struct {
		Edge     float64
		Slippage float64
		Net      float64
	}
	if err := query.Select(`
		COALESCE(SUM(COALESCE(expected_edge,0)),0) AS edge,
		COALESCE(SUM(COALESCE(slippage_loss,0)),0) AS slippage,
		COALESCE(SUM(COALESCE(realized_pnl,0)),0) AS net
	`).Scan(&row).Error; err != nil {
		return repository.AttributionResult{}, err
	}

	feeQuery := s.db.WithContext(ctx).
		Table("fills AS f").
		Select("COALESCE(SUM(COALESCE(f.fee,0)),0)").
		Joins("JOIN execution_plans AS p ON p.id = f.plan_id").
		Where("p.strategy_name = ?", strategyName)
	if since != nil && !since.IsZero() {
		feeQuery = feeQuery.Where("f.created_at >= ?", since.UTC())
	}
	if until != nil && !until.IsZero() {
		feeQuery = feeQuery.Where("f.created_at <= ?", until.UTC())
	}
	var fee float64
	if err := feeQuery.Scan(&fee).Error; err != nil {
		return repository.AttributionResult{}, err
	}
	timing := row.Net - row.Edge + row.Slippage + fee
	return repository.AttributionResult{
		EdgeContribution: row.Edge,
		SlippageCost:     row.Slippage,
		FeeCost:          fee,
		TimingValue:      timing,
		NetPnL:           row.Net,
	}, nil
}

func (s *Store) PortfolioDrawdown(ctx context.Context) (repository.DrawdownResult, error) {
	if s == nil || s.db == nil {
		return repository.DrawdownResult{}, nil
	}
	var rows []struct {
		TS  *time.Time
		PnL float64
	}
	if err := s.db.WithContext(ctx).Table("pnl_records").
		Select("COALESCE(settled_at, created_at) AS ts, COALESCE(realized_pnl,0) AS pnl").
		Order("COALESCE(settled_at, created_at) asc").
		Scan(&rows).Error; err != nil {
		return repository.DrawdownResult{}, err
	}
	if len(rows) == 0 {
		return repository.DrawdownResult{}, nil
	}
	cum := 0.0
	peak := 0.0
	trough := 0.0
	maxDD := 0.0
	maxDDPct := 0.0
	curDD := 0.0
	var peakTime, troughTime *time.Time
	for _, r := range rows {
		cum += r.PnL
		if cum > peak || peakTime == nil {
			peak = cum
			t := time.Now().UTC()
			if r.TS != nil {
				t = r.TS.UTC()
			}
			peakTime = &t
		}
		dd := peak - cum
		if dd > maxDD {
			maxDD = dd
			trough = cum
			t := time.Now().UTC()
			if r.TS != nil {
				t = r.TS.UTC()
			}
			troughTime = &t
		}
		if peak > 0 {
			ddPct := dd / peak
			if ddPct > maxDDPct {
				maxDDPct = ddPct
			}
		}
		curDD = dd
	}
	ddDays := 0
	if peakTime != nil && troughTime != nil && troughTime.After(*peakTime) {
		ddDays = int(troughTime.Sub(*peakTime).Hours() / 24)
	}
	return repository.DrawdownResult{
		MaxDrawdownUSD:       maxDD,
		MaxDrawdownPct:       maxDDPct,
		DrawdownDurationDays: ddDays,
		CurrentDrawdownUSD:   curDD,
		PeakPnL:              peak,
		TroughPnL:            trough,
	}, nil
}

func (s *Store) StrategyCorrelation(ctx context.Context, since, until *time.Time) ([]repository.CorrelationRow, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Table("strategy_daily_stats")
	if since != nil && !since.IsZero() {
		query = query.Where("date >= ?", since.UTC())
	}
	if until != nil && !until.IsZero() {
		query = query.Where("date <= ?", until.UTC())
	}
	var rows []struct {
		Strategy string
		Date     time.Time
		PnL      float64
	}
	if err := query.Select("strategy_name AS strategy, date, COALESCE(pnl_usd,0) AS pnl").Scan(&rows).Error; err != nil {
		return nil, err
	}
	series := map[string]map[string]float64{}
	for _, r := range rows {
		if _, ok := series[r.Strategy]; !ok {
			series[r.Strategy] = map[string]float64{}
		}
		series[r.Strategy][r.Date.Format("2006-01-02")] = r.PnL
	}
	names := make([]string, 0, len(series))
	for n := range series {
		names = append(names, n)
	}
	out := make([]repository.CorrelationRow, 0)
	for i := 0; i < len(names); i++ {
		for j := i; j < len(names); j++ {
			a, b := names[i], names[j]
			corr := correlationOfMaps(series[a], series[b])
			out = append(out, repository.CorrelationRow{
				StrategyA:   a,
				StrategyB:   b,
				Correlation: corr,
			})
		}
	}
	return out, nil
}

func (s *Store) PerformanceRatios(ctx context.Context, since, until *time.Time) (repository.RatiosResult, error) {
	if s == nil || s.db == nil {
		return repository.RatiosResult{}, nil
	}
	query := s.db.WithContext(ctx).Table("pnl_records")
	if since != nil && !since.IsZero() {
		query = query.Where("created_at >= ?", since.UTC())
	}
	if until != nil && !until.IsZero() {
		query = query.Where("created_at <= ?", until.UTC())
	}
	var rows []struct {
		PnL float64
	}
	if err := query.Select("COALESCE(realized_pnl,0) AS pnl").Scan(&rows).Error; err != nil {
		return repository.RatiosResult{}, err
	}
	if len(rows) == 0 {
		return repository.RatiosResult{}, nil
	}
	return calcRatios(rows), nil
}

func (s *Store) RebuildStrategyDailyStats(ctx context.Context, since, until *time.Time) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Table("pnl_records AS r")
	if since != nil && !since.IsZero() {
		query = query.Where("COALESCE(r.settled_at, r.created_at) >= ?", since.UTC())
	}
	if until != nil && !until.IsZero() {
		query = query.Where("COALESCE(r.settled_at, r.created_at) <= ?", until.UTC())
	}
	var rows []struct {
		StrategyName   string
		Date           time.Time
		TradesCount    int
		WinCount       int
		LossCount      int
		PnLUSD         float64
		AvgEdgePct     float64
		AvgSlippageBps float64
		AvgHoldHours   float64
	}
	err := query.
		Select(`
			r.strategy_name AS strategy_name,
			DATE(COALESCE(r.settled_at, r.created_at)) AS date,
			COUNT(*) AS trades_count,
			COALESCE(SUM(CASE WHEN r.outcome = 'win' THEN 1 ELSE 0 END),0) AS win_count,
			COALESCE(SUM(CASE WHEN r.outcome = 'loss' THEN 1 ELSE 0 END),0) AS loss_count,
			COALESCE(SUM(COALESCE(r.realized_pnl,0)),0) AS pnl_usd,
			COALESCE(AVG(COALESCE(r.expected_edge,0)),0) AS avg_edge_pct,
			COALESCE(AVG(COALESCE(r.slippage_loss,0))*10000,0) AS avg_slippage_bps,
			COALESCE(AVG(EXTRACT(EPOCH FROM (p.executed_at - p.created_at))/3600.0),0) AS avg_hold_hours
		`).
		Joins("LEFT JOIN execution_plans AS p ON p.id = r.plan_id").
		Group("r.strategy_name, DATE(COALESCE(r.settled_at, r.created_at))").
		Order("r.strategy_name asc, DATE(COALESCE(r.settled_at, r.created_at)) asc").
		Scan(&rows).Error
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	cumByStrategy := map[string]float64{}
	peakByStrategy := map[string]float64{}
	updated := 0
	for _, r := range rows {
		name := strings.TrimSpace(r.StrategyName)
		if name == "" {
			continue
		}
		cum := cumByStrategy[name] + r.PnLUSD
		cumByStrategy[name] = cum
		peak := peakByStrategy[name]
		if cum > peak {
			peak = cum
			peakByStrategy[name] = peak
		}
		maxDD := peak - cum
		item := &models.StrategyDailyStats{
			StrategyName:   name,
			Date:           r.Date.UTC(),
			TradesCount:    r.TradesCount,
			WinCount:       r.WinCount,
			LossCount:      r.LossCount,
			PnLUSD:         decimal.NewFromFloat(r.PnLUSD),
			AvgEdgePct:     decimal.NewFromFloat(r.AvgEdgePct),
			AvgSlippageBps: decimal.NewFromFloat(r.AvgSlippageBps),
			AvgHoldHours:   decimal.NewFromFloat(r.AvgHoldHours),
			MaxDrawdownUSD: decimal.NewFromFloat(maxDD),
			CumulativePnL:  decimal.NewFromFloat(cum),
			UpdatedAt:      time.Now().UTC(),
		}
		if err := s.UpsertStrategyDailyStats(ctx, item); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

func correlationOfMaps(a, b map[string]float64) float64 {
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	x := make([]float64, 0, len(keys))
	y := make([]float64, 0, len(keys))
	for k := range keys {
		x = append(x, a[k])
		y = append(y, b[k])
	}
	if len(x) < 2 {
		return 0
	}
	mx := mean(x)
	my := mean(y)
	num := 0.0
	dx := 0.0
	dy := 0.0
	for i := range x {
		ax := x[i] - mx
		by := y[i] - my
		num += ax * by
		dx += ax * ax
		dy += by * by
	}
	if dx == 0 || dy == 0 {
		return 0
	}
	return num / math.Sqrt(dx*dy)
}

func calcRatios(rows []struct{ PnL float64 }) repository.RatiosResult {
	rets := make([]float64, 0, len(rows))
	win := 0
	loss := 0
	sumPos := 0.0
	sumNegAbs := 0.0
	sum := 0.0
	for _, r := range rows {
		rets = append(rets, r.PnL)
		sum += r.PnL
		if r.PnL > 0 {
			win++
			sumPos += r.PnL
		}
		if r.PnL < 0 {
			loss++
			sumNegAbs += -r.PnL
		}
	}
	m := mean(rets)
	std := stddev(rets, m)
	downside := downsideStd(rets, m)
	sharpe := 0.0
	sortino := 0.0
	if std > 0 {
		sharpe = m / std
	}
	if downside > 0 {
		sortino = m / downside
	}
	winRate := float64(win) / float64(len(rows))
	profitFactor := 0.0
	if sumNegAbs > 0 {
		profitFactor = sumPos / sumNegAbs
	}
	avgWin := 0.0
	if win > 0 {
		avgWin = sumPos / float64(win)
	}
	avgLoss := 0.0
	if loss > 0 {
		avgLoss = -sumNegAbs / float64(loss)
	}
	expectancy := sum / float64(len(rows))
	return repository.RatiosResult{
		SharpeRatio:  sharpe,
		SortinoRatio: sortino,
		WinRate:      winRate,
		ProfitFactor: profitFactor,
		AvgWin:       avgWin,
		AvgLoss:      avgLoss,
		Expectancy:   expectancy,
	}
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func stddev(v []float64, m float64) float64 {
	if len(v) < 2 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(v)))
}

func downsideStd(v []float64, target float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	n := 0
	for _, x := range v {
		if x >= target {
			continue
		}
		d := x - target
		s += d * d
		n++
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(s / float64(n))
}

func (s *Store) UpsertMarketSettlementHistory(ctx context.Context, item *models.MarketSettlementHistory) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	if strings.TrimSpace(item.MarketID) == "" || strings.TrimSpace(item.EventID) == "" || strings.TrimSpace(item.Outcome) == "" {
		return nil
	}
	// Uniqueness is enforced by unique index on market_id.
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "market_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"event_id",
			"question",
			"outcome",
			"category",
			"labels",
			"initial_yes_price",
			"final_yes_price",
			"settled_at",
		}),
	}).Create(item).Error
}

func (s *Store) ListMarketSettlementHistoryByMarketIDs(ctx context.Context, marketIDs []string) ([]models.MarketSettlementHistory, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	marketIDs = cleanStrings(marketIDs)
	if len(marketIDs) == 0 {
		return nil, nil
	}
	var items []models.MarketSettlementHistory
	if err := s.db.WithContext(ctx).
		Model(&models.MarketSettlementHistory{}).
		Where("market_id IN ?", marketIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListRecentMarketSettlementHistory(ctx context.Context, since time.Time, limit int) ([]models.MarketSettlementHistory, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if since.IsZero() {
		since = time.Now().UTC().Add(-24 * time.Hour)
	}
	limit = normalizeLimit(limit, 500)
	var items []models.MarketSettlementHistory
	if err := s.db.WithContext(ctx).
		Model(&models.MarketSettlementHistory{}).
		Where("settled_at >= ?", since.UTC()).
		Order("settled_at desc").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListLabelNoRateStats(ctx context.Context, labels []string) ([]repository.LabelNoRateRow, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	labels = cleanStrings(labels)
	query := s.db.WithContext(ctx).
		Table("market_settlement_history AS h").
		Select(`
			ml.label AS label,
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN h.outcome = 'NO' THEN 1 ELSE 0 END),0) AS no_count
		`).
		Joins("JOIN market_labels AS ml ON ml.market_id = h.market_id").
		Group("ml.label").
		Order("total desc")
	if len(labels) > 0 {
		query = query.Where("ml.label IN ?", labels)
	}
	var rows []struct {
		Label   string
		Total   int64
		NoCount int64
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]repository.LabelNoRateRow, 0, len(rows))
	for _, r := range rows {
		noRate := 0.0
		if r.Total > 0 {
			noRate = float64(r.NoCount) / float64(r.Total)
		}
		out = append(out, repository.LabelNoRateRow{
			Label:   r.Label,
			Total:   r.Total,
			NoCount: r.NoCount,
			NoRate:  noRate,
		})
	}
	return out, nil
}

func (s *Store) UpsertMarketReview(ctx context.Context, item *models.MarketReview) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	item.MarketID = strings.TrimSpace(item.MarketID)
	if item.MarketID == "" {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "market_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"event_id",
			"our_action",
			"opportunity_id",
			"strategy_name",
			"edge_at_entry",
			"final_outcome",
			"final_price",
			"hypothetical_pnl",
			"actual_pnl",
			"lesson_tags",
			"notes",
			"settled_at",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) GetMarketReviewByMarketID(ctx context.Context, marketID string) (*models.MarketReview, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	marketID = strings.TrimSpace(marketID)
	if marketID == "" {
		return nil, nil
	}
	var item models.MarketReview
	err := s.db.WithContext(ctx).Model(&models.MarketReview{}).Where("market_id = ?", marketID).First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListMarketReviews(ctx context.Context, params repository.ListMarketReviewParams) ([]models.MarketReview, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.MarketReview{})
	if params.OurAction != nil && strings.TrimSpace(*params.OurAction) != "" {
		query = query.Where("our_action = ?", strings.TrimSpace(*params.OurAction))
	}
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("settled_at >= ?", params.Since.UTC())
	}
	if params.Until != nil && !params.Until.IsZero() {
		query = query.Where("settled_at <= ?", params.Until.UTC())
	}
	if params.MinPnL != nil {
		query = query.Where("hypothetical_pnl >= ?", *params.MinPnL)
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "hypothetical_pnl")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.MarketReview
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountMarketReviews(ctx context.Context, params repository.ListMarketReviewParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.MarketReview{})
	if params.OurAction != nil && strings.TrimSpace(*params.OurAction) != "" {
		query = query.Where("our_action = ?", strings.TrimSpace(*params.OurAction))
	}
	if params.StrategyName != nil && strings.TrimSpace(*params.StrategyName) != "" {
		query = query.Where("strategy_name = ?", strings.TrimSpace(*params.StrategyName))
	}
	if params.Since != nil && !params.Since.IsZero() {
		query = query.Where("settled_at >= ?", params.Since.UTC())
	}
	if params.Until != nil && !params.Until.IsZero() {
		query = query.Where("settled_at <= ?", params.Until.UTC())
	}
	if params.MinPnL != nil {
		query = query.Where("hypothetical_pnl >= ?", *params.MinPnL)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) MissedAlphaSummary(ctx context.Context) (repository.MissedAlphaSummary, error) {
	if s == nil || s.db == nil {
		return repository.MissedAlphaSummary{}, nil
	}
	var row struct {
		TotalDismissed      int64
		ProfitableDismissed int64
		MissedAlphaUSD      float64
		AvgMissedEdge       float64
	}
	err := s.db.WithContext(ctx).Table("market_reviews").
		Select(`
			COALESCE(SUM(CASE WHEN our_action = 'dismissed' THEN 1 ELSE 0 END),0) AS total_dismissed,
			COALESCE(SUM(CASE WHEN our_action = 'dismissed' AND hypothetical_pnl > 0 THEN 1 ELSE 0 END),0) AS profitable_dismissed,
			COALESCE(SUM(CASE WHEN our_action IN ('dismissed','expired','missed') AND hypothetical_pnl > 0 THEN hypothetical_pnl ELSE 0 END),0) AS missed_alpha_usd,
			COALESCE(AVG(COALESCE(edge_at_entry,0)),0) AS avg_missed_edge
		`).Scan(&row).Error
	if err != nil {
		return repository.MissedAlphaSummary{}, err
	}
	regret := 0.0
	if row.TotalDismissed > 0 {
		regret = float64(row.ProfitableDismissed) / float64(row.TotalDismissed)
	}
	return repository.MissedAlphaSummary{
		TotalDismissed:      row.TotalDismissed,
		ProfitableDismissed: row.ProfitableDismissed,
		RegretRate:          regret,
		MissedAlphaUSD:      row.MissedAlphaUSD,
		AvgMissedEdge:       row.AvgMissedEdge,
	}, nil
}

func (s *Store) LabelPerformance(ctx context.Context) ([]repository.LabelPerformanceRow, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var rows []repository.LabelPerformanceRow
	err := s.db.WithContext(ctx).Table("market_reviews AS r").
		Select(`
			ml.label AS label,
			COALESCE(SUM(CASE WHEN r.our_action = 'traded' THEN 1 ELSE 0 END),0) AS traded_count,
			COALESCE(SUM(CASE WHEN r.our_action = 'traded' THEN r.actual_pnl ELSE 0 END),0) AS traded_pnl,
			COALESCE(SUM(CASE WHEN r.our_action IN ('dismissed','expired','missed') THEN 1 ELSE 0 END),0) AS missed_count,
			COALESCE(SUM(CASE WHEN r.our_action IN ('dismissed','expired','missed') THEN r.hypothetical_pnl ELSE 0 END),0) AS missed_alpha,
			COALESCE(AVG(CASE WHEN r.our_action = 'traded' AND r.actual_pnl > 0 THEN 1.0 ELSE 0.0 END),0) AS win_rate
		`).
		Joins("JOIN market_labels AS ml ON ml.market_id = r.market_id").
		Group("ml.label").
		Order("missed_alpha desc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) UpdateMarketReviewNotes(ctx context.Context, id uint64, notes string, lessonTags []byte) error {
	if s == nil || s.db == nil {
		return nil
	}
	if id == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&models.MarketReview{}).Where("id = ?", id).Updates(map[string]any{
		"notes":       strings.TrimSpace(notes),
		"lesson_tags": lessonTags,
		"updated_at":  time.Now().UTC(),
	}).Error
}

func (s *Store) AnalyticsOverview(ctx context.Context) (repository.AnalyticsOverview, error) {
	if s == nil || s.db == nil {
		return repository.AnalyticsOverview{}, nil
	}
	var row struct {
		TotalPlans   int64
		TotalPnLUSD  float64
		AvgROI       float64
		WinCount     int64
		LossCount    int64
		PendingCount int64
	}
	err := s.db.WithContext(ctx).
		Table("pnl_records").
		Select(`
			COUNT(*) AS total_plans,
			COALESCE(SUM(COALESCE(realized_pnl,0)),0) AS total_pnl_usd,
			COALESCE(AVG(COALESCE(realized_roi,0)),0) AS avg_roi,
			COALESCE(SUM(CASE WHEN outcome = 'win' THEN 1 ELSE 0 END),0) AS win_count,
			COALESCE(SUM(CASE WHEN outcome = 'loss' THEN 1 ELSE 0 END),0) AS loss_count,
			COALESCE(SUM(CASE WHEN outcome IS NULL OR outcome = '' OR outcome = 'pending' THEN 1 ELSE 0 END),0) AS pending_count
		`).
		Scan(&row).Error
	if err != nil {
		return repository.AnalyticsOverview{}, err
	}
	return repository.AnalyticsOverview{
		TotalPlans:   row.TotalPlans,
		TotalPnLUSD:  row.TotalPnLUSD,
		AvgROI:       row.AvgROI,
		WinCount:     row.WinCount,
		LossCount:    row.LossCount,
		PendingCount: row.PendingCount,
	}, nil
}

func (s *Store) AnalyticsByStrategy(ctx context.Context) ([]repository.StrategyAnalyticsRow, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var rows []repository.StrategyAnalyticsRow
	err := s.db.WithContext(ctx).
		Table("pnl_records").
		Select(`
			strategy_name AS strategy_name,
			COUNT(*) AS plans,
			COALESCE(SUM(COALESCE(realized_pnl,0)),0) AS total_pnl_usd,
			COALESCE(AVG(COALESCE(realized_roi,0)),0) AS avg_roi
		`).
		Group("strategy_name").
		Order("total_pnl_usd desc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) AnalyticsStrategyOutcomes(ctx context.Context) ([]repository.StrategyOutcomeRow, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var rows []repository.StrategyOutcomeRow
	err := s.db.WithContext(ctx).
		Table("pnl_records").
		Select(`
			strategy_name AS strategy_name,
			COALESCE(SUM(CASE WHEN outcome = 'win' THEN 1 ELSE 0 END),0) AS win_count,
			COALESCE(SUM(CASE WHEN outcome = 'loss' THEN 1 ELSE 0 END),0) AS loss_count,
			COALESCE(SUM(CASE WHEN outcome = 'partial' THEN 1 ELSE 0 END),0) AS partial_count,
			COALESCE(SUM(CASE WHEN outcome IS NULL OR outcome = '' OR outcome = 'pending' THEN 1 ELSE 0 END),0) AS pending_count
		`).
		Group("strategy_name").
		Order("strategy_name asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) AnalyticsFailures(ctx context.Context) ([]repository.FailureAnalyticsRow, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var rows []repository.FailureAnalyticsRow
	err := s.db.WithContext(ctx).
		Table("pnl_records").
		Select("COALESCE(failure_reason,'') AS failure_reason, COUNT(*) AS count").
		Where("failure_reason IS NOT NULL AND failure_reason <> ''").
		Group("failure_reason").
		Order("count desc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) ListEvents(ctx context.Context, params repository.ListEventsParams) ([]models.Event, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Event{})
	if params.Active != nil {
		query = query.Where("active = ?", *params.Active)
	}
	if params.Closed != nil {
		query = query.Where("closed = ?", *params.Closed)
	}
	if params.Slug != nil && *params.Slug != "" {
		query = query.Where("slug = ?", *params.Slug)
	}
	if params.Title != nil && *params.Title != "" {
		query = query.Where("title ILIKE ?", "%"+*params.Title+"%")
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "external_updated_at")
	limit := normalizeLimit(params.Limit, 100)
	offset := normalizeOffset(params.Offset)
	var items []models.Event
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountEvents(ctx context.Context, params repository.ListEventsParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Event{})
	if params.Active != nil {
		query = query.Where("active = ?", *params.Active)
	}
	if params.Closed != nil {
		query = query.Where("closed = ?", *params.Closed)
	}
	if params.Slug != nil && *params.Slug != "" {
		query = query.Where("slug = ?", *params.Slug)
	}
	if params.Title != nil && *params.Title != "" {
		query = query.Where("title ILIKE ?", "%"+*params.Title+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListMarkets(ctx context.Context, params repository.ListMarketsParams) ([]models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Market{})
	if params.Active != nil {
		query = query.Where("active = ?", *params.Active)
	}
	if params.Closed != nil {
		query = query.Where("closed = ?", *params.Closed)
	}
	if params.EventID != nil && *params.EventID != "" {
		query = query.Where("event_id = ?", *params.EventID)
	}
	if params.Slug != nil && *params.Slug != "" {
		query = query.Where("slug = ?", *params.Slug)
	}
	if params.Question != nil && *params.Question != "" {
		query = query.Where("question ILIKE ?", "%"+*params.Question+"%")
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "external_updated_at")
	limit := normalizeLimit(params.Limit, 100)
	offset := normalizeOffset(params.Offset)
	var items []models.Market
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountMarkets(ctx context.Context, params repository.ListMarketsParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Market{})
	if params.Active != nil {
		query = query.Where("active = ?", *params.Active)
	}
	if params.Closed != nil {
		query = query.Where("closed = ?", *params.Closed)
	}
	if params.EventID != nil && *params.EventID != "" {
		query = query.Where("event_id = ?", *params.EventID)
	}
	if params.Slug != nil && *params.Slug != "" {
		query = query.Where("slug = ?", *params.Slug)
	}
	if params.Question != nil && *params.Question != "" {
		query = query.Where("question ILIKE ?", "%"+*params.Question+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) ListMarketIDsForStream(ctx context.Context, limit int) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	limit = normalizeLimit(limit, 200)
	var ids []string
	err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("active = ?", true).
		Where("closed = ?", false).
		Order("external_updated_at desc").
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *Store) ListTokensByMarketIDs(ctx context.Context, marketIDs []string) ([]models.Token, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(marketIDs) == 0 {
		return nil, nil
	}
	var items []models.Token
	if err := s.db.WithContext(ctx).
		Model(&models.Token{}).
		Where("market_id IN ?", marketIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListTokensByIDs(ctx context.Context, tokenIDs []string) ([]models.Token, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	tokenIDs = cleanStrings(tokenIDs)
	if len(tokenIDs) == 0 {
		return nil, nil
	}
	var items []models.Token
	if err := s.db.WithContext(ctx).
		Model(&models.Token{}).
		Where("id IN ?", tokenIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListMarketDataHealthByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.MarketDataHealth, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(tokenIDs) == 0 {
		return nil, nil
	}
	var items []models.MarketDataHealth
	if err := s.db.WithContext(ctx).
		Model(&models.MarketDataHealth{}).
		Where("token_id IN ?", tokenIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListOrderbookLatestByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.OrderbookLatest, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(tokenIDs) == 0 {
		return nil, nil
	}
	var items []models.OrderbookLatest
	if err := s.db.WithContext(ctx).
		Model(&models.OrderbookLatest{}).
		Where("token_id IN ?", tokenIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListLastTradePricesByTokenIDs(ctx context.Context, tokenIDs []string) ([]models.LastTradePrice, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(tokenIDs) == 0 {
		return nil, nil
	}
	var items []models.LastTradePrice
	if err := s.db.WithContext(ctx).
		Model(&models.LastTradePrice{}).
		Where("token_id IN ?", tokenIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListMarketAggregates(ctx context.Context, limit int) ([]repository.EventAggregate, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	limit = normalizeLimit(limit, 2000)
	var rows []struct {
		EventID       string
		MarketCount   int
		SumLiquidity  decimal.Decimal
		SumVolume     decimal.Decimal
		LatestUpdated *time.Time
	}
	if err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Select("event_id as event_id, COUNT(*) as market_count, COALESCE(SUM(liquidity),0) as sum_liquidity, COALESCE(SUM(volume),0) as sum_volume, MAX(external_updated_at) as latest_updated").
		Where("active = ?", true).
		Where("closed = ?", false).
		Group("event_id").
		Order("sum_liquidity desc").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]repository.EventAggregate, 0, len(rows))
	for _, row := range rows {
		out = append(out, repository.EventAggregate{
			EventID:       row.EventID,
			MarketCount:   row.MarketCount,
			SumLiquidity:  row.SumLiquidity,
			SumVolume:     row.SumVolume,
			LatestUpdated: row.LatestUpdated,
		})
	}
	return out, nil
}

func (s *Store) ListEventsByIDs(ctx context.Context, ids []string) ([]models.Event, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var items []models.Event
	if err := s.db.WithContext(ctx).
		Model(&models.Event{}).
		Where("id IN ?", ids).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) FindMarketsByConditionIDs(ctx context.Context, conditionIDs []string) ([]models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(conditionIDs) == 0 {
		return nil, nil
	}
	var items []models.Market
	if err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("condition_id IN ?", conditionIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) FindMarketsBySlugs(ctx context.Context, slugs []string) ([]models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(slugs) == 0 {
		return nil, nil
	}
	var items []models.Market
	if err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("slug IN ?", slugs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetMarketBySlug(ctx context.Context, slug string) (*models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if strings.TrimSpace(slug) == "" {
		return nil, nil
	}
	var item models.Market
	err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("slug = ?", slug).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) GetEventBySlug(ctx context.Context, slug string) (*models.Event, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if strings.TrimSpace(slug) == "" {
		return nil, nil
	}
	var item models.Event
	err := s.db.WithContext(ctx).
		Model(&models.Event{}).
		Where("slug = ?", slug).
		First(&item).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) ListMarketsByEventID(ctx context.Context, eventID string) ([]models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if strings.TrimSpace(eventID) == "" {
		return nil, nil
	}
	var items []models.Market
	if err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("event_id = ?", eventID).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListMarketsByEventIDs(ctx context.Context, eventIDs []string) ([]models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(eventIDs) == 0 {
		return nil, nil
	}
	var items []models.Market
	if err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("event_id IN ?", eventIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListMarketsByIDs(ctx context.Context, marketIDs []string) ([]models.Market, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if len(marketIDs) == 0 {
		return nil, nil
	}
	var items []models.Market
	if err := s.db.WithContext(ctx).
		Model(&models.Market{}).
		Where("id IN ?", marketIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) ListTokens(ctx context.Context, params repository.ListTokensParams) ([]models.Token, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Token{})
	if params.MarketID != nil && *params.MarketID != "" {
		query = query.Where("market_id = ?", *params.MarketID)
	}
	if params.Outcome != nil && *params.Outcome != "" {
		query = query.Where("outcome = ?", *params.Outcome)
	}
	if params.Side != nil && *params.Side != "" {
		query = query.Where("side = ?", *params.Side)
	}
	query = applyOrder(query, params.OrderBy, params.Asc, "external_updated_at")
	limit := normalizeLimit(params.Limit, 200)
	offset := normalizeOffset(params.Offset)
	var items []models.Token
	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) CountTokens(ctx context.Context, params repository.ListTokensParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	query := s.db.WithContext(ctx).Model(&models.Token{})
	if params.MarketID != nil && *params.MarketID != "" {
		query = query.Where("market_id = ?", *params.MarketID)
	}
	if params.Outcome != nil && *params.Outcome != "" {
		query = query.Where("outcome = ?", *params.Outcome)
	}
	if params.Side != nil && *params.Side != "" {
		query = query.Where("side = ?", *params.Side)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Store) UpsertEventsTx(ctx context.Context, tx *gorm.DB, items []models.Event) error {
	if len(items) == 0 {
		return nil
	}
	return createInBatches(tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"slug",
			"title",
			"description",
			"active",
			"closed",
			"neg_risk",
			"start_time",
			"end_time",
			"series_id",
			"external_created_at",
			"external_updated_at",
			"last_seen_at",
			"raw_json",
		}),
	}), items, 200)
}

func (s *Store) UpsertMarketsTx(ctx context.Context, tx *gorm.DB, items []models.Market) error {
	if len(items) == 0 {
		return nil
	}
	return createInBatches(tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"event_id",
			"slug",
			"question",
			"condition_id",
			"market_address",
			"tick_size",
			"volume",
			"liquidity",
			"active",
			"closed",
			"neg_risk",
			"status",
			"external_created_at",
			"external_updated_at",
			"last_seen_at",
			"raw_json",
		}),
	}), items, 200)
}

func (s *Store) UpsertTokensTx(ctx context.Context, tx *gorm.DB, items []models.Token) error {
	if len(items) == 0 {
		return nil
	}
	return createInBatches(tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"market_id",
			"outcome",
			"side",
			"external_created_at",
			"external_updated_at",
			"last_seen_at",
			"raw_json",
		}),
	}), items, 300)
}

func (s *Store) UpsertSeriesTx(ctx context.Context, tx *gorm.DB, items []models.Series) error {
	if len(items) == 0 {
		return nil
	}
	return createInBatches(tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"title",
			"slug",
			"image",
			"external_updated_at",
			"last_seen_at",
			"raw_json",
		}),
	}), items, 200)
}

func (s *Store) UpsertTagsTx(ctx context.Context, tx *gorm.DB, items []models.Tag) error {
	if len(items) == 0 {
		return nil
	}
	return createInBatches(tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"label",
			"slug",
			"external_updated_at",
			"last_seen_at",
			"raw_json",
		}),
	}), items, 200)
}

func (s *Store) UpsertEventTagsTx(ctx context.Context, tx *gorm.DB, items []models.EventTag) error {
	if len(items) == 0 {
		return nil
	}
	return createInBatches(tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}, {Name: "tag_id"}},
		DoNothing: true,
	}), items, 500)
}

func (s *Store) UpsertOrderbookLatest(ctx context.Context, item *models.OrderbookLatest) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"snapshot_ts",
			"bids_json",
			"asks_json",
			"best_bid",
			"best_ask",
			"mid",
			"source",
			"data_age_seconds",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) UpsertMarketDataHealth(ctx context.Context, item *models.MarketDataHealth) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"ws_connected",
			"last_ws_ts",
			"last_rest_ts",
			"data_age_seconds",
			"stale",
			"needs_resync",
			"last_resync_ts",
			"spread",
			"spread_bps",
			"price_jump_bps",
			"last_book_change_ts",
			"reason",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) UpsertLastTradePrice(ctx context.Context, item *models.LastTradePrice) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "token_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"price",
			"trade_ts",
			"source",
			"updated_at",
		}),
	}).Create(item).Error
}

func (s *Store) InsertRawWSEvent(ctx context.Context, item *models.RawWSEvent) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) InsertRawRESTSnapshot(ctx context.Context, item *models.RawRESTSnapshot) error {
	if s == nil || s.db == nil || item == nil {
		return nil
	}
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *Store) GetSyncState(ctx context.Context, scope string) (*models.SyncState, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var state models.SyncState
	err := s.db.WithContext(ctx).First(&state, "scope = ?", scope).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) SaveSyncStateTx(ctx context.Context, tx *gorm.DB, state *models.SyncState) error {
	if state == nil {
		return nil
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "scope"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"cursor",
			"watermark_ts",
			"last_success_at",
			"last_attempt_at",
			"last_error",
			"stats_json",
		}),
	}).Create(state).Error
}

func (s *Store) ListSyncStates(ctx context.Context) ([]models.SyncState, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var states []models.SyncState
	if err := s.db.WithContext(ctx).Order("scope asc").Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}

func applyOrder(query *gorm.DB, orderBy string, asc *bool, fallback string) *gorm.DB {
	column := strings.TrimSpace(orderBy)
	if column == "" {
		column = fallback
	}
	direction := "desc"
	if asc != nil && *asc {
		direction = "asc"
	}
	return query.Order(column + " " + direction)
}

func createInBatches[T any](db *gorm.DB, items []T, batchSize int) error {
	if len(items) == 0 {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 200
	}
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		if err := db.CreateInBatches(items[i:end], batchSize).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func cleanStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, raw := range items {
		val := strings.TrimSpace(raw)
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out
}

func (s *Store) ListActiveEventsEndingSoon(ctx context.Context, hoursToExpiry int, limit int) ([]models.Event, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	limit = normalizeLimit(limit, 200)
	deadline := time.Now().Add(time.Duration(hoursToExpiry) * time.Hour)
	var items []models.Event
	if err := s.db.WithContext(ctx).
		Model(&models.Event{}).
		Where("active = ?", true).
		Where("closed = ?", false).
		Where("end_time IS NOT NULL").
		Where("end_time <= ?", deadline).
		Where("end_time > ?", time.Now()).
		Order("end_time asc").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

var _ repository.CatalogRepository = (*Store)(nil)
