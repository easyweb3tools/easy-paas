package labeler

import (
	"testing"

	"polymarket/internal/models"
)

func TestNormalizeCity(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"NYC", "new-york"},
		{"new york", "new-york"},
		{"Los Angeles", "los-angeles"},
		{"LA", "los-angeles"},
		{"london", "london"},
	}
	for _, tt := range tests {
		if got := normalizeCity(tt.in); got != tt.want {
			t.Fatalf("normalizeCity(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWeatherRuleSubLabel(t *testing.T) {
	rules := DefaultRules()
	var weather *LabelRule
	for i := range rules {
		if rules[i].Label == "weather" {
			weather = &rules[i]
			break
		}
	}
	if weather == nil {
		t.Fatalf("weather rule missing")
	}
	l := &MarketLabeler{Rules: rules}
	l.compile()
	for i := range l.Rules {
		if l.Rules[i].Label == "weather" {
			weather = &l.Rules[i]
			break
		}
	}
	if weather == nil {
		t.Fatalf("compiled weather rule missing")
	}
	title := "Temperature in NYC on Feb 20?"
	sub := matchSubLabel(*weather, title)
	if sub == nil || *sub != "new-york" {
		t.Fatalf("expected sublabel new-york, got %#v", sub)
	}
}

func TestMatchTags(t *testing.T) {
	rule := LabelRule{
		Label:    "sports",
		TagMatch: []string{"Sports", "NBA"},
	}
	if !matchTags(rule, []models.Tag{{Label: "NBA"}}) {
		t.Fatalf("expected true")
	}
	if matchTags(rule, []models.Tag{{Label: "Politics"}}) {
		t.Fatalf("expected false")
	}
}

func TestBroadLabelRules(t *testing.T) {
	rules := DefaultRules()
	l := &MarketLabeler{Rules: rules}
	l.compile()

	tests := []struct {
		title string
		want  string
	}{
		{"Will the 2024 presidential election be won by Biden?", "election"},
		{"Will voters approve the ballot measure in November?", "election"},
		{"Will the SEC approve the Bitcoin ETF?", "regulation"},
		{"Will the FDA approve the new drug before March?", "regulation"},
		{"Will the TGE for Project X happen before Q2?", "tge_deadline"},
		{"Will the token launch before July 2025?", "tge_deadline"},
		{"Will GDP growth exceed 3% in Q1?", "macro_economic"},
		{"Will the Fed rate be above 5% in June?", "macro_economic"},
		{"Will CPI come in below 3% for January?", "macro_economic"},
	}

	ruleByLabel := map[string]*LabelRule{}
	for i := range l.Rules {
		ruleByLabel[l.Rules[i].Label] = &l.Rules[i]
	}

	for _, tt := range tests {
		rule := ruleByLabel[tt.want]
		if rule == nil {
			t.Fatalf("missing rule for label %q", tt.want)
		}
		if !matchAny(*rule, tt.title) {
			t.Errorf("expected label %q to match title %q", tt.want, tt.title)
		}
	}
}
