// Package ws implements the SaaS-side WebSocket Hub that accepts Agent
// connections, dispatches TradeCommands, and ingests DeltaReports.
//
// Protocol reference: docs/QuanSaas系统总体拓扑结构.md §5.
// Design tenet: "云端只信上报，端侧无脑执行" — SaaS is the authoritative
// portfolio ledger; the Agent merely executes and reports.
package ws

import "encoding/json"

// WS envelope types (Agent → SaaS).
const (
	MsgAuth        = "auth"
	MsgHeartbeat   = "heartbeat"
	MsgCommandAck  = "command_ack"
	MsgDeltaReport = "delta_report"
)

// WS envelope types (SaaS → Agent).
const (
	MsgAuthResult   = "auth_result"
	MsgHeartbeatAck = "heartbeat_ack"
	MsgCommand      = "command"
	MsgReportAck    = "report_ack"
)

// Envelope is the top-level wire wrapper for every WebSocket message.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// outEnvelope is used when sending — Payload is any so we can encode in-place.
type outEnvelope struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// AuthMsg carries the agent JWT.
type AuthMsg struct {
	JWT string `json:"jwt"`
}

// AuthResultMsg is returned to the agent after authentication.
type AuthResultMsg struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// CommandAckMsg acknowledges receipt (not execution) of a TradeCommand.
type CommandAckMsg struct {
	ClientOrderID string `json:"client_order_id"`
}

// ReportAckMsg acknowledges processing of a DeltaReport.
type ReportAckMsg struct {
	ClientOrderID string `json:"client_order_id,omitempty"`
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
}

// TradeCommand is the unit of work pushed from SaaS to Agent.
// Field semantics are defined in docs/QuanSaas系统总体拓扑结构.md §5.3.
type TradeCommand struct {
	ClientOrderID string `json:"client_order_id"` // inst{id}-{type}-{ts}
	Action        string `json:"action"`          // BUY / SELL
	Engine        string `json:"engine"`          // MACRO / MICRO
	Symbol        string `json:"symbol"`          // e.g. 510300.SH
	AmountCNY     string `json:"amount_cny"`      // buy-side spend (string to avoid fp drift)
	QtyAsset      string `json:"qty_asset"`       // sell-side quantity
	LotType       string `json:"lot_type"`        // DEAD_STACK / FLOATING
}

// Execution mirrors broker fill detail reported by the Agent.
type Execution struct {
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Action        string  `json:"action"`
	FilledQty     float64 `json:"filled_qty"`
	FilledPrice   float64 `json:"filled_price"`
	Fee           float64 `json:"fee"`
	Status        string  `json:"status"` // filled / failed
}

// Balance is one asset slice in the broker account snapshot.
type Balance struct {
	Asset     string  `json:"asset"`
	Available float64 `json:"available"`
	Frozen    float64 `json:"frozen"`
}

// DeltaReport is the canonical Agent → SaaS state update.
// ClientOrderID/Execution are empty when this is a reconnect snapshot.
type DeltaReport struct {
	ClientOrderID string     `json:"client_order_id,omitempty"`
	Execution     *Execution `json:"execution,omitempty"`
	Balances      []Balance  `json:"balances"`
}
