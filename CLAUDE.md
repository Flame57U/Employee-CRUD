# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Authoritative Source of Truth

All features must be grounded in one of three documents under `docs/`:
- `QuanSaas系统总体拓扑结构.md` — system topology, deployment, data flows
- `纯粹策略数学引擎.md` — math engine: factor DSL, signal generation, portfolio optimization, statistical testing
- `进化计算引擎.md` — evo engine: GA/DE/PSO/CMA-ES algorithms, NSGA-II multi-objective, island model

**If a feature is not defined in these three documents, do not implement it.**

## Commands

```bash
go list ./...          # verify module structure compiles
go test ./...          # run all tests
go test ./internal/... # run a single package subtree
go test -run TestName ./path/to/pkg  # run a single test
go build ./cmd/saas    # build SaaS server
go build ./cmd/agent   # build agent binary
```

## Architecture

### Layer Separation (non-negotiable)

```
cmd/saas/            — SaaS HTTP server entry point
cmd/agent/           — agent binary entry point
internal/saas/       — SaaS-side business logic, API handlers
internal/agent/      — agent runtime, orchestration
internal/strategy/   — strategy interface definitions (no I/O allowed)
internal/strategies/ — one subdirectory per named strategy
internal/quant/      — Math Engine: factor DSL, signal generation, portfolio optimization
internal/adapters/backtest/ — backtest adapter (bridges quant engine to data)
```

### Five Iron Rules

1. **Compound interest prerequisite**: every strategy must satisfy the mathematical preconditions for compounding before any position sizing is computed.
2. **Unified execution path**: backtest and live trading both call the same `Step()` function — no divergent code paths.
3. **Step() is SaaS-side only**: `Step()` executes inside the SaaS layer; the agent only receives the resulting orders.
4. **Strategy packages are pure**: `internal/strategies/*/` and `internal/strategy/` must have zero network, database, or file I/O. All external state arrives via function arguments.
5. **Secrets in .env only**: API keys and credentials are read from `.env` and never hardcoded or committed.

### Go Backend Rules

- ORM: GORM Code-First only — schema is managed exclusively via `AutoMigrate`, never raw DDL.
- Price/return calculations: prefer dimensionless expressions (ratios, z-scores, ranks) over raw price values.

### Math Engine Concepts (Python, `internal/quant/`)

- **Factors** are pure functions decorated with `@factor(lookback, universe)` — no global state, no future data.
- **IC threshold**: only factors with ICIR > 0.3 are candidates.
- **Signal range**: always normalized to [-1, +1]; positive = long bias, negative = short bias.
- **Portfolio optimizers available**: MVO (Ledoit-Wolf covariance), CVaR (Rockafellar-Uryasev), Risk Parity, Maximum Diversification, Kelly (half-Kelly cap).

### Evo Engine Concepts (Python + Ray, `internal/` or separate service)

- **Algorithms**: GA, DE (DE/rand/1/bin default), PSO, CMA-ES (preferred for >20 dimensions).
- **Fitness**: use OOS Sharpe as primary, never IS Sharpe. Composite fitness weights: OOS Sharpe 35%, MaxDD 25%, Calmar 20%, Turnover 10%, IS/OOS ratio 10%.
- **Anti-overfitting**: fitness function penalizes IS/OOS split; `robustness_score > 0.8` required for Pareto front entry.
- **Island model**: 3 islands (GA/DE/PSO), migrate top 5% every 10 generations in a ring topology.
- **Pareto front output**: human must review and approve before any solution reaches `validated` status in the strategy registry.

### Execution Layer Latency Targets

| Component | Target |
|-----------|--------|
| Pre-trade risk check | < 10μs (synchronous, blocking) |
| Real-time risk monitor | < 1ms (async) |
| Factor DAG (live) | < 50μs per tick |
| Factor DAG (research) | < 500μs per tick |

### Strategy Registry Lifecycle

`draft → validated → live → deprecated`

Validated requires: human review + risk signature. Strategy math fingerprint (`sha256` of factor definition + signal logic) is immutable once registered — changes require a new version.

## Tech Stack

| Concern | Technology |
|---------|-----------|
| SaaS API | Go + Gin |
| Database ORM | GORM + PostgreSQL |
| Cache / streams | Redis |
| Auth | golang-jwt |
| Scheduling | robfig/cron |
| WebSocket | gorilla/websocket |
| Testing | testify |
| Math Engine | Python + NumPy/Polars |
| Evo Engine | Python + Ray |
| Backtest core loop | Rust + PyO3 |
| Factor DAG (live) | Rust |
| Execution (OMS/Risk) | Rust |
| Message bus | NATS JetStream |
| Time-series store | QuestDB |
| Hot data cache | Redis Streams |
| Strategy metadata | PostgreSQL |
| Workflow orchestration | Temporal |
