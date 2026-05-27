package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/quantsaas/platform/internal/saas/store"
	"gorm.io/gorm"
)

// cnyAsset is the canonical key used in DeltaReport.Balances for cash.
// The Agent normalises to this string regardless of broker conventions.
const cnyAsset = "CNY"

// processDeltaReport applies one Agent → SaaS state update.
//
// Two distinct shapes share this entry point:
//
//  1. Trade fill: ClientOrderID is non-empty and Execution is present.
//     We locate the pending SpotExecution, mark it filled, splice the fill
//     into DeadHold/FloatHold per the original LotType, append a
//     TradeRecord, and then refresh balances from the snapshot.
//
//  2. Reconnect snapshot: ClientOrderID is empty (and Execution is nil).
//     Only the balance snapshot is applied; no lot bookkeeping occurs.
//     This is how the system self-heals after Agent restarts.
//
// All mutations happen inside a single transaction so a mid-flight failure
// cannot leave PortfolioState diverged from SpotExecution.
//
// The returned ReportAckMsg is sent back to the Agent verbatim.
func (h *Hub) processDeltaReport(userID uint, report DeltaReport) ReportAckMsg {
	ack := ReportAckMsg{ClientOrderID: report.ClientOrderID, OK: true}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var (
			instanceID uint
			lotType    string
		)

		// Shape 1: this report corresponds to a previously dispatched command.
		if report.ClientOrderID != "" {
			exec, err := loadPendingExecution(tx, report.ClientOrderID)
			if err != nil {
				return err
			}
			instanceID = exec.InstanceID
			lotType = exec.LotType

			if err := applyExecutionFill(tx, exec, report.Execution); err != nil {
				return err
			}
		}

		// Shape 2 (and also a tail-step of shape 1): refresh the broker
		// balance snapshot onto PortfolioState. Reconnect-only snapshots
		// arrive without a known instanceID — we resolve it by looking up
		// any RUNNING instance owned by this user.
		if instanceID == 0 {
			id, err := resolveInstanceForUser(tx, userID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// No RUNNING instance for this user — accept the
					// snapshot silently and let the next cron tick recover.
					return writeAudit(tx, 0, "delta_report_orphan_snapshot", report)
				}
				return err
			}
			instanceID = id
		}

		if err := applyBalanceSnapshot(tx, instanceID, lotType, report.Execution, report.Balances); err != nil {
			return err
		}
		return writeAudit(tx, instanceID, auditEventFor(report), report)
	})

	if err != nil {
		log.Printf("[ws] userID=%d processDeltaReport: %v", userID, err)
		return ReportAckMsg{ClientOrderID: report.ClientOrderID, OK: false, Error: err.Error()}
	}
	return ack
}

// loadPendingExecution fetches the pending SpotExecution for the order id.
// It returns an error (and aborts the transaction) if the row is missing or
// has already been settled — duplicate reports must not double-credit.
func loadPendingExecution(tx *gorm.DB, clientOrderID string) (*store.SpotExecution, error) {
	var exec store.SpotExecution
	if err := tx.Where("client_order_id = ?", clientOrderID).First(&exec).Error; err != nil {
		return nil, fmt.Errorf("lookup SpotExecution %s: %w", clientOrderID, err)
	}
	if exec.Status != "pending" {
		return nil, fmt.Errorf("SpotExecution %s already in status %q", clientOrderID, exec.Status)
	}
	return &exec, nil
}

// applyExecutionFill marks the SpotExecution settled and appends a permanent
// TradeRecord. The Execution payload is the broker truth — we trust it as-is
// per the "云端只信上报" tenet.
func applyExecutionFill(tx *gorm.DB, exec *store.SpotExecution, ex *Execution) error {
	if ex == nil {
		return errors.New("delta_report missing execution for non-empty client_order_id")
	}

	status := ex.Status
	if status != "filled" && status != "failed" {
		return fmt.Errorf("invalid execution status %q", status)
	}

	exec.Status = status
	exec.FilledQty = ex.FilledQty
	exec.FilledPrice = ex.FilledPrice
	exec.Fee = ex.Fee
	if err := tx.Save(exec).Error; err != nil {
		return fmt.Errorf("update SpotExecution: %w", err)
	}

	// Failed orders leave no permanent trade record but still settle the
	// pending row — Agent never retries unilaterally; the next tick will.
	if status != "filled" {
		return nil
	}

	rec := store.TradeRecord{
		InstanceID:    exec.InstanceID,
		ClientOrderID: exec.ClientOrderID,
		Action:        exec.Action,
		Engine:        engineFromOrderID(exec.ClientOrderID),
		Symbol:        exec.Symbol,
		FilledQty:     ex.FilledQty,
		FilledPrice:   ex.FilledPrice,
		Fee:           ex.Fee,
	}
	if err := tx.Create(&rec).Error; err != nil {
		return fmt.Errorf("create TradeRecord: %w", err)
	}
	return nil
}

// applyBalanceSnapshot overwrites the CNY cash balance and, when a lotType
// is supplied, splices the filled quantity into the corresponding hold
// bucket (DeadHold or FloatHold). When no lotType is supplied (reconnect),
// we recompute the *asset* hold totals as a single block held under their
// existing dead/float split — i.e. we do not redistribute existing splits.
//
// Rationale: the Agent reports the broker's truth for cash and aggregate
// asset shares, but it does not know about SaaS's dead/float bookkeeping.
// On a reconnect, we therefore:
//   - trust cash absolutely (overwrite CNYBalance);
//   - re-normalise the asset side so DeadHold + FloatHold == aggregate share
//     count from the snapshot, preserving the existing ratio.
//
// This is the "天然自愈" path from §5.5: on the next tick, Step() decides
// against the now-correct ledger.
func applyBalanceSnapshot(
	tx *gorm.DB,
	instanceID uint,
	lotType string,
	ex *Execution,
	balances []Balance,
) error {
	var ps store.PortfolioState
	if err := tx.Where("instance_id = ?", instanceID).First(&ps).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load PortfolioState: %w", err)
		}
		ps = store.PortfolioState{InstanceID: instanceID}
	}

	var (
		cny         float64
		assetTotal  float64
		hasCNY      bool
		hasAsset    bool
	)
	for _, b := range balances {
		net := b.Available + b.Frozen
		if b.Asset == cnyAsset {
			cny = net
			hasCNY = true
			continue
		}
		// We sum across any non-CNY asset slot; an instance trades one
		// symbol, so in practice this loop sees exactly one such entry.
		assetTotal += net
		hasAsset = true
	}

	if hasCNY {
		ps.CNYBalance = cny
	}

	switch {
	case lotType != "" && ex != nil && ex.Status == "filled":
		// Trade fill: shift the filled quantity into the correct bucket,
		// signed by action. We do not touch the *other* bucket — the
		// authoritative asset total is reasserted below.
		signed := ex.FilledQty
		if ex.Action == "SELL" {
			signed = -signed
		}
		switch lotType {
		case store.LotTypeDeadStack:
			ps.DeadHold += signed
		case store.LotTypeFloating:
			ps.FloatHold += signed
		}
		// After the splice, force the aggregate to match the broker truth.
		// If the splice + prior state diverge from the snapshot, the
		// surplus/deficit is absorbed by FloatHold (the elastic bucket).
		if hasAsset {
			diff := assetTotal - (ps.DeadHold + ps.FloatHold)
			ps.FloatHold += diff
		}

	case hasAsset:
		// Reconnect snapshot or fill on an unknown lot: preserve the
		// existing dead/float ratio while resizing to the broker truth.
		ps.DeadHold, ps.FloatHold = renormalise(ps.DeadHold, ps.FloatHold, assetTotal)
	}

	ps.TotalEquity = ps.CNYBalance + ps.DeadHold + ps.FloatHold + ps.ColdSealedHold

	if ps.ID == 0 {
		return tx.Create(&ps).Error
	}
	return tx.Save(&ps).Error
}

// renormalise rescales (dead, float) so their sum equals total while
// preserving their ratio. When the prior sum is zero we route everything
// to FloatHold — bootstrapping new instances always start as floating.
func renormalise(dead, float, total float64) (float64, float64) {
	prior := dead + float
	if prior <= 0 {
		return 0, total
	}
	ratio := total / prior
	return dead * ratio, float * ratio
}

// resolveInstanceForUser picks a RUNNING instance for orphan snapshots.
// In multi-instance setups the snapshot still updates one ledger; the
// audit log records the choice for forensics.
func resolveInstanceForUser(tx *gorm.DB, userID uint) (uint, error) {
	var inst store.StrategyInstance
	if err := tx.Where("user_id = ? AND status = ?", userID, store.InstanceStatusRunning).
		Order("id ASC").First(&inst).Error; err != nil {
		return 0, err
	}
	return inst.ID, nil
}

// writeAudit appends one event to the AuditLog table. We swallow JSON
// marshalling errors only in the sense of substituting a placeholder —
// the audit log is append-only and must never silently drop events.
func writeAudit(tx *gorm.DB, instanceID uint, event string, payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = json.RawMessage(fmt.Sprintf(`{"marshal_error":%q}`, err.Error()))
	}
	return tx.Create(&store.AuditLog{
		InstanceID: instanceID,
		EventType:  event,
		Payload:    raw,
	}).Error
}

// auditEventFor categorises the report shape for downstream querying.
func auditEventFor(r DeltaReport) string {
	switch {
	case r.ClientOrderID == "":
		return "delta_report_snapshot"
	case r.Execution != nil && r.Execution.Status == "failed":
		return "delta_report_failed"
	default:
		return "delta_report_filled"
	}
}

// engineFromOrderID extracts the MACRO/MICRO marker embedded in the
// canonical order-id format `inst{id}-{ENGINE}-{ts}`. Falls back to
// the empty string if the format is non-canonical — callers (Step())
// generate the id, so this is defensive only.
func engineFromOrderID(orderID string) string {
	// inst123-MACRO-1716800000  →  parts = [inst123, MACRO, 1716800000]
	first := -1
	for i := 0; i < len(orderID); i++ {
		if orderID[i] == '-' {
			first = i
			break
		}
	}
	if first < 0 {
		return ""
	}
	second := -1
	for i := first + 1; i < len(orderID); i++ {
		if orderID[i] == '-' {
			second = i
			break
		}
	}
	if second < 0 {
		return orderID[first+1:]
	}
	return orderID[first+1 : second]
}
