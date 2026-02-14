package service

import "testing"

func TestExtractBinarySettlement_BasicYes(t *testing.T) {
	raw := []byte(`{"resolution":"YES","resolvedAt":"2026-02-14T00:00:00Z"}`)
	outcome, settledAt, _, _, err := extractBinarySettlement(raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if outcome != "YES" {
		t.Fatalf("outcome=%q want YES", outcome)
	}
	if settledAt.IsZero() {
		t.Fatalf("settledAt is zero")
	}
}

func TestExtractBinarySettlement_BasicNo(t *testing.T) {
	raw := []byte(`{"resolvedOutcome":"No"}`)
	outcome, _, _, _, err := extractBinarySettlement(raw)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if outcome != "NO" {
		t.Fatalf("outcome=%q want NO", outcome)
	}
}

func TestExtractBinarySettlement_Missing(t *testing.T) {
	raw := []byte(`{"foo":"bar"}`)
	_, _, _, _, err := extractBinarySettlement(raw)
	if err == nil {
		t.Fatalf("expected error")
	}
}
