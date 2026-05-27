package quant

import "math"

const secondsPerDay int64 = 86400

// SpotLot is one buy batch tracked at lot granularity.
type SpotLot struct {
	LotID        string
	LotType      LotType
	Amount       float64
	CostPrice    float64
	CreatedAt    int64
	IsColdSealed bool
}

// DeadHoldTotal sums non-cold-sealed DEAD_STACK lot amounts.
func DeadHoldTotal(lots []SpotLot) float64 {
	var s float64
	for _, lot := range lots {
		if lot.LotType == LotDeadStack && !lot.IsColdSealed {
			s += lot.Amount
		}
	}
	return s
}

// FloatHoldTotal sums FLOATING lot amounts.
func FloatHoldTotal(lots []SpotLot) float64 {
	var s float64
	for _, lot := range lots {
		if lot.LotType == LotFloating {
			s += lot.Amount
		}
	}
	return s
}

// ColdSealedHoldTotal sums lots flagged IsColdSealed (any LotType).
func ColdSealedHoldTotal(lots []SpotLot) float64 {
	var s float64
	for _, lot := range lots {
		if lot.IsColdSealed {
			s += lot.Amount
		}
	}
	return s
}

// SoftReleaseConfig controls SoftRelease behaviour.
type SoftReleaseConfig struct {
	NowUnix         int64
	MinAgeMonths    int
	MaxReleaseRatio float64 // [0,1] of total DeadHold released this tick
	SellGap         float64 // how much extra FloatHold is "wanted" (in asset units)
}

// SoftRelease moves up to (MaxReleaseRatio × deadTotal) and (SellGap) amount
// from aged, non-ColdSealed DEAD_STACK lots into FLOATING lots, preserving
// CostPrice. Returns the mutated lot slice and emitted ReleaseEvents.
//
// Iron rule: lots with IsColdSealed=true are never touched.
func SoftRelease(lots []SpotLot, cfg SoftReleaseConfig) ([]SpotLot, []ReleaseEvent) {
	if cfg.SellGap <= 0 || cfg.MaxReleaseRatio <= 0 {
		return lots, nil
	}
	ageCutoff := cfg.NowUnix - int64(cfg.MinAgeMonths)*30*secondsPerDay
	deadTotal := DeadHoldTotal(lots)
	maxByRatio := deadTotal * cfg.MaxReleaseRatio
	budget := math.Min(cfg.SellGap, maxByRatio)
	if budget <= 0 {
		return lots, nil
	}

	out := make([]SpotLot, 0, len(lots)+4)
	events := make([]ReleaseEvent, 0)
	for _, lot := range lots {
		if budget <= 0 ||
			lot.LotType != LotDeadStack ||
			lot.IsColdSealed ||
			lot.CreatedAt > ageCutoff {
			out = append(out, lot)
			continue
		}
		take := math.Min(lot.Amount, budget)
		if take >= lot.Amount {
			released := lot
			released.LotType = LotFloating
			out = append(out, released)
			events = append(events, ReleaseEvent{
				LotID:       lot.LotID,
				QtyAsset:    lot.Amount,
				FromState:   LotDeadStack,
				ToState:     LotFloating,
				ReleaseTime: cfg.NowUnix,
				AuditNote:   "soft release (aged, ratio-bounded)",
			})
			budget -= take
		} else {
			remaining := lot
			remaining.Amount = lot.Amount - take
			out = append(out, remaining)
			released := lot
			released.LotID = lot.LotID + "_sr"
			released.LotType = LotFloating
			released.Amount = take
			out = append(out, released)
			events = append(events, ReleaseEvent{
				LotID:       released.LotID,
				QtyAsset:    take,
				FromState:   LotDeadStack,
				ToState:     LotFloating,
				ReleaseTime: cfg.NowUnix,
				AuditNote:   "soft release (partial, aged, ratio-bounded)",
			})
			budget = 0
		}
	}
	return out, events
}

// HardRelease covers a micro-engine sell intent when FloatHold is insufficient,
// pulling from any non-ColdSealed DEAD_STACK lot (FIFO by CreatedAt) regardless
// of age. Returns updated lots, emitted events, and the unfilled shortfall.
func HardRelease(lots []SpotLot, shortfall float64, nowUnix int64) ([]SpotLot, []ReleaseEvent, float64) {
	if shortfall <= 0 {
		return lots, nil, 0
	}
	out := make([]SpotLot, 0, len(lots)+4)
	events := make([]ReleaseEvent, 0)
	remaining := shortfall
	for _, lot := range lots {
		if remaining <= 0 ||
			lot.LotType != LotDeadStack ||
			lot.IsColdSealed {
			out = append(out, lot)
			continue
		}
		take := math.Min(lot.Amount, remaining)
		if take >= lot.Amount {
			released := lot
			released.LotType = LotFloating
			out = append(out, released)
			events = append(events, ReleaseEvent{
				LotID:       lot.LotID,
				QtyAsset:    lot.Amount,
				FromState:   LotDeadStack,
				ToState:     LotFloating,
				ReleaseTime: nowUnix,
				AuditNote:   "hard release (sell shortfall)",
			})
			remaining -= take
		} else {
			rest := lot
			rest.Amount = lot.Amount - take
			out = append(out, rest)
			released := lot
			released.LotID = lot.LotID + "_hr"
			released.LotType = LotFloating
			released.Amount = take
			out = append(out, released)
			events = append(events, ReleaseEvent{
				LotID:       released.LotID,
				QtyAsset:    take,
				FromState:   LotDeadStack,
				ToState:     LotFloating,
				ReleaseTime: nowUnix,
				AuditNote:   "hard release (partial, sell shortfall)",
			})
			remaining = 0
		}
	}
	return out, events, remaining
}
