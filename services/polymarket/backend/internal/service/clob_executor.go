package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	polymarketclob "polymarket/internal/client/polymarket/clob"

	"polymarket/internal/models"
	"polymarket/internal/repository"
	"polymarket/internal/risk"
)

type ExecutorConfig struct {
	Mode                 string
	MaxOrderSizeUSD      decimal.Decimal
	SlippageToleranceBps int
}

type SubmitResult struct {
	PlanID     uint64   `json:"plan_id"`
	OrderIDs   []uint64 `json:"order_ids"`
	Mode       string   `json:"mode"`
	PlanStatus string   `json:"plan_status"`
}

type CLOBExecutor struct {
	Repo         repository.Repository
	Risk         *risk.Manager
	Logger       *zap.Logger
	Config       ExecutorConfig
	PositionSync *PositionSyncService
	Client       *polymarketclob.Client
}

type orderLeg struct {
	TokenID        string   `json:"token_id"`
	Direction      string   `json:"direction"`
	TargetPrice    *float64 `json:"target_price"`
	CurrentBestAsk *float64 `json:"current_best_ask"`
	SizeUSD        *float64 `json:"size_usd"`
	SignedOrder    any      `json:"signed_order"`
	UnsignedOrder  any      `json:"unsigned_order"`
	SigningHash    string   `json:"signing_hash"`
	SignatureField string   `json:"signature_field"`
	OwnerField     string   `json:"owner_field"`
	OrderType      string   `json:"order_type"`
	Owner          string   `json:"owner"`
	PostOnly       *bool    `json:"post_only"`
}

func (e *CLOBExecutor) SubmitPlan(ctx context.Context, planID uint64) (*SubmitResult, error) {
	if e == nil || e.Repo == nil || planID == 0 {
		return nil, nil
	}
	plan, err := e.Repo.GetExecutionPlanByID(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	if plan.Status != "preflight_pass" && plan.Status != "executing" {
		return nil, fmt.Errorf("plan status %s is not submittable", plan.Status)
	}
	if e.Risk != nil {
		res, err := e.Risk.PreflightPlan(ctx, planID)
		if err != nil {
			return nil, err
		}
		if res != nil && !res.Passed {
			return nil, fmt.Errorf("preflight failed")
		}
	}
	mode := e.resolveMode(ctx)
	legs, err := parseOrderLegs(plan.Legs)
	if err != nil {
		return nil, err
	}
	if len(legs) == 0 {
		return nil, fmt.Errorf("plan has no legs")
	}

	orderIDs := make([]uint64, 0, len(legs))
	perLeg := plan.PlannedSizeUSD.Div(decimal.NewFromInt(int64(len(legs))))
	for _, leg := range legs {
		tokenID := strings.TrimSpace(leg.TokenID)
		if tokenID == "" {
			continue
		}
		price := decimal.NewFromFloat(0.5)
		if leg.TargetPrice != nil && *leg.TargetPrice > 0 {
			price = decimal.NewFromFloat(*leg.TargetPrice)
		} else if leg.CurrentBestAsk != nil && *leg.CurrentBestAsk > 0 {
			price = decimal.NewFromFloat(*leg.CurrentBestAsk)
		}
		sizeUSD := perLeg
		if leg.SizeUSD != nil && *leg.SizeUSD > 0 {
			sizeUSD = decimal.NewFromFloat(*leg.SizeUSD)
		}
		if e.Config.MaxOrderSizeUSD.GreaterThan(decimal.Zero) && sizeUSD.GreaterThan(e.Config.MaxOrderSizeUSD) {
			sizeUSD = e.Config.MaxOrderSizeUSD
		}
		order := &models.Order{
			PlanID:    plan.ID,
			TokenID:   tokenID,
			Side:      strings.ToUpper(strings.TrimSpace(leg.Direction)),
			OrderType: "limit",
			Price:     price,
			SizeUSD:   sizeUSD,
			FilledUSD: decimal.Zero,
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if order.Side == "" {
			order.Side = "BUY_YES"
		}
		if err := e.Repo.InsertOrder(ctx, order); err != nil {
			return nil, err
		}
		orderIDs = append(orderIDs, order.ID)

		if mode == "dry-run" {
			now := time.Now().UTC()
			_ = e.Repo.UpdateOrderStatus(ctx, order.ID, "filled", map[string]any{
				"filled_usd": sizeUSD,
				"filled_at":  &now,
			})
			fillSize := decimal.Zero
			if price.GreaterThan(decimal.Zero) {
				fillSize = sizeUSD.Div(price)
			}
			fill := &models.Fill{
				PlanID:     plan.ID,
				TokenID:    tokenID,
				Direction:  order.Side,
				FilledSize: fillSize,
				AvgPrice:   price,
				Fee:        decimal.Zero,
				FilledAt:   now,
				CreatedAt:  now,
			}
			_ = e.Repo.InsertFill(ctx, fill)
			if e.PositionSync != nil {
				_ = e.PositionSync.SyncFromFill(ctx, *fill)
			}
		} else {
			status, updates, err := e.submitLiveOrder(ctx, *plan, *order, leg)
			if err != nil {
				_ = e.Repo.UpdateOrderStatus(ctx, order.ID, "failed", map[string]any{
					"failure_reason": err.Error(),
				})
				if e.Logger != nil {
					e.Logger.Warn("live order submit failed", zap.Uint64("order_id", order.ID), zap.Error(err))
				}
			} else {
				_ = e.Repo.UpdateOrderStatus(ctx, order.ID, status, updates)
				if status == "filled" || status == "partial" {
					_ = e.applyOrderFillDelta(ctx, *order, updates)
				}
			}
		}
	}
	_ = e.reconcilePlanStatus(ctx, plan.ID)

	if mode == "dry-run" {
		now := time.Now().UTC()
		_ = e.Repo.UpdateExecutionPlanExecutedAt(ctx, plan.ID, "executed", &now)
		_ = e.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executed")
	} else {
		_ = e.Repo.UpdateExecutionPlanStatus(ctx, plan.ID, "executing")
		_ = e.Repo.UpdateOpportunityStatus(ctx, plan.OpportunityID, "executing")
	}

	return &SubmitResult{
		PlanID:     plan.ID,
		OrderIDs:   orderIDs,
		Mode:       mode,
		PlanStatus: map[bool]string{true: "executed", false: "executing"}[mode == "dry-run"],
	}, nil
}

func (e *CLOBExecutor) PollOrders(ctx context.Context) error {
	if e == nil || e.Repo == nil {
		return nil
	}
	mode := e.resolveMode(ctx)
	if mode == "live" {
		orders, err := e.listLiveSyncCandidates(ctx)
		if err != nil {
			return err
		}
		for _, order := range orders {
			if strings.TrimSpace(order.ClobOrderID) == "" {
				continue
			}
			status, updates, err := e.fetchLiveOrder(ctx, order.ClobOrderID)
			if err != nil {
				if e.Logger != nil {
					e.Logger.Warn("live order poll failed", zap.Uint64("order_id", order.ID), zap.Error(err))
				}
				continue
			}
			if err := e.Repo.UpdateOrderStatus(ctx, order.ID, status, updates); err != nil {
				continue
			}
			if status == "filled" || status == "partial" {
				_ = e.applyOrderFillDelta(ctx, order, updates)
			}
			_ = e.reconcilePlanStatus(ctx, order.PlanID)
		}
	}
	return nil
}

func (e *CLOBExecutor) CancelOrder(ctx context.Context, orderID uint64) error {
	if e == nil || e.Repo == nil || orderID == 0 {
		return nil
	}
	order, err := e.Repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return nil
	}
	switch order.Status {
	case "submitted", "partial", "pending":
		if e.resolveMode(ctx) == "live" && strings.TrimSpace(order.ClobOrderID) != "" {
			status, updates, err := e.cancelLiveOrder(ctx, order.ClobOrderID)
			if err == nil {
				return e.Repo.UpdateOrderStatus(ctx, orderID, status, updates)
			}
			if e.Logger != nil {
				e.Logger.Warn("cancel live order failed, fallback local cancel", zap.Uint64("order_id", orderID), zap.Error(err))
			}
		}
		now := time.Now().UTC()
		return e.Repo.UpdateOrderStatus(ctx, orderID, "cancelled", map[string]any{"cancelled_at": &now})
	default:
		return nil
	}
}

func parseOrderLegs(raw []byte) ([]orderLeg, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []orderLeg
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (e *CLOBExecutor) resolveMode(ctx context.Context) string {
	mode := strings.ToLower(strings.TrimSpace(e.Config.Mode))
	if e != nil && e.Repo != nil {
		if row, err := e.Repo.GetSystemSettingByKey(ctx, "trading.executor_mode"); err == nil && row != nil && len(row.Value) > 0 {
			var v string
			if err := json.Unmarshal(row.Value, &v); err == nil {
				v = strings.ToLower(strings.TrimSpace(v))
				if v == "dry-run" || v == "live" {
					mode = v
				}
			}
		}
	}
	if mode == "" {
		return "dry-run"
	}
	return mode
}

type liveBrokerConfig struct {
	BaseURL          string
	SubmitPath       string
	StatusPath       string
	CancelPath       string
	AuthMode         string
	APIKey           string
	APIKeyHeader     string
	BearerToken      string
	APISecret        string
	TimestampHeader  string
	SignatureHeader  string
	Passphrase       string
	PassphraseHeader string
	Address          string
	AddressHeader    string
	SignerURL        string
	PrivateKey       string
}

func (e *CLOBExecutor) loadLiveBrokerConfig(ctx context.Context) liveBrokerConfig {
	cfg := liveBrokerConfig{
		SubmitPath:       "/orders",
		StatusPath:       "/orders/{order_id}",
		CancelPath:       "/orders/{order_id}/cancel",
		AuthMode:         "api_key",
		APIKeyHeader:     "X-API-Key",
		TimestampHeader:  "X-Timestamp",
		SignatureHeader:  "X-Signature",
		PassphraseHeader: "X-Passphrase",
		AddressHeader:    "X-Address",
	}
	if e == nil || e.Repo == nil {
		return cfg
	}
	read := func(key string) string {
		row, err := e.Repo.GetSystemSettingByKey(ctx, key)
		if err != nil || row == nil || len(row.Value) == 0 {
			return ""
		}
		raw := RevealSettingValue(key, row.Value)
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s)
		}
		return ""
	}
	if v := read("trading.live.base_url"); v != "" {
		cfg.BaseURL = v
	}
	if v := read("trading.live.submit_path"); v != "" {
		cfg.SubmitPath = v
	}
	if v := read("trading.live.status_path"); v != "" {
		cfg.StatusPath = v
	}
	if v := read("trading.live.cancel_path"); v != "" {
		cfg.CancelPath = v
	}
	if v := strings.ToLower(read("trading.live.auth_mode")); v != "" {
		cfg.AuthMode = v
	}
	if v := read("trading.live.api_key"); v != "" {
		cfg.APIKey = v
	}
	if v := read("trading.live.api_key_header"); v != "" {
		cfg.APIKeyHeader = v
	}
	if v := read("trading.live.bearer_token"); v != "" {
		cfg.BearerToken = v
	}
	if v := read("trading.live.api_secret"); v != "" {
		cfg.APISecret = v
	}
	if v := read("trading.live.timestamp_header"); v != "" {
		cfg.TimestampHeader = v
	}
	if v := read("trading.live.signature_header"); v != "" {
		cfg.SignatureHeader = v
	}
	if v := read("trading.live.passphrase"); v != "" {
		cfg.Passphrase = v
	}
	if v := read("trading.live.passphrase_header"); v != "" {
		cfg.PassphraseHeader = v
	}
	if v := read("trading.live.address"); v != "" {
		cfg.Address = v
	}
	if v := read("trading.live.address_header"); v != "" {
		cfg.AddressHeader = v
	}
	if v := read("trading.live.signer_url"); v != "" {
		cfg.SignerURL = v
	}
	if v := read("trading.live.private_key"); v != "" {
		cfg.PrivateKey = v
	}
	if cfg.AuthMode == "polymarket_l2" || cfg.AuthMode == "polymarket_l2_signer" || cfg.AuthMode == "polymarket_l2_local" {
		if strings.TrimSpace(cfg.APIKeyHeader) == "" || strings.EqualFold(cfg.APIKeyHeader, "X-API-Key") {
			cfg.APIKeyHeader = "POLY_API_KEY"
		}
		if strings.TrimSpace(cfg.TimestampHeader) == "" || strings.EqualFold(cfg.TimestampHeader, "X-Timestamp") {
			cfg.TimestampHeader = "POLY_TIMESTAMP"
		}
		if strings.TrimSpace(cfg.SignatureHeader) == "" || strings.EqualFold(cfg.SignatureHeader, "X-Signature") {
			cfg.SignatureHeader = "POLY_SIGNATURE"
		}
		if strings.TrimSpace(cfg.PassphraseHeader) == "" || strings.EqualFold(cfg.PassphraseHeader, "X-Passphrase") {
			cfg.PassphraseHeader = "POLY_PASSPHRASE"
		}
		if strings.TrimSpace(cfg.AddressHeader) == "" || strings.EqualFold(cfg.AddressHeader, "X-Address") {
			cfg.AddressHeader = "POLY_ADDRESS"
		}
	}
	return cfg
}

func (e *CLOBExecutor) buildLiveClient(ctx context.Context) (*polymarketclob.Client, liveBrokerConfig, error) {
	cfg := e.loadLiveBrokerConfig(ctx)
	client := e.Client
	if strings.TrimSpace(cfg.BaseURL) != "" {
		client = polymarketclob.NewClient(&http.Client{Timeout: 15 * time.Second}, cfg.BaseURL)
	}
	if client == nil {
		return nil, cfg, fmt.Errorf("live client unavailable: configure trading.live.base_url")
	}
	return client, cfg, nil
}

func (e *CLOBExecutor) submitLiveOrder(ctx context.Context, plan models.ExecutionPlan, order models.Order, leg orderLeg) (string, map[string]any, error) {
	client, cfg, err := e.buildLiveClient(ctx)
	if err != nil {
		return "", nil, err
	}
	auth := polymarketclob.TradingAuth{
		APIKeyHeader:     cfg.APIKeyHeader,
		APIKey:           cfg.APIKey,
		BearerToken:      cfg.BearerToken,
		APISecret:        cfg.APISecret,
		SignRequests:     cfg.AuthMode == "hmac" || cfg.AuthMode == "polymarket_l2" || cfg.AuthMode == "polymarket_l2_signer" || cfg.AuthMode == "polymarket_l2_local",
		TimestampHeader:  cfg.TimestampHeader,
		SignatureHeader:  cfg.SignatureHeader,
		Passphrase:       cfg.Passphrase,
		PassphraseHeader: cfg.PassphraseHeader,
		Address:          cfg.Address,
		AddressHeader:    cfg.AddressHeader,
	}
	if leg.SignedOrder == nil && cfg.AuthMode == "polymarket_l2_signer" {
		signedOrder, owner, orderType, postOnly, err := e.requestSignedOrder(ctx, cfg, plan, order, leg)
		if err != nil {
			return "", nil, err
		}
		leg.SignedOrder = signedOrder
		leg.Owner = owner
		if strings.TrimSpace(orderType) != "" {
			leg.OrderType = orderType
		}
		if postOnly != nil {
			leg.PostOnly = postOnly
		}
	}
	if leg.SignedOrder == nil && cfg.AuthMode == "polymarket_l2_local" {
		signedOrder, owner, orderType, postOnly, err := e.signOrderLocally(cfg, order, leg)
		if err != nil {
			return "", nil, err
		}
		leg.SignedOrder = signedOrder
		leg.Owner = owner
		if strings.TrimSpace(orderType) != "" {
			leg.OrderType = orderType
		}
		if postOnly != nil {
			leg.PostOnly = postOnly
		}
	}
	var resp *polymarketclob.TradingOrder
	if leg.SignedOrder != nil {
		submitPath := strings.TrimSpace(cfg.SubmitPath)
		if submitPath == "" || strings.EqualFold(submitPath, "/orders") {
			submitPath = "/order"
		}
		postOnly := leg.PostOnly
		orderType := strings.TrimSpace(leg.OrderType)
		if orderType == "" {
			orderType = "GTC"
		}
		owner := strings.TrimSpace(leg.Owner)
		if owner == "" {
			owner = strings.TrimSpace(cfg.Address)
		}
		resp, err = client.PlaceSignedOrder(ctx, submitPath, polymarketclob.PlaceSignedOrderRequest{
			Order:     leg.SignedOrder,
			Owner:     owner,
			OrderType: orderType,
			PostOnly:  postOnly,
		}, auth)
	} else {
		req := polymarketclob.PlaceOrderRequest{
			TokenID:       strings.TrimSpace(order.TokenID),
			Side:          strings.TrimSpace(order.Side),
			OrderType:     strings.TrimSpace(order.OrderType),
			Price:         order.Price.InexactFloat64(),
			SizeUSD:       order.SizeUSD.InexactFloat64(),
			ClientOrderID: strconv.FormatUint(order.ID, 10),
			PlanID:        plan.ID,
		}
		resp, err = client.PlaceOrder(ctx, cfg.SubmitPath, req, auth)
	}
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	status := normalizeLiveStatus(resp.Status)
	if status == "" {
		status = "submitted"
	}
	updates := map[string]any{
		"clob_order_id": resp.OrderID,
		"submitted_at":  timeOrPtr(resp.SubmittedAt, &now),
	}
	if resp.FilledUSD > 0 {
		updates["filled_usd"] = decimal.NewFromFloat(resp.FilledUSD)
	}
	if resp.FilledAt != nil {
		updates["filled_at"] = resp.FilledAt
	}
	if resp.CancelledAt != nil {
		updates["cancelled_at"] = resp.CancelledAt
	}
	if strings.TrimSpace(resp.Failure) != "" {
		updates["failure_reason"] = strings.TrimSpace(resp.Failure)
	}
	return status, updates, nil
}

func (e *CLOBExecutor) fetchLiveOrder(ctx context.Context, clobOrderID string) (string, map[string]any, error) {
	client, cfg, err := e.buildLiveClient(ctx)
	if err != nil {
		return "", nil, err
	}
	resp, err := client.GetOrder(ctx, cfg.StatusPath, clobOrderID, polymarketclob.TradingAuth{
		APIKeyHeader:     cfg.APIKeyHeader,
		APIKey:           cfg.APIKey,
		BearerToken:      cfg.BearerToken,
		APISecret:        cfg.APISecret,
		SignRequests:     cfg.AuthMode == "hmac" || cfg.AuthMode == "polymarket_l2" || cfg.AuthMode == "polymarket_l2_signer" || cfg.AuthMode == "polymarket_l2_local",
		TimestampHeader:  cfg.TimestampHeader,
		SignatureHeader:  cfg.SignatureHeader,
		Passphrase:       cfg.Passphrase,
		PassphraseHeader: cfg.PassphraseHeader,
		Address:          cfg.Address,
		AddressHeader:    cfg.AddressHeader,
	})
	if err != nil {
		return "", nil, err
	}
	status := normalizeLiveStatus(resp.Status)
	updates := map[string]any{}
	if resp.FilledUSD > 0 {
		updates["filled_usd"] = decimal.NewFromFloat(resp.FilledUSD)
	}
	if resp.FilledAt != nil {
		updates["filled_at"] = resp.FilledAt
	}
	if resp.CancelledAt != nil {
		updates["cancelled_at"] = resp.CancelledAt
	}
	if strings.TrimSpace(resp.Failure) != "" {
		updates["failure_reason"] = strings.TrimSpace(resp.Failure)
	}
	return status, updates, nil
}

func (e *CLOBExecutor) cancelLiveOrder(ctx context.Context, clobOrderID string) (string, map[string]any, error) {
	client, cfg, err := e.buildLiveClient(ctx)
	if err != nil {
		return "", nil, err
	}
	resp, err := client.CancelOrder(ctx, cfg.CancelPath, clobOrderID, polymarketclob.TradingAuth{
		APIKeyHeader:     cfg.APIKeyHeader,
		APIKey:           cfg.APIKey,
		BearerToken:      cfg.BearerToken,
		APISecret:        cfg.APISecret,
		SignRequests:     cfg.AuthMode == "hmac" || cfg.AuthMode == "polymarket_l2" || cfg.AuthMode == "polymarket_l2_signer" || cfg.AuthMode == "polymarket_l2_local",
		TimestampHeader:  cfg.TimestampHeader,
		SignatureHeader:  cfg.SignatureHeader,
		Passphrase:       cfg.Passphrase,
		PassphraseHeader: cfg.PassphraseHeader,
		Address:          cfg.Address,
		AddressHeader:    cfg.AddressHeader,
	})
	if err != nil {
		return "", nil, err
	}
	status := normalizeLiveStatus(resp.Status)
	if status == "" {
		status = "cancelled"
	}
	now := time.Now().UTC()
	updates := map[string]any{"cancelled_at": timeOrPtr(resp.CancelledAt, &now)}
	return status, updates, nil
}

func (e *CLOBExecutor) listLiveSyncCandidates(ctx context.Context) ([]models.Order, error) {
	submitted := "submitted"
	partial := "partial"
	a, err := e.Repo.ListOrders(ctx, repository.ListOrdersParams{Limit: 500, Offset: 0, Status: &submitted, OrderBy: "updated_at", Asc: boolPtrExecutor(false)})
	if err != nil {
		return nil, err
	}
	b, err := e.Repo.ListOrders(ctx, repository.ListOrdersParams{Limit: 500, Offset: 0, Status: &partial, OrderBy: "updated_at", Asc: boolPtrExecutor(false)})
	if err != nil {
		return nil, err
	}
	out := make([]models.Order, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out, nil
}

func (e *CLOBExecutor) applyOrderFillDelta(ctx context.Context, order models.Order, updates map[string]any) error {
	remoteFilledUSD, ok := updates["filled_usd"]
	if !ok {
		return nil
	}
	next, ok := remoteFilledUSD.(decimal.Decimal)
	if !ok {
		return nil
	}
	prev := order.FilledUSD
	deltaUSD := next.Sub(prev)
	if !deltaUSD.GreaterThan(decimal.Zero) {
		return nil
	}
	price := order.Price
	if price.LessThanOrEqual(decimal.Zero) {
		price = decimal.NewFromFloat(0.5)
	}
	deltaSize := deltaUSD.Div(price)
	fill := &models.Fill{
		PlanID:     order.PlanID,
		TokenID:    order.TokenID,
		Direction:  order.Side,
		FilledSize: deltaSize,
		AvgPrice:   price,
		Fee:        decimal.Zero,
		FilledAt:   time.Now().UTC(),
		CreatedAt:  time.Now().UTC(),
	}
	if err := e.Repo.InsertFill(ctx, fill); err != nil {
		return err
	}
	if e.PositionSync != nil {
		_ = e.PositionSync.SyncFromFill(ctx, *fill)
	}
	return nil
}

func normalizeLiveStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "submitted", "open", "accepted", "placed":
		return "submitted"
	case "partial", "partially_filled", "partial_fill":
		return "partial"
	case "filled", "done", "executed":
		return "filled"
	case "cancelled", "canceled":
		return "cancelled"
	case "failed", "rejected", "error":
		return "failed"
	default:
		return s
	}
}

func timeOrPtr(src *time.Time, fallback *time.Time) *time.Time {
	if src != nil {
		return src
	}
	return fallback
}

func boolPtrExecutor(v bool) *bool { return &v }

func (e *CLOBExecutor) requestSignedOrder(ctx context.Context, cfg liveBrokerConfig, plan models.ExecutionPlan, order models.Order, leg orderLeg) (any, string, string, *bool, error) {
	if strings.TrimSpace(cfg.SignerURL) == "" {
		return nil, "", "", nil, fmt.Errorf("trading.live.signer_url is required for auth_mode=polymarket_l2_signer")
	}
	payload := map[string]any{
		"plan_id":      plan.ID,
		"order_id":     order.ID,
		"token_id":     order.TokenID,
		"side":         order.Side,
		"order_type":   order.OrderType,
		"price":        order.Price.String(),
		"size_usd":     order.SizeUSD.String(),
		"strategy":     plan.StrategyName,
		"opportunity":  plan.OpportunityID,
		"client_order": strconv.FormatUint(order.ID, 10),
		"leg":          leg,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.SignerURL, strings.NewReader(string(raw)))
	if err != nil {
		return nil, "", "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", nil, fmt.Errorf("signer error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		SignedOrder any    `json:"signed_order"`
		Owner       string `json:"owner"`
		OrderType   string `json:"order_type"`
		PostOnly    *bool  `json:"post_only"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", "", nil, err
	}
	if parsed.SignedOrder == nil {
		return nil, "", "", nil, fmt.Errorf("signer response missing signed_order")
	}
	return parsed.SignedOrder, strings.TrimSpace(parsed.Owner), strings.TrimSpace(parsed.OrderType), parsed.PostOnly, nil
}

func (e *CLOBExecutor) reconcilePlanStatus(ctx context.Context, planID uint64) error {
	if e == nil || e.Repo == nil || planID == 0 {
		return nil
	}
	planRef := planID
	orders, err := e.Repo.ListOrders(ctx, repository.ListOrdersParams{
		Limit:   1000,
		Offset:  0,
		PlanID:  &planRef,
		OrderBy: "created_at",
		Asc:     boolPtrExecutor(true),
	})
	if err != nil || len(orders) == 0 {
		return err
	}
	total := len(orders)
	filled := 0
	partial := 0
	open := 0
	failed := 0
	cancelled := 0
	for _, o := range orders {
		switch strings.ToLower(strings.TrimSpace(o.Status)) {
		case "filled":
			filled++
		case "partial":
			partial++
			open++
		case "submitted", "pending":
			open++
		case "failed":
			failed++
		case "cancelled":
			cancelled++
		}
	}
	plan, _ := e.Repo.GetExecutionPlanByID(ctx, planID)
	var oppID uint64
	if plan != nil {
		oppID = plan.OpportunityID
	}
	switch {
	case filled == total:
		now := time.Now().UTC()
		_ = e.Repo.UpdateExecutionPlanExecutedAt(ctx, planID, "executed", &now)
		if oppID > 0 {
			_ = e.Repo.UpdateOpportunityStatus(ctx, oppID, "executed")
		}
	case open > 0:
		_ = e.Repo.UpdateExecutionPlanStatus(ctx, planID, "executing")
		if oppID > 0 {
			_ = e.Repo.UpdateOpportunityStatus(ctx, oppID, "executing")
		}
	case filled > 0 && (failed > 0 || cancelled > 0 || partial > 0):
		_ = e.Repo.UpdateExecutionPlanStatus(ctx, planID, "partial")
		if oppID > 0 {
			_ = e.Repo.UpdateOpportunityStatus(ctx, oppID, "executing")
		}
	case failed == total || cancelled == total:
		_ = e.Repo.UpdateExecutionPlanStatus(ctx, planID, "failed")
		if oppID > 0 {
			_ = e.Repo.UpdateOpportunityStatus(ctx, oppID, "failed")
		}
	}
	return nil
}
