package quant

import "testing"

func TestDefaultSeedClampIsStable(t *testing.T) {
	c := ClampChromosome(DefaultSeedChromosome)
	if c.NEMAFast >= c.NEMAMid || c.NEMAMid >= c.NEMASlow {
		t.Fatalf("EMA ordering violated: %d/%d/%d", c.NEMAFast, c.NEMAMid, c.NEMASlow)
	}
	sum := c.AlphaTrend + c.AlphaReversion + c.AlphaVol
	if sum < 0.999 || sum > 1.001 {
		t.Fatalf("alpha weights not normalised: sum=%f", sum)
	}
	if c.MacroBaseAllocPct > c.MacroMaxSinglePct {
		t.Fatalf("base alloc > max single: %f > %f", c.MacroBaseAllocPct, c.MacroMaxSinglePct)
	}
}

func TestClampFixesEMAOrderViolation(t *testing.T) {
	c := DefaultSeedChromosome
	c.NEMAFast = 50
	c.NEMAMid = 40
	c.NEMASlow = 30
	c = ClampChromosome(c)
	if !(c.NEMAFast < c.NEMAMid && c.NEMAMid < c.NEMASlow) {
		t.Fatalf("expected ordering, got %d/%d/%d", c.NEMAFast, c.NEMAMid, c.NEMASlow)
	}
}

func TestClampPinsOutOfRange(t *testing.T) {
	c := DefaultSeedChromosome
	c.Beta = 999
	c.Gamma = -10
	c = ClampChromosome(c)
	if c.Beta != HardBounds.Beta.Max {
		t.Fatalf("Beta not pinned: %f", c.Beta)
	}
	if c.Gamma != HardBounds.Gamma.Min {
		t.Fatalf("Gamma not pinned: %f", c.Gamma)
	}
}

func TestClampHandlesZeroAlphaSum(t *testing.T) {
	c := DefaultSeedChromosome
	c.AlphaTrend = 0
	c.AlphaReversion = 0
	c.AlphaVol = 0
	c = ClampChromosome(c)
	// All three get pinned to their HardBounds.Min, so post-pin sum is > 0
	// and normalisation produces a deterministic result.
	sum := c.AlphaTrend + c.AlphaReversion + c.AlphaVol
	if sum < 0.999 || sum > 1.001 {
		t.Fatalf("normalised sum should be 1, got %f", sum)
	}
}
