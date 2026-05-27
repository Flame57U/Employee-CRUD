package quant

// Chromosome carries all GA-evolvable parameters.
//
// The micro-engine fields (Beta/Gamma/SigmaFloor) follow Build Plan 3C's
// Sigmoid + InventoryBias formulation rather than the doc's w_min/w_max
// formulation. Other groups (market sensing, signal, macro, release) follow
// docs/纯粹策略数学引擎.md Chapter 7.
type Chromosome struct {
	// --- Market sensing ---
	NEMAFast          int     `json:"n_ema_fast"`
	NEMAMid           int     `json:"n_ema_mid"`
	NEMASlow          int     `json:"n_ema_slow"`
	NAtr              int     `json:"n_atr"`
	AtrQuietThreshold float64 `json:"atr_quiet_threshold"`
	QuietBand         float64 `json:"quiet_band"`

	// --- Signal composition ---
	AlphaTrend     float64 `json:"alpha_trend"`
	AlphaReversion float64 `json:"alpha_reversion"`
	AlphaVol       float64 `json:"alpha_vol"`
	RevertScale    float64 `json:"revert_scale"`
	AtrBaseline    float64 `json:"atr_baseline"`

	// --- Micro engine (Sigmoid dynamic balance, BP 3C verbatim) ---
	Beta       float64 `json:"beta"`         // Sigmoid steepness
	Gamma      float64 `json:"gamma"`        // inventory bias coefficient
	SigmaFloor float64 `json:"sigma_floor"`  // σ minimum (dimensionless)
	MicroDustCNY    float64 `json:"micro_dust_cny"`    // wedge dust threshold (CNY)
	DeltaWeightWedge float64 `json:"delta_weight_wedge"` // wedge breakthrough by |ΔW|
	VolRatioWedge    float64 `json:"vol_ratio_wedge"`    // wedge breakthrough by VolRatio
	BearMicroScale   float64 `json:"bear_micro_scale"`   // suppress longs in BEAR

	// --- Macro engine (DCA) ---
	MacroReservePct    float64 `json:"macro_reserve_pct"`
	MacroBaseAllocPct  float64 `json:"macro_base_alloc_pct"`
	MacroMaxSinglePct  float64 `json:"macro_max_single_pct"`
	MacroCooldownTicks int     `json:"macro_cooldown_ticks"`
	NPeakWindow        int     `json:"n_peak_window"`
	DcaBoostCoef       float64 `json:"dca_boost_coef"`
	DcaDdScale         float64 `json:"dca_dd_scale"`
	DcaConvexity       float64 `json:"dca_convexity"`
	BearMacroBoost     float64 `json:"bear_macro_boost"`

	// --- DeadHold release ---
	ReleaseProfitThreshold float64 `json:"release_profit_threshold"`
	ReleaseMinHoldDays     int     `json:"release_min_hold_days"`
	WMicroReleaseCap       float64 `json:"w_micro_release_cap"`
}

// IntBounds and FloatBounds describe inclusive [min, max] hard ranges.
type IntBounds struct{ Min, Max int }
type FloatBounds struct{ Min, Max float64 }

// HardBounds is the per-field GA hard range. Variations outside these bounds
// are clamped by ClampChromosome.
var HardBounds = struct {
	NEMAFast          IntBounds
	NEMAMid           IntBounds
	NEMASlow          IntBounds
	NAtr              IntBounds
	AtrQuietThreshold FloatBounds
	QuietBand         FloatBounds

	AlphaTrend     FloatBounds
	AlphaReversion FloatBounds
	AlphaVol       FloatBounds
	RevertScale    FloatBounds
	AtrBaseline    FloatBounds

	Beta             FloatBounds
	Gamma            FloatBounds
	SigmaFloor       FloatBounds
	MicroDustCNY     FloatBounds
	DeltaWeightWedge FloatBounds
	VolRatioWedge    FloatBounds
	BearMicroScale   FloatBounds

	MacroReservePct    FloatBounds
	MacroBaseAllocPct  FloatBounds
	MacroMaxSinglePct  FloatBounds
	MacroCooldownTicks IntBounds
	NPeakWindow        IntBounds
	DcaBoostCoef       FloatBounds
	DcaDdScale         FloatBounds
	DcaConvexity       FloatBounds
	BearMacroBoost     FloatBounds

	ReleaseProfitThreshold FloatBounds
	ReleaseMinHoldDays     IntBounds
	WMicroReleaseCap       FloatBounds
}{
	NEMAFast:          IntBounds{5, 30},
	NEMAMid:           IntBounds{20, 100},
	NEMASlow:          IntBounds{60, 250},
	NAtr:              IntBounds{7, 30},
	AtrQuietThreshold: FloatBounds{0.001, 0.015},
	QuietBand:         FloatBounds{0.005, 0.05},

	AlphaTrend:     FloatBounds{0.10, 0.80},
	AlphaReversion: FloatBounds{0.10, 0.70},
	AlphaVol:       FloatBounds{0.05, 0.50},
	RevertScale:    FloatBounds{0.3, 3.0},
	AtrBaseline:    FloatBounds{0.005, 0.05},

	Beta:             FloatBounds{0.5, 6.0},
	Gamma:            FloatBounds{0.0, 4.0},
	SigmaFloor:       FloatBounds{1e-4, 0.05},
	MicroDustCNY:     FloatBounds{100, 2000},
	DeltaWeightWedge: FloatBounds{0.001, 0.05},
	VolRatioWedge:    FloatBounds{1.0, 3.0},
	BearMicroScale:   FloatBounds{0.0, 1.0},

	MacroReservePct:    FloatBounds{0.02, 0.20},
	MacroBaseAllocPct:  FloatBounds{0.02, 0.30},
	MacroMaxSinglePct:  FloatBounds{0.10, 0.60},
	MacroCooldownTicks: IntBounds{1, 30},
	NPeakWindow:        IntBounds{30, 500},
	DcaBoostCoef:       FloatBounds{0.5, 5.0},
	DcaDdScale:         FloatBounds{0.10, 0.40},
	DcaConvexity:       FloatBounds{1.0, 4.0},
	BearMacroBoost:     FloatBounds{1.0, 3.0},

	ReleaseProfitThreshold: FloatBounds{0.05, 0.50},
	ReleaseMinHoldDays:     IntBounds{30, 365},
	WMicroReleaseCap:       FloatBounds{0.05, 0.25},
}

func clipI(v int, b IntBounds) int {
	if v < b.Min {
		return b.Min
	}
	if v > b.Max {
		return b.Max
	}
	return v
}

func clipF(v float64, b FloatBounds) float64 {
	if v < b.Min {
		return b.Min
	}
	if v > b.Max {
		return b.Max
	}
	return v
}

// ClampChromosome enforces both element-wise hard bounds and structural
// constraints from doc §7.3. Always callable post-mutation.
func ClampChromosome(c Chromosome) Chromosome {
	// Element-wise bounds
	c.NEMAFast = clipI(c.NEMAFast, HardBounds.NEMAFast)
	c.NEMAMid = clipI(c.NEMAMid, HardBounds.NEMAMid)
	c.NEMASlow = clipI(c.NEMASlow, HardBounds.NEMASlow)
	c.NAtr = clipI(c.NAtr, HardBounds.NAtr)
	c.AtrQuietThreshold = clipF(c.AtrQuietThreshold, HardBounds.AtrQuietThreshold)
	c.QuietBand = clipF(c.QuietBand, HardBounds.QuietBand)

	c.AlphaTrend = clipF(c.AlphaTrend, HardBounds.AlphaTrend)
	c.AlphaReversion = clipF(c.AlphaReversion, HardBounds.AlphaReversion)
	c.AlphaVol = clipF(c.AlphaVol, HardBounds.AlphaVol)
	c.RevertScale = clipF(c.RevertScale, HardBounds.RevertScale)
	c.AtrBaseline = clipF(c.AtrBaseline, HardBounds.AtrBaseline)

	c.Beta = clipF(c.Beta, HardBounds.Beta)
	c.Gamma = clipF(c.Gamma, HardBounds.Gamma)
	c.SigmaFloor = clipF(c.SigmaFloor, HardBounds.SigmaFloor)
	c.MicroDustCNY = clipF(c.MicroDustCNY, HardBounds.MicroDustCNY)
	c.DeltaWeightWedge = clipF(c.DeltaWeightWedge, HardBounds.DeltaWeightWedge)
	c.VolRatioWedge = clipF(c.VolRatioWedge, HardBounds.VolRatioWedge)
	c.BearMicroScale = clipF(c.BearMicroScale, HardBounds.BearMicroScale)

	c.MacroReservePct = clipF(c.MacroReservePct, HardBounds.MacroReservePct)
	c.MacroBaseAllocPct = clipF(c.MacroBaseAllocPct, HardBounds.MacroBaseAllocPct)
	c.MacroMaxSinglePct = clipF(c.MacroMaxSinglePct, HardBounds.MacroMaxSinglePct)
	c.MacroCooldownTicks = clipI(c.MacroCooldownTicks, HardBounds.MacroCooldownTicks)
	c.NPeakWindow = clipI(c.NPeakWindow, HardBounds.NPeakWindow)
	c.DcaBoostCoef = clipF(c.DcaBoostCoef, HardBounds.DcaBoostCoef)
	c.DcaDdScale = clipF(c.DcaDdScale, HardBounds.DcaDdScale)
	c.DcaConvexity = clipF(c.DcaConvexity, HardBounds.DcaConvexity)
	c.BearMacroBoost = clipF(c.BearMacroBoost, HardBounds.BearMacroBoost)

	c.ReleaseProfitThreshold = clipF(c.ReleaseProfitThreshold, HardBounds.ReleaseProfitThreshold)
	c.ReleaseMinHoldDays = clipI(c.ReleaseMinHoldDays, HardBounds.ReleaseMinHoldDays)
	c.WMicroReleaseCap = clipF(c.WMicroReleaseCap, HardBounds.WMicroReleaseCap)

	// C_struct_1: EMA ordering (relativity lock)
	if c.NEMAMid <= c.NEMAFast {
		c.NEMAMid = c.NEMAFast + 1
		if c.NEMAMid > HardBounds.NEMAMid.Max {
			c.NEMAMid = HardBounds.NEMAMid.Max
			c.NEMAFast = c.NEMAMid - 1
		}
	}
	if c.NEMASlow <= c.NEMAMid {
		c.NEMASlow = c.NEMAMid + 1
		if c.NEMASlow > HardBounds.NEMASlow.Max {
			c.NEMASlow = HardBounds.NEMASlow.Max
			c.NEMAMid = c.NEMASlow - 1
		}
	}

	// C_struct_2: alpha normalisation
	sum := c.AlphaTrend + c.AlphaReversion + c.AlphaVol
	if sum > 0 {
		c.AlphaTrend /= sum
		c.AlphaReversion /= sum
		c.AlphaVol /= sum
	} else {
		c.AlphaTrend, c.AlphaReversion, c.AlphaVol = 1.0/3, 1.0/3, 1.0/3
	}

	// C_struct_5: base_alloc ≤ max_single
	if c.MacroBaseAllocPct > c.MacroMaxSinglePct {
		c.MacroBaseAllocPct = c.MacroMaxSinglePct
	}

	return c
}

// DefaultSeedChromosome is the champion seed used at GA cold start and as a
// fallback for JSON decode failures.
var DefaultSeedChromosome = Chromosome{
	NEMAFast:          10,
	NEMAMid:           30,
	NEMASlow:          90,
	NAtr:              14,
	AtrQuietThreshold: 0.005,
	QuietBand:         0.02,

	AlphaTrend:     0.50,
	AlphaReversion: 0.30,
	AlphaVol:       0.20,
	RevertScale:    1.0,
	AtrBaseline:    0.015,

	Beta:             2.0,
	Gamma:            1.0,
	SigmaFloor:      0.001,
	MicroDustCNY:    500,
	DeltaWeightWedge: 0.01,
	VolRatioWedge:    1.5,
	BearMicroScale:   0.5,

	MacroReservePct:    0.05,
	MacroBaseAllocPct:  0.10,
	MacroMaxSinglePct:  0.30,
	MacroCooldownTicks: 5,
	NPeakWindow:        120,
	DcaBoostCoef:       2.0,
	DcaDdScale:         0.20,
	DcaConvexity:       2.0,
	BearMacroBoost:     1.5,

	ReleaseProfitThreshold: 0.15,
	ReleaseMinHoldDays:     90,
	WMicroReleaseCap:       0.12,
}

// SpawnPoint groups Epoch parameters that are frozen at instance creation and
// never participate in GA. Modifying any field requires a new strategy version.
type SpawnPoint struct {
	Policy Policy
	Risk   Risk
}

// Policy is the Epoch-level capital policy bundle.
type Policy struct {
	InstanceID         string
	Symbol             string
	AssetClass         string // "A_STOCK_ETF" | "GOLD_ETF"
	TotalCapitalCNY    float64
	MonthlyInjectCNY   float64
	DeadlineRatio      float64 // dead-money deadline override ratio
	MacroMinOrderCNY   float64
	QuietDustThreshold float64
	MaxLotsPerTick     int
	ReleaseTrigger     float64 // release threshold override
	CreatedAt          int64
}

// Risk is the Epoch-level risk-boundary bundle.
type Risk struct {
	FeeRate           float64 // round-trip fee rate (e.g. 0.0001)
	GlobalStopLoss    float64 // hard equity-drawdown stop
	MaxDailyDrawdown  float64 // intra-day drawdown cap
	MaxConsecLoseDays int
}
