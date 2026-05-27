package instance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/quantsaas/platform/internal/quant"
	"github.com/quantsaas/platform/internal/saas/store"
	"gorm.io/gorm"
)

// Hub sends trade commands to connected agents via WebSocket.
// SendToAgent returns false if the agent for this instance is not currently connected.
type Hub interface {
	SendToAgent(ctx context.Context, instanceID uint, cmd TradeCommand) bool
}

// MarketFetcher fetches OHLCV bars from a public market data source.
type MarketFetcher interface {
	FetchBars(ctx context.Context, symbol, interval string, limit int) ([]quant.Bar, error)
}

// Strategy is the pure-function strategy contract.
// Step() must have zero I/O, no global state, no branching on live-vs-backtest.
type Strategy interface {
	Step(input quant.StrategyInput) quant.StrategyOutput
}

// TradeCommand is the wire format for instructions pushed from SaaS to Agent.
// Mirrors the protocol spec in docs/QuanSaas系统总体拓扑结构.md §5.3.
type TradeCommand struct {
	ClientOrderID string `json:"client_order_id"` // inst{id}-{engine}-{ts}
	Action        string `json:"action"`          // BUY | SELL
	Engine        string `json:"engine"`          // MACRO | MICRO
	Symbol        string `json:"symbol"`
	AmountCNY     string `json:"amount_cny"` // buy: CNY amount (non-empty when Action=BUY)
	QtyAsset      string `json:"qty_asset"`  // sell: asset qty (non-empty when Action=SELL)
	LotType       string `json:"lot_type"`   // DEAD_STACK | FLOATING
}

// TemplateManifest is the typed payload stored in StrategyTemplate.Manifest.
type TemplateManifest struct {
	Symbol     string           `json:"symbol"`
	Interval   string           `json:"interval"` // bar aggregation period, e.g. "1d", "60m"
	SpawnPoint quant.SpawnPoint `json:"spawn_point"`
}

const (
	redisKeyChampion = "champion_gene:%d" // placeholder arg: templateID
	barsLimit        = 600                // history window fed to Step()
)

// Manager owns instance lifecycle transitions and drives Tick execution.
type Manager struct {
	db         *store.DB
	redis      *store.RedisClient
	hub        Hub
	fetcher    MarketFetcher
	strategies map[string]Strategy
}

// New constructs a Manager. strategies maps template Name → Strategy impl.
func New(
	db *store.DB,
	redis *store.RedisClient,
	hub Hub,
	fetcher MarketFetcher,
	strategies map[string]Strategy,
) *Manager {
	if strategies == nil {
		strategies = map[string]Strategy{}
	}
	return &Manager{db: db, redis: redis, hub: hub, fetcher: fetcher, strategies: strategies}
}

// ─── Lifecycle ────────────────────────────────────────────────────────────────

// Start transitions an instance from STOPPED → RUNNING.
func (m *Manager) Start(ctx context.Context, instanceID uint) error {
	res := m.db.WithContext(ctx).
		Model(&store.StrategyInstance{}).
		Where("id = ? AND status = ?", instanceID, store.InstanceStatusStopped).
		Update("status", store.InstanceStatusRunning)
	if res.Error != nil {
		return fmt.Errorf("start instance %d: %w", instanceID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("instance %d: not found or not in STOPPED state", instanceID)
	}
	return nil
}

// Stop transitions an instance from RUNNING → STOPPED.
func (m *Manager) Stop(ctx context.Context, instanceID uint) error {
	res := m.db.WithContext(ctx).
		Model(&store.StrategyInstance{}).
		Where("id = ? AND status = ?", instanceID, store.InstanceStatusRunning).
		Update("status", store.InstanceStatusStopped)
	if res.Error != nil {
		return fmt.Errorf("stop instance %d: %w", instanceID, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("instance %d: not found or not in RUNNING state", instanceID)
	}
	return nil
}

// Delete soft-deletes an instance regardless of its current status.
func (m *Manager) Delete(ctx context.Context, instanceID uint) error {
	if err := m.db.WithContext(ctx).Delete(&store.StrategyInstance{}, instanceID).Error; err != nil {
		return fmt.Errorf("delete instance %d: %w", instanceID, err)
	}
	return nil
}

// ─── Tick ─────────────────────────────────────────────────────────────────────

// Tick advances one strategy instance by one completed bar. Called by the cron
// scheduler for every RUNNING instance. Panics are recovered; any error
// transitions the instance to ERROR state.
func (m *Manager) Tick(ctx context.Context, inst store.StrategyInstance) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[tick] instance %d panic: %v", inst.ID, r)
			m.markError(ctx, inst.ID)
		}
	}()
	if err := m.tick(ctx, inst); err != nil {
		log.Printf("[tick] instance %d: %v", inst.ID, err)
		m.markError(ctx, inst.ID)
	}
}

func (m *Manager) markError(ctx context.Context, instanceID uint) {
	m.db.WithContext(ctx).
		Model(&store.StrategyInstance{}).
		Where("id = ?", instanceID).
		Update("status", store.InstanceStatusError)
}

func (m *Manager) tick(ctx context.Context, inst store.StrategyInstance) error {
	var mf TemplateManifest
	if err := json.Unmarshal(inst.Template.Manifest, &mf); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}

	// ── Step 1: Idempotent bar dedup check ───────────────────────────────────
	bars, err := m.fetcher.FetchBars(ctx, mf.Symbol, mf.Interval, barsLimit)
	if err != nil {
		return fmt.Errorf("fetch bars %s/%s: %w", mf.Symbol, mf.Interval, err)
	}
	if len(bars) == 0 {
		return fmt.Errorf("no bars returned for %s/%s", mf.Symbol, mf.Interval)
	}
	latestBarTime := time.Unix(bars[len(bars)-1].OpenTime, 0).UTC()

	var portfolio store.PortfolioState
	if err := m.db.WithContext(ctx).
		Where("instance_id = ?", inst.ID).
		First(&portfolio).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load portfolio state: %w", err)
	}
	if portfolio.LastProcessedBarTime != nil &&
		!latestBarTime.After(*portfolio.LastProcessedBarTime) {
		// Same aggregation bucket already processed — skip this tick.
		return nil
	}

	// ── Step 2: Load PortfolioState (above) and RuntimeState ─────────────────
	var rtRow store.RuntimeState
	if err := m.db.WithContext(ctx).
		Where("instance_id = ?", inst.ID).
		First(&rtRow).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("load runtime state: %w", err)
	}
	var rt quant.RuntimeState
	if len(rtRow.Payload) > 0 {
		if err := json.Unmarshal(rtRow.Payload, &rt); err != nil {
			return fmt.Errorf("decode runtime state: %w", err)
		}
	}

	// ── Step 3: Load champion params from Redis → DB → default seed ──────────
	champion, err := m.loadChampion(ctx, inst.TemplateID)
	if err != nil {
		return fmt.Errorf("load champion params: %w", err)
	}

	// ── Step 4: ACL outer ring — extract closes + timestamps (IsSpot iron rule)
	closes := quant.ExtractCloses(bars)
	timestamps := quant.ExtractTimestamps(bars)

	// ── Step 5: Build StrategyInput ───────────────────────────────────────────
	var storeLots []store.SpotLot
	if err := m.db.WithContext(ctx).
		Where("instance_id = ?", inst.ID).
		Find(&storeLots).Error; err != nil {
		return fmt.Errorf("load lots: %w", err)
	}
	quantLots := make([]quant.SpotLot, len(storeLots))
	for i, sl := range storeLots {
		quantLots[i] = toQuantLot(sl)
	}

	currentPrice := closes[len(closes)-1]
	input := quant.StrategyInput{
		Symbol:       mf.Symbol,
		Closes:       closes,
		Timestamps:   timestamps,
		CurrentTime:  timestamps[len(timestamps)-1],
		CurrentPrice: currentPrice,
		Portfolio: quant.PortfolioSnapshot{
			CNYBalance:     portfolio.CNYBalance,
			DeadHold:       portfolio.DeadHold,
			FloatHold:      portfolio.FloatHold,
			ColdSealedHold: portfolio.ColdSealedHold,
			Lots:           quantLots,
		},
		Params:  champion,
		Spawn:   mf.SpawnPoint,
		Runtime: rt,
	}

	// ── Step 6: Call Step() — the only live call site, identical to backtest ──
	strategy, ok := m.strategies[inst.Template.Name]
	if !ok {
		return fmt.Errorf("strategy %q not registered", inst.Template.Name)
	}
	output := strategy.Step(input)

	// ── Step 7: Persist updated RuntimeState ─────────────────────────────────
	updatedRT := quant.RuntimeState{
		TicksSinceLastMacro: rt.TicksSinceLastMacro + 1,
		LastMacroTimestamp:  rt.LastMacroTimestamp,
	}
	if len(output.MacroOrders) > 0 {
		updatedRT.TicksSinceLastMacro = 0
		updatedRT.LastMacroTimestamp = timestamps[len(timestamps)-1]
	}
	if err := m.persistRuntimeState(ctx, inst.ID, updatedRT, &rtRow); err != nil {
		return fmt.Errorf("persist runtime state: %w", err)
	}

	// ── Step 8: Handle release events — SaaS ledger only, no Agent command ───
	if err := m.applyReleaseEvents(ctx, inst.ID, output.ReleaseEvents); err != nil {
		return fmt.Errorf("apply release events: %w", err)
	}

	// ── Step 9: Translate orders → TradeCommand → SpotExecution → Agent ──────
	barTS := timestamps[len(timestamps)-1]
	if err := m.dispatchOrders(ctx, inst.ID, mf.Symbol, barTS, output); err != nil {
		return fmt.Errorf("dispatch orders: %w", err)
	}

	// ── Step 10: Update LastProcessedBarTime ──────────────────────────────────
	if err := m.advanceBarCursor(ctx, inst.ID, latestBarTime); err != nil {
		return fmt.Errorf("advance bar cursor: %w", err)
	}

	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (m *Manager) loadChampion(ctx context.Context, templateID uint) (quant.Chromosome, error) {
	key := fmt.Sprintf(redisKeyChampion, templateID)
	if raw, err := m.redis.Get(ctx, key); err == nil {
		var c quant.Chromosome
		if jsonErr := json.Unmarshal([]byte(raw), &c); jsonErr == nil {
			return c, nil
		}
	}

	var gene store.GeneRecord
	if err := m.db.WithContext(ctx).
		Where("strategy_id = ? AND role = ?", templateID, store.GeneRoleChampion).
		Order("created_at DESC").
		First(&gene).Error; err == nil {
		var c quant.Chromosome
		if jsonErr := json.Unmarshal(gene.ParamPack, &c); jsonErr == nil {
			return c, nil
		}
	}

	// Fall back to the default seed so the instance never stalls.
	log.Printf("[champion] no champion gene for template %d, using default seed", templateID)
	return quant.DefaultSeedChromosome, nil
}

func (m *Manager) persistRuntimeState(
	ctx context.Context,
	instanceID uint,
	rt quant.RuntimeState,
	existing *store.RuntimeState,
) error {
	payload, err := json.Marshal(rt)
	if err != nil {
		return err
	}
	if existing.ID != 0 {
		return m.db.WithContext(ctx).
			Model(existing).
			Update("payload", json.RawMessage(payload)).Error
	}
	row := store.RuntimeState{InstanceID: instanceID, Payload: payload}
	return m.db.WithContext(ctx).Create(&row).Error
}

// applyReleaseEvents updates the SaaS-side lot ledger for every
// DeadHold → FloatHold semantic transition. No TradeCommand is emitted.
// An audit log entry is written for every event (iron rule §9).
func (m *Manager) applyReleaseEvents(ctx context.Context, instanceID uint, events []quant.ReleaseEvent) error {
	for _, ev := range events {
		if err := m.applyOneRelease(ctx, instanceID, ev); err != nil {
			return err
		}
		payload, _ := json.Marshal(ev)
		entry := store.AuditLog{
			InstanceID: instanceID,
			EventType:  "LOT_RELEASE",
			Payload:    payload,
		}
		if err := m.db.WithContext(ctx).Create(&entry).Error; err != nil {
			return fmt.Errorf("audit release %s: %w", ev.LotID, err)
		}
	}
	return nil
}

func (m *Manager) applyOneRelease(ctx context.Context, instanceID uint, ev quant.ReleaseEvent) error {
	lotIDStr := ev.LotID
	isSplit := strings.HasSuffix(lotIDStr, "_sr") || strings.HasSuffix(lotIDStr, "_hr")

	if !isSplit {
		// Full transition: update lot_type in place.
		lotID, err := strconv.ParseUint(lotIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parse lot id %q: %w", lotIDStr, err)
		}
		return m.db.WithContext(ctx).
			Model(&store.SpotLot{}).
			Where("id = ? AND instance_id = ?", lotID, instanceID).
			Update("lot_type", string(ev.ToState)).Error
	}

	// Partial split: find original lot, reduce its amount, create new floating lot.
	suffix := "_sr"
	if strings.HasSuffix(lotIDStr, "_hr") {
		suffix = "_hr"
	}
	baseIDStr := strings.TrimSuffix(lotIDStr, suffix)
	baseID, err := strconv.ParseUint(baseIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parse base lot id %q: %w", baseIDStr, err)
	}

	var orig store.SpotLot
	if err := m.db.WithContext(ctx).
		Where("id = ? AND instance_id = ?", baseID, instanceID).
		First(&orig).Error; err != nil {
		return fmt.Errorf("find lot %d: %w", baseID, err)
	}

	newAmount := orig.Amount - ev.QtyAsset
	if newAmount < 0 {
		newAmount = 0
	}
	if err := m.db.WithContext(ctx).
		Model(&orig).
		Update("amount", newAmount).Error; err != nil {
		return fmt.Errorf("reduce lot %d: %w", baseID, err)
	}

	newLot := store.SpotLot{
		InstanceID:   instanceID,
		LotType:      string(ev.ToState),
		Amount:       ev.QtyAsset,
		CostPrice:    orig.CostPrice,
		IsColdSealed: false,
	}
	return m.db.WithContext(ctx).Create(&newLot).Error
}

// dispatchOrders translates StrategyOutput orders into TradeCommands,
// writes a pending SpotExecution record for each, and pushes to the Agent.
// If the Agent is not connected, the warning is logged and the send is
// skipped; the pending SpotExecution record remains for the Hub to deliver
// on reconnect.
func (m *Manager) dispatchOrders(
	ctx context.Context,
	instanceID uint,
	symbol string,
	barTS int64,
	output quant.StrategyOutput,
) error {
	type taggedOrder struct {
		order  quant.Order
		engine string
	}
	var all []taggedOrder
	for _, o := range output.MacroOrders {
		all = append(all, taggedOrder{o, "MACRO"})
	}
	for _, o := range output.MicroOrders {
		all = append(all, taggedOrder{o, "MICRO"})
	}

	for i, to := range all {
		clientOrderID := fmt.Sprintf("inst%d-%s-%d-%d", instanceID, to.engine, barTS, i)

		cmd := TradeCommand{
			ClientOrderID: clientOrderID,
			Action:        string(to.order.Action),
			Engine:        to.engine,
			Symbol:        symbol,
			LotType:       string(to.order.LotType),
		}
		if to.order.Action == quant.OrderBuy {
			cmd.AmountCNY = strconv.FormatFloat(to.order.AmountCNY, 'f', 2, 64)
		} else {
			cmd.QtyAsset = strconv.FormatFloat(to.order.QtyAsset, 'f', 8, 64)
		}

		exec := store.SpotExecution{
			InstanceID:    instanceID,
			ClientOrderID: clientOrderID,
			Status:        "pending",
			Symbol:        symbol,
			Action:        string(to.order.Action),
			LotType:       string(to.order.LotType),
		}
		if err := m.db.WithContext(ctx).Create(&exec).Error; err != nil {
			return fmt.Errorf("create spot execution %s: %w", clientOrderID, err)
		}

		if sent := m.hub.SendToAgent(ctx, instanceID, cmd); !sent {
			log.Printf("[tick] instance %d: agent not connected, %s queued for reconnect", instanceID, clientOrderID)
		}
	}
	return nil
}

// advanceBarCursor sets LastProcessedBarTime on PortfolioState, creating
// the record if it does not yet exist.
func (m *Manager) advanceBarCursor(ctx context.Context, instanceID uint, t time.Time) error {
	res := m.db.WithContext(ctx).
		Model(&store.PortfolioState{}).
		Where("instance_id = ?", instanceID).
		Update("last_processed_bar_time", t)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		row := store.PortfolioState{InstanceID: instanceID, LastProcessedBarTime: &t}
		return m.db.WithContext(ctx).Create(&row).Error
	}
	return nil
}

// toQuantLot converts a store lot record into the quant engine's SpotLot type.
func toQuantLot(s store.SpotLot) quant.SpotLot {
	return quant.SpotLot{
		LotID:        fmt.Sprintf("%d", s.ID),
		LotType:      quant.LotType(s.LotType),
		Amount:       s.Amount,
		CostPrice:    s.CostPrice,
		CreatedAt:    s.CreatedAt.Unix(),
		IsColdSealed: s.IsColdSealed,
	}
}
