package ws

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/quantsaas/platform/internal/saas/auth"
	"github.com/quantsaas/platform/internal/saas/config"
	"github.com/quantsaas/platform/internal/saas/store"
)

// newTestDB returns an in-memory SQLite-backed *store.DB with the schema needed
// by the Hub's DeltaReport path. The connection pool is pinned to a single
// connection so the in-memory database survives across queries/transactions.
func newTestDB(t *testing.T) *store.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := gdb.AutoMigrate(
		&store.User{},
		&store.StrategyTemplate{},
		&store.StrategyInstance{},
		&store.PortfolioState{},
		&store.SpotExecution{},
		&store.TradeRecord{},
		&store.AuditLog{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return &store.DB{DB: gdb}
}

func testAuthService() *auth.Service {
	return auth.NewService(&config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", ExpiryHours: 1},
	})
}

func testServer(t *testing.T, hub *Hub) (*httptest.Server, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws/agent", hub.HandleConnection)
	srv := httptest.NewServer(r)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/agent"
	return srv, wsURL
}

func mustRaw(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// readEnvelope reads one frame and decodes the outer envelope.
func readEnvelope(t *testing.T, conn *websocket.Conn) Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	return env
}

// TestUnauthenticatedConnectionTimesOut verifies that a connection which never
// sends an auth frame is dropped after authTimeout. authTimeout is shortened
// here so the test runs quickly while still exercising the real timeout path.
func TestUnauthenticatedConnectionTimesOut(t *testing.T) {
	orig := authTimeout
	authTimeout = 200 * time.Millisecond
	defer func() { authTimeout = orig }()

	hub := NewHub(testAuthService(), newTestDB(t))
	srv, wsURL := testServer(t, hub)
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	start := time.Now()
	// Never send auth. Drain frames (the server may emit an auth_result before
	// closing) until the read fails — that error marks the disconnect.
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	elapsed := time.Since(start)

	if elapsed < authTimeout {
		t.Fatalf("connection closed before authTimeout: %v < %v", elapsed, authTimeout)
	}
	if elapsed > authTimeout+2*time.Second {
		t.Fatalf("connection not closed promptly after authTimeout: %v", elapsed)
	}
}

// TestDeltaReportUpdatesPortfolioState verifies that a valid reconnect-snapshot
// DeltaReport, delivered over an authenticated socket, is applied to the
// corresponding instance's PortfolioState.
func TestDeltaReportUpdatesPortfolioState(t *testing.T) {
	db := newTestDB(t)
	authSvc := testAuthService()
	hub := NewHub(authSvc, db)
	srv, wsURL := testServer(t, hub)
	defer srv.Close()

	// Seed a user, a template, and a RUNNING instance for that user.
	user := store.User{Email: "agent@example.com", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	tmpl := store.StrategyTemplate{Name: "dca_balance", Version: "1"}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	inst := store.StrategyInstance{
		UserID:     user.ID,
		TemplateID: tmpl.ID,
		Status:     store.InstanceStatusRunning,
	}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	token, err := authSvc.SignToken(user.ID, "user")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// 1. Authenticate.
	if err := conn.WriteJSON(Envelope{Type: MsgAuth, Payload: mustRaw(t, AuthMsg{JWT: token})}); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	authResp := readEnvelope(t, conn)
	if authResp.Type != MsgAuthResult {
		t.Fatalf("expected %s, got %s", MsgAuthResult, authResp.Type)
	}
	var ar AuthResultMsg
	if err := json.Unmarshal(authResp.Payload, &ar); err != nil {
		t.Fatalf("unmarshal auth_result: %v", err)
	}
	if !ar.OK {
		t.Fatalf("authentication rejected: %s", ar.Error)
	}

	// 2. Send a reconnect-snapshot DeltaReport (no ClientOrderID / Execution).
	report := DeltaReport{
		Balances: []Balance{
			{Asset: "CNY", Available: 1000, Frozen: 0},
			{Asset: "510300", Available: 5, Frozen: 0},
		},
	}
	if err := conn.WriteJSON(Envelope{Type: MsgDeltaReport, Payload: mustRaw(t, report)}); err != nil {
		t.Fatalf("write delta_report: %v", err)
	}

	// 3. Expect a successful report_ack.
	ackEnv := readEnvelope(t, conn)
	if ackEnv.Type != MsgReportAck {
		t.Fatalf("expected %s, got %s", MsgReportAck, ackEnv.Type)
	}
	var ack ReportAckMsg
	if err := json.Unmarshal(ackEnv.Payload, &ack); err != nil {
		t.Fatalf("unmarshal report_ack: %v", err)
	}
	if !ack.OK {
		t.Fatalf("report_ack not OK: %s", ack.Error)
	}

	// 4. Verify the PortfolioState now reflects the snapshot.
	var ps store.PortfolioState
	if err := db.Where("instance_id = ?", inst.ID).First(&ps).Error; err != nil {
		t.Fatalf("load PortfolioState: %v", err)
	}
	if ps.CNYBalance != 1000 {
		t.Errorf("CNYBalance = %v, want 1000", ps.CNYBalance)
	}
	if ps.DeadHold != 0 {
		t.Errorf("DeadHold = %v, want 0", ps.DeadHold)
	}
	if ps.FloatHold != 5 {
		t.Errorf("FloatHold = %v, want 5 (reconnect routes all asset to float)", ps.FloatHold)
	}
	if ps.TotalEquity != 1005 {
		t.Errorf("TotalEquity = %v, want 1005", ps.TotalEquity)
	}
}
