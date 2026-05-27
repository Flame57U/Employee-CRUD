// Package ws implements the LocalAgent WebSocket main loop with automatic
// reconnection and exponential back-off.
package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/quantsaas/platform/internal/agent/types"
)

const (
	heartbeatInterval = 30 * time.Second

	reconnectInitial = 1 * time.Second
	reconnectMax     = 5 * time.Minute

	// dial / write deadlines
	dialTimeout  = 10 * time.Second
	writeTimeout = 5 * time.Second
)

// BrokerClient is the minimal broker interface the WS client depends on.
type BrokerClient interface {
	PlaceOrder(cmd types.TradeCommand) (types.Execution, error)
	GetBalances() ([]types.Balance, error)
}

// AgentClient is the WebSocket main loop for the LocalAgent.
type AgentClient struct {
	saasURL  string
	email    string
	password string
	broker   BrokerClient

	writeMu sync.Mutex // serialises concurrent writes to the WS connection
}

// New constructs an AgentClient.
func New(saasURL, email, password string, broker BrokerClient) *AgentClient {
	return &AgentClient{
		saasURL:  saasURL,
		email:    email,
		password: password,
		broker:   broker,
	}
}

// Run starts the agent main loop. It blocks until ctx is cancelled.
// On any connection failure it reconnects with exponential back-off.
func (a *AgentClient) Run(ctx context.Context) {
	delay := reconnectInitial
	for {
		if err := a.connect(ctx); err != nil {
			if ctx.Err() != nil {
				return // context cancelled — clean shutdown
			}
			log.Printf("[agent] disconnected: %v — reconnecting in %s", err, delay)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay *= 2
		if delay > reconnectMax {
			delay = reconnectMax
		}
	}
}

// connect performs one full connection lifecycle:
// login → dial WS → auth → initial delta report → message loop.
// Returns when the connection is lost or ctx is cancelled.
func (a *AgentClient) connect(ctx context.Context) error {
	jwt, err := a.login(ctx)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	wsURL := toWSURL(a.saasURL) + "/ws/agent"
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	conn, _, err := websocket.DefaultDialer.DialContext(dialCtx, wsURL, nil)
	cancel()
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer conn.Close()

	log.Printf("[agent] connected to %s", wsURL)

	// Step 1: send auth message.
	if err := a.writeJSON(conn, types.Envelope{
		Type:    types.MsgAuth,
		Payload: types.AuthMsg{JWT: jwt},
	}); err != nil {
		return fmt.Errorf("send auth: %w", err)
	}

	// Step 2: wait for auth_result.
	if err := a.awaitAuthResult(conn); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	log.Printf("[agent] authenticated")

	// Step 3: send initial balance snapshot.
	if err := a.sendBalanceSnapshot(conn, ""); err != nil {
		log.Printf("[agent] warning: initial balance snapshot failed: %v", err)
	}

	// Step 4: enter message loop.
	return a.messageLoop(ctx, conn)
}

// awaitAuthResult reads the first message and expects auth_result with ok=true.
func (a *AgentClient) awaitAuthResult(conn *websocket.Conn) error {
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	var env struct {
		Type    string               `json:"type"`
		Payload types.AuthResultMsg  `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("parse auth_result: %w", err)
	}
	if env.Type != types.MsgAuthResult {
		return fmt.Errorf("expected %s, got %s", types.MsgAuthResult, env.Type)
	}
	if !env.Payload.OK {
		return fmt.Errorf("auth rejected: %s", env.Payload.Error)
	}
	return nil
}

// messageLoop drives the heartbeat ticker and dispatches incoming messages.
func (a *AgentClient) messageLoop(ctx context.Context, conn *websocket.Conn) error {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Run the read pump in a goroutine so heartbeats can fire concurrently.
	readErr := make(chan error, 1)
	msgs := make(chan []byte, 32)

	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				readErr <- err
				return
			}
			msgs <- raw
		}
	}()

	for {
		select {
		case <-ctx.Done():
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "shutdown"))
			return nil

		case err := <-readErr:
			return err

		case <-ticker.C:
			if err := a.writeJSON(conn, types.Envelope{Type: types.MsgHeartbeat}); err != nil {
				return fmt.Errorf("send heartbeat: %w", err)
			}

		case raw := <-msgs:
			if err := a.dispatch(conn, raw); err != nil {
				log.Printf("[agent] dispatch error: %v", err)
			}
		}
	}
}

// dispatch handles one inbound WebSocket message.
func (a *AgentClient) dispatch(conn *websocket.Conn, raw []byte) error {
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}

	switch env.Type {
	case types.MsgCommand:
		var cmd types.TradeCommand
		if err := json.Unmarshal(env.Payload, &cmd); err != nil {
			return fmt.Errorf("parse TradeCommand: %w", err)
		}
		// Acknowledge immediately, then execute asynchronously.
		if err := a.writeJSON(conn, types.Envelope{
			Type:    types.MsgCommandAck,
			Payload: types.CommandAckMsg{ClientOrderID: cmd.ClientOrderID},
		}); err != nil {
			return fmt.Errorf("send command_ack: %w", err)
		}
		go a.executeAndReport(conn, cmd)

	case types.MsgHeartbeatAck, types.MsgReportAck:
		// No action required.

	default:
		log.Printf("[agent] unknown message type: %s", env.Type)
	}
	return nil
}

// executeAndReport runs the broker order and sends a delta_report on completion.
func (a *AgentClient) executeAndReport(conn *websocket.Conn, cmd types.TradeCommand) {
	exec, err := a.broker.PlaceOrder(cmd)
	if err != nil {
		log.Printf("[agent] PlaceOrder(%s) failed: %v", cmd.ClientOrderID, err)
		exec = types.Execution{
			ClientOrderID: cmd.ClientOrderID,
			Symbol:        cmd.Symbol,
			Action:        cmd.Action,
			Status:        "failed",
		}
	}

	if sendErr := a.sendBalanceSnapshot(conn, cmd.ClientOrderID); sendErr != nil {
		// Best-effort: try to send at least the execution status.
		log.Printf("[agent] balance fetch failed after order: %v", sendErr)
		_ = a.writeJSON(conn, types.Envelope{
			Type: types.MsgDeltaReport,
			Payload: types.DeltaReport{
				ClientOrderID: cmd.ClientOrderID,
				Execution:     &exec,
			},
		})
	}
}

// sendBalanceSnapshot fetches current broker balances and sends a delta_report.
// clientOrderID may be empty for initial reconnect snapshots.
func (a *AgentClient) sendBalanceSnapshot(conn *websocket.Conn, clientOrderID string) error {
	balances, err := a.broker.GetBalances()
	if err != nil {
		return err
	}
	report := types.DeltaReport{
		ClientOrderID: clientOrderID,
		Balances:      balances,
	}
	return a.writeJSON(conn, types.Envelope{
		Type:    types.MsgDeltaReport,
		Payload: report,
	})
}

// writeJSON serialises v to JSON and sends it as a text WebSocket frame.
// writeMu ensures only one goroutine writes at a time.
func (a *AgentClient) writeJSON(conn *websocket.Conn, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// -- HTTP helpers --

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// login calls the SaaS REST login endpoint and returns a JWT.
func (a *AgentClient) login(ctx context.Context) (string, error) {
	body, _ := json.Marshal(loginRequest{Email: a.email, Password: a.password})
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		a.saasURL+"/api/v1/auth/login",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login returned HTTP %d", resp.StatusCode)
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if lr.Token == "" {
		return "", fmt.Errorf("empty token in login response")
	}
	return lr.Token, nil
}

// toWSURL converts an http(s) base URL to the corresponding ws(s) URL.
func toWSURL(u string) string {
	if len(u) >= 5 && u[:5] == "https" {
		return "wss" + u[5:]
	}
	if len(u) >= 4 && u[:4] == "http" {
		return "ws" + u[4:]
	}
	return u
}
