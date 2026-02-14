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
