package quant

import "testing"

func TestLotTotals(t *testing.T) {
	lots := []SpotLot{
		{LotID: "a", LotType: LotDeadStack, Amount: 100, IsColdSealed: false},
		{LotID: "b", LotType: LotDeadStack, Amount: 50, IsColdSealed: true}, // cold sealed
		{LotID: "c", LotType: LotFloating, Amount: 30},
	}
	if got := DeadHoldTotal(lots); got != 100 {
		t.Fatalf("DeadHoldTotal = %f, want 100", got)
	}
	if got := FloatHoldTotal(lots); got != 30 {
		t.Fatalf("FloatHoldTotal = %f, want 30", got)
	}
	if got := ColdSealedHoldTotal(lots); got != 50 {
		t.Fatalf("ColdSealedHoldTotal = %f, want 50", got)
	}
}

func TestSoftReleaseRespectsColdSealed(t *testing.T) {
	now := int64(365 * 86400 * 5) // 5y in seconds
	old := now - 365*86400        // 1y ago
	lots := []SpotLot{
		{LotID: "cold", LotType: LotDeadStack, Amount: 1000, CreatedAt: old, IsColdSealed: true},
		{LotID: "warm", LotType: LotDeadStack, Amount: 1000, CreatedAt: old, IsColdSealed: false},
	}
	out, events := SoftRelease(lots, SoftReleaseConfig{
		NowUnix:         now,
		MinAgeMonths:    6,
		MaxReleaseRatio: 0.5,
		SellGap:         200,
	})
	// cold-sealed lot must NOT be released.
	for _, l := range out {
		if l.LotID == "cold" && l.LotType != LotDeadStack {
			t.Fatalf("cold-sealed lot was released: %+v", l)
		}
	}
	if len(events) == 0 {
		t.Fatal("expected at least one release event from warm lot")
	}
}

func TestHardReleaseConvertsFromDeadStack(t *testing.T) {
	lots := []SpotLot{
		{LotID: "d1", LotType: LotDeadStack, Amount: 100},
		{LotID: "d2", LotType: LotDeadStack, Amount: 100, IsColdSealed: true},
	}
	out, events, remaining := HardRelease(lots, 50, 1700000000)
	if remaining != 0 {
		t.Fatalf("expected fully filled, remaining=%f", remaining)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	// d2 (cold-sealed) must be untouched
	for _, l := range out {
		if l.LotID == "d2" && l.LotType != LotDeadStack {
			t.Fatalf("cold-sealed lot got released by HardRelease")
		}
	}
}
