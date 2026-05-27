package quant

// Bar is a single OHLCV K-line.
type Bar struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// OrderAction enumerates the trading intents the engine can emit.
type OrderAction string

const (
	OrderBuy  OrderAction = "BUY"
	OrderSell OrderAction = "SELL"
)

// LotType enumerates the three asset states.
type LotType string

const (
	LotDeadStack   LotType = "DEAD_STACK"
	LotFloating    LotType = "FLOATING"
	LotColdSealed  LotType = "COLD_SEALED"
)

// Order is one trading intent produced by Step().
type Order struct {
	Action    OrderAction
	LotType   LotType
	AmountCNY float64
	QtyAsset  float64
	Symbol    string
}

// ReleaseEvent records a DeadHold → FloatHold semantic transition.
type ReleaseEvent struct {
	LotID       string
	QtyAsset    float64
	FromState   LotType
	ToState     LotType
	ReleaseTime int64
	AuditNote   string
}

// RuntimeState carries SaaS-side persistent counters into Step().
type RuntimeState struct {
	TicksSinceLastMacro int
	LastMacroTimestamp  int64
}

// PortfolioSnapshot is the account snapshot fed into Step().
type PortfolioSnapshot struct {
	CNYBalance     float64
	DeadHold       float64
	FloatHold      float64
	ColdSealedHold float64
	Lots           []SpotLot
}

// StrategyInput is the complete state snapshot fed to Step().
type StrategyInput struct {
	Symbol      string
	Closes      []float64
	Timestamps  []int64
	CurrentTime int64
	CurrentPrice float64
	Portfolio   PortfolioSnapshot
	Params      Chromosome
	Spawn       SpawnPoint
	Runtime     RuntimeState
}

// StrategyOutput is the complete intent set produced by Step().
type StrategyOutput struct {
	MacroOrders   []Order
	MicroOrders   []Order
	ReleaseEvents []ReleaseEvent
	MarketState   MarketState
}
