package labeler

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type MarketLabeler struct {
	Rules  []LabelRule
	Repo   repository.Repository
	Logger *zap.Logger
}

type LabelRule struct {
	Label      string
	TitleRegex []string
	TagMatch   []string
	Confidence float64

	compiled []*regexp.Regexp
}

func DefaultRules() []LabelRule {
	return []LabelRule{
		{
			Label: "weather",
			TitleRegex: []string{
				`(?i)temperature.*in\s+(nyc|new york|london|los angeles|la)`,
				`(?i)highest.*temp`,
				`(?i)weather.*forecast`,
			},
			Confidence: 0.95,
		},
		{
			Label: "btc_15min",
			TitleRegex: []string{
				`(?i)bitcoin.*(up|down).*\d+\s*(min|minute)`,
				`(?i)btc.*(up|down)`,
			},
			Confidence: 0.95,
		},
		{
			Label: "pre_market_fdv",
			TitleRegex: []string{
				`(?i)(fdv|fully diluted|market cap).*\$?\d+[bmk]`,
				`(?i)(tge|token generation|launch).*before`,
			},
			TagMatch:   []string{"Crypto", "DeFi", "Token Launch"},
			Confidence: 0.85,
		},
		{
			Label: "geopolitical",
			TitleRegex: []string{
				`(?i)(war|strike|attack|invade|sanction|ban)`,
				`(?i)(iran|russia|china|north korea|ukraine).*before`,
			},
			TagMatch:   []string{"Politics", "Geopolitics"},
			Confidence: 0.80,
		},
		{
			Label: "safe_no",
			TitleRegex: []string{
				`(?i)(jesus.*return|alien.*exist|ufo.*confirm|rapture)`,
				`(?i)(zombie|vampire|werewolf).*before`,
			},
			Confidence: 0.99,
		},
		{
			Label: "app_store",
			TitleRegex: []string{
				`(?i)#?\d+\s*(free\s+)?app.*app\s*store`,
			},
			Confidence: 0.95,
		},
		{
			Label:      "sports",
			TagMatch:   []string{"Sports", "NBA", "NFL", "MLB", "Soccer", "Tennis"},
			Confidence: 0.95,
		},
		{
			Label: "crypto_price",
			TitleRegex: []string{
				`(?i)(bitcoin|btc|eth|ethereum|sol|solana).*price.*above`,
				`(?i)(bitcoin|btc|eth|ethereum).*\$\d+`,
			},
			TagMatch:   []string{"Crypto"},
			Confidence: 0.85,
		},
	}
}

func (l *MarketLabeler) compile() {
	for i := range l.Rules {
		if len(l.Rules[i].compiled) > 0 {
			continue
		}
		for _, raw := range l.Rules[i].TitleRegex {
			re, err := regexp.Compile(raw)
			if err != nil {
				if l.Logger != nil {
					l.Logger.Warn("label rule regex compile failed", zap.String("label", l.Rules[i].Label), zap.String("regex", raw), zap.Error(err))
				}
				continue
			}
			l.Rules[i].compiled = append(l.Rules[i].compiled, re)
		}
	}
}

// LabelMarkets scans active markets and writes missing labels.
// MVP: uses market.Question only; tag matching is planned but not wired to DB joins yet.
func (l *MarketLabeler) LabelMarkets(ctx context.Context) error {
	if l == nil || l.Repo == nil {
		return nil
	}
	if len(l.Rules) == 0 {
		l.Rules = DefaultRules()
	}
	l.compile()

	const pageSize = 500
	offset := 0
	active := true
	closed := false
	for {
		markets, err := l.Repo.ListMarkets(ctx, repository.ListMarketsParams{
			Limit:   pageSize,
			Offset:  offset,
			Active:  &active,
			Closed:  &closed,
			OrderBy: "external_updated_at",
			Asc:     boolPtr(false),
		})
		if err != nil {
			return err
		}
		if len(markets) == 0 {
			break
		}
		eventIDs := make([]string, 0, len(markets))
		seen := map[string]struct{}{}
		for _, m := range markets {
			if m.EventID == "" {
				continue
			}
			if _, ok := seen[m.EventID]; ok {
				continue
			}
			seen[m.EventID] = struct{}{}
			eventIDs = append(eventIDs, m.EventID)
		}
		tagsByEvent, err := l.Repo.ListTagsByEventIDs(ctx, eventIDs)
		if err != nil {
			return err
		}
		for _, market := range markets {
			eventTags := tagsByEvent[market.EventID]
			if err := l.labelMarket(ctx, market, eventTags); err != nil {
				if l.Logger != nil {
					l.Logger.Warn("label market failed", zap.String("market_id", market.ID), zap.Error(err))
				}
			}
		}
		if len(markets) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil
}

func (l *MarketLabeler) labelMarket(ctx context.Context, market models.Market, eventTags []models.Tag) error {
	title := strings.TrimSpace(market.Question)
	if title == "" {
		return nil
	}
	for _, rule := range l.Rules {
		subLabel := matchSubLabel(rule, title)
		if subLabel == nil && !matchAny(rule, title) && !matchTags(rule, eventTags) {
			continue
		}
		item := &models.MarketLabel{
			MarketID:    market.ID,
			Label:       rule.Label,
			SubLabel:    subLabel,
			AutoLabeled: true,
			Confidence:  rule.Confidence,
			CreatedAt:   time.Now().UTC(),
		}
		if err := l.Repo.UpsertMarketLabel(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func matchAny(rule LabelRule, title string) bool {
	for _, re := range rule.compiled {
		if re.MatchString(title) {
			return true
		}
	}
	return false
}

func matchTags(rule LabelRule, tags []models.Tag) bool {
	if len(rule.TagMatch) == 0 || len(tags) == 0 {
		return false
	}
	want := map[string]struct{}{}
	for _, t := range rule.TagMatch {
		key := strings.ToLower(strings.TrimSpace(t))
		if key != "" {
			want[key] = struct{}{}
		}
	}
	for _, tag := range tags {
		key := strings.ToLower(strings.TrimSpace(tag.Label))
		if key == "" {
			continue
		}
		if _, ok := want[key]; ok {
			return true
		}
	}
	return false
}

func matchSubLabel(rule LabelRule, title string) *string {
	// Only extract sublabels where useful; keep MVP simple.
	switch rule.Label {
	case "weather":
		for _, re := range rule.compiled {
			m := re.FindStringSubmatch(title)
			if len(m) >= 2 && strings.TrimSpace(m[1]) != "" {
				city := normalizeCity(m[1])
				return &city
			}
		}
	}
	return nil
}

func normalizeCity(val string) string {
	s := strings.ToLower(strings.TrimSpace(val))
	switch s {
	case "nyc", "new york":
		return "new-york"
	case "los angeles", "la":
		return "los-angeles"
	default:
		s = strings.ReplaceAll(s, " ", "-")
		return s
	}
}

func boolPtr(v bool) *bool { return &v }
