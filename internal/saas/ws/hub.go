package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/quantsaas/platform/internal/saas/auth"
	"github.com/quantsaas/platform/internal/saas/store"
)

const (
	// authTimeout is the window during which the Agent must send the first
	// auth frame; closes the connection otherwise.
	authTimeout = 10 * time.Second

	// writeTimeout bounds every outbound frame.
	writeTimeout = 5 * time.Second

	// readDeadline guards the message loop against half-open connections.
	// Agents heartbeat every 30s, so 70s gives us two missed beats of slack.
	readDeadline = 70 * time.Second
)

// ErrAgentNotConnected is returned by SendToAgent when no live socket exists
// for the target user. Callers (e.g. the cron tick) should log + skip.
var ErrAgentNotConnected = errors.New("agent not connected")

// upgrader is shared because it has no per-connection state.
// CheckOrigin is permissive — the auth handshake on the first frame is the
// real gate, not the HTTP origin header.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

// AgentConn represents one live Agent WebSocket session.
//
// writeMu serialises concurrent writes — gorilla/websocket forbids parallel
// writes on the same connection. The Hub's SendToAgent and the read loop's
// ack sends both go through writeJSON below.
type AgentConn struct {
	userID  uint
	conn    *websocket.Conn
	writeMu sync.Mutex
}

// writeJSON serialises v as a text frame, holding writeMu for the duration.
func (c *AgentConn) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Hub is the central connection manager. One Hub per SaaS process.
//
// At most one AgentConn per userID is retained — a new connection for the
// same user displaces (and closes) the prior one. This matches the topology
// doc's assumption that a user runs exactly one Agent process at a time.
type Hub struct {
	auth *auth.Service
	db   *store.DB

	// conns maps userID (uint) → *AgentConn. sync.Map is sized for the
	// read-heavy SendToAgent path; writes happen only at connect/disconnect.
	conns sync.Map
}

// NewHub constructs a Hub. It does not start any goroutines; connections
// are driven entirely by HandleConnection invocations from the HTTP router.
func NewHub(authSvc *auth.Service, db *store.DB) *Hub {
	return &Hub{auth: authSvc, db: db}
}

// IsConnected reports whether an Agent socket is currently registered for userID.
// Used by the REST status endpoint.
func (h *Hub) IsConnected(userID uint) bool {
	_, ok := h.conns.Load(userID)
	return ok
}

// ListConnectedUsers returns the userIDs of every Agent currently connected.
// Snapshot only; the set can change immediately after return.
func (h *Hub) ListConnectedUsers() []uint {
	ids := make([]uint, 0)
	h.conns.Range(func(k, _ any) bool {
		if id, ok := k.(uint); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

// SendToAgent pushes a TradeCommand to the Agent owned by userID.
// Returns ErrAgentNotConnected if no live socket exists; the caller is
// expected to skip the tick rather than retry — the next cron tick will
// naturally re-evaluate using the latest PortfolioState.
func (h *Hub) SendToAgent(userID uint, cmd TradeCommand) error {
	v, ok := h.conns.Load(userID)
	if !ok {
		return ErrAgentNotConnected
	}
	ac := v.(*AgentConn)
	return ac.writeJSON(outEnvelope{Type: MsgCommand, Payload: cmd})
}

// HandleConnection is the Gin handler bound to GET /ws/agent.
//
// Lifecycle:
//  1. Upgrade HTTP → WebSocket.
//  2. Read the first frame within authTimeout; reject unless it is an
//     `auth` envelope with a valid JWT.
//  3. Register the (userID → *AgentConn) mapping, displacing any prior conn.
//  4. Run the message loop until the socket closes or read fails.
//  5. Unregister on exit.
//
// We deliberately do NOT spawn a writer pump goroutine — writes are
// performed inline from the read goroutine (acks) and from external
// callers (SendToAgent), all serialised by AgentConn.writeMu. This keeps
// the connection lifecycle on a single goroutine per socket.
func (h *Hub) HandleConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade itself already wrote an HTTP error response on failure.
		log.Printf("[ws] upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	userID, err := h.authenticate(conn)
	if err != nil {
		log.Printf("[ws] auth failed: %v", err)
		_ = writeAuthResult(conn, false, err.Error())
		return
	}
	if err := writeAuthResult(conn, true, ""); err != nil {
		log.Printf("[ws] write auth_result: %v", err)
		return
	}

	ac := &AgentConn{userID: userID, conn: conn}
	h.register(ac)
	defer h.unregister(userID, ac)

	log.Printf("[ws] agent connected: userID=%d", userID)
	h.messageLoop(ac)
	log.Printf("[ws] agent disconnected: userID=%d", userID)
}

// authenticate reads the first frame under a strict deadline and validates
// the JWT. Returns the authenticated userID.
func (h *Hub) authenticate(conn *websocket.Conn) (uint, error) {
	_ = conn.SetReadDeadline(time.Now().Add(authTimeout))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return 0, fmt.Errorf("read first frame: %w", err)
	}

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, fmt.Errorf("parse envelope: %w", err)
	}
	if env.Type != MsgAuth {
		return 0, fmt.Errorf("expected %s, got %s", MsgAuth, env.Type)
	}

	var msg AuthMsg
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		return 0, fmt.Errorf("parse auth payload: %w", err)
	}
	if msg.JWT == "" {
		return 0, errors.New("missing jwt")
	}

	claims, err := h.auth.ParseToken(msg.JWT)
	if err != nil {
		return 0, fmt.Errorf("verify jwt: %w", err)
	}
	if claims.UserID == 0 {
		return 0, errors.New("jwt missing user_id")
	}
	return claims.UserID, nil
}

// register inserts ac into the connection map, closing any prior conn for
// the same user. The displaced read loop will observe a read error and
// unwind on its own.
func (h *Hub) register(ac *AgentConn) {
	if prev, loaded := h.conns.Swap(ac.userID, ac); loaded {
		old := prev.(*AgentConn)
		log.Printf("[ws] displacing prior connection for userID=%d", ac.userID)
		_ = old.conn.Close()
	}
}

// unregister removes ac from the map iff the current entry is still ac.
// This guards against a race where a newer connection has already taken
// the slot before this loop's defer fires.
func (h *Hub) unregister(userID uint, ac *AgentConn) {
	h.conns.CompareAndDelete(userID, ac)
}

// messageLoop is the single read pump for one connection. It dispatches
// heartbeat and delta_report frames; unknown frames are logged and ignored
// (forward compatibility with future Agent versions).
func (h *Hub) messageLoop(ac *AgentConn) {
	for {
		_ = ac.conn.SetReadDeadline(time.Now().Add(readDeadline))
		_, raw, err := ac.conn.ReadMessage()
		if err != nil {
			return
		}

		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			log.Printf("[ws] userID=%d: malformed envelope: %v", ac.userID, err)
			continue
		}

		switch env.Type {
		case MsgHeartbeat:
			if err := ac.writeJSON(outEnvelope{Type: MsgHeartbeatAck}); err != nil {
				log.Printf("[ws] userID=%d: heartbeat_ack write failed: %v", ac.userID, err)
				return
			}

		case MsgDeltaReport:
			var report DeltaReport
			if err := json.Unmarshal(env.Payload, &report); err != nil {
				log.Printf("[ws] userID=%d: malformed delta_report: %v", ac.userID, err)
				_ = ac.writeJSON(outEnvelope{
					Type:    MsgReportAck,
					Payload: ReportAckMsg{OK: false, Error: "malformed payload"},
				})
				continue
			}
			ackPayload := h.processDeltaReport(ac.userID, report)
			if err := ac.writeJSON(outEnvelope{Type: MsgReportAck, Payload: ackPayload}); err != nil {
				log.Printf("[ws] userID=%d: report_ack write failed: %v", ac.userID, err)
				return
			}

		case MsgCommandAck:
			// Receipt-only ack from the Agent. The spec defines this purely
			// as a flow-control signal; we do not persist it.

		default:
			log.Printf("[ws] userID=%d: unknown message type %q", ac.userID, env.Type)
		}
	}
}

// writeAuthResult is a one-shot writer used before the AgentConn exists.
func writeAuthResult(conn *websocket.Conn, ok bool, errMsg string) error {
	data, err := json.Marshal(outEnvelope{
		Type:    MsgAuthResult,
		Payload: AuthResultMsg{OK: ok, Error: errMsg},
	})
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}
