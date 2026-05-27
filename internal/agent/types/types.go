package types

// TradeCommand is received from SaaS via WebSocket (type: "command").
type TradeCommand struct {
	ClientOrderID string `json:"client_order_id"`
	Action        string `json:"action"`    // BUY / SELL
	Engine        string `json:"engine"`    // MACRO / MICRO
	Symbol        string `json:"symbol"`    // e.g. 510300.SH
	AmountCNY     string `json:"amount_cny"` // buy: CNY spend amount
	QtyAsset      string `json:"qty_asset"`  // sell: share quantity
	LotType       string `json:"lot_type"`   // DEAD_STACK / FLOATING
}

// Execution is the broker fill report for one market order.
type Execution struct {
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Action        string  `json:"action"`
	FilledQty     float64 `json:"filled_qty"`
	FilledPrice   float64 `json:"filled_price"`
	Fee           float64 `json:"fee"`
	Status        string  `json:"status"` // filled / failed
}

// Balance represents one asset position in the broker account.
type Balance struct {
	Asset     string  `json:"asset"`
	Available float64 `json:"available"`
	Frozen    float64 `json:"frozen"`
}

// DeltaReport is sent to SaaS after order execution, or as an initial
// balance snapshot on reconnect (ClientOrderID and Execution are empty).
type DeltaReport struct {
	ClientOrderID string     `json:"client_order_id,omitempty"`
	Execution     *Execution `json:"execution,omitempty"`
	Balances      []Balance  `json:"balances"`
}

// WS envelope types (Agent → SaaS).
const (
	MsgAuth        = "auth"
	MsgHeartbeat   = "heartbeat"
	MsgCommandAck  = "command_ack"
	MsgDeltaReport = "delta_report"
)

// WS envelope types (SaaS → Agent).
const (
	MsgAuthResult  = "auth_result"
	MsgHeartbeatAck = "heartbeat_ack"
	MsgCommand     = "command"
	MsgReportAck   = "report_ack"
)

// Envelope is the top-level WebSocket message wrapper.
type Envelope struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// AuthMsg is sent immediately after the WebSocket connection is established.
type AuthMsg struct {
	JWT string `json:"jwt"`
}

// AuthResultMsg is received from SaaS after authentication.
type AuthResultMsg struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// CommandAckMsg acknowledges a received TradeCommand without waiting for execution.
type CommandAckMsg struct {
	ClientOrderID string `json:"client_order_id"`
}
