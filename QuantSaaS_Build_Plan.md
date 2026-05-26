# QuantSaas 从零复刻完整构建Plan
## 二、Sigmoid 动态天平 — 微观引擎设计哲学

> 这是一个真正的无状态天平。每个 tick，它只问一个问题：
> 当前持仓与信号所隐含的目标持仓之间，差距有多大？然后它施加一个与差距成比例的力，推动持仓收敛。

策略不持有"我昨天买了多少"的记忆，天平只感知当下信号与当前仓位的偏差，输出一个目标权重。这使回测与实盘的 `Step()` 调用路径完全相同，消除状态不一致的根源。

### 核心公式

```
Exponent   = -β × MarketBetaMultiplier × Signal + γ × (CurrentWeight - 0.5)
TargetWeight = 1 / (1 + exp(Exponent))
Δ          = TargetWeight - CurrentWeight
```

### 公式解读

| 场景 | Signal | Exponent 方向 | TargetWeight | 动作 |
|------|--------|--------------|-------------|------|
| 信号看空 | 正 | 增大 | < 0.5 | 减仓 |
| 信号看多 | 负 | 减小 | > 0.5 | 加仓 |
| 仓位 > 0.5, γ > 0 | 任意 | 额外增大 | 进一步降低 | 均值回归 |
| 仓位 < 0.5, γ > 0 | 任意 | 额外缩小 | 进一步拉高 | 均值回归 |

- **Signal**：策略信号，可以是均值回归信号、动量信号、确权信号，甚至多信号的加权合成——只要归一化为一个标量即可接入
- **β（渐进系数）**：越大调仓越频繁；市场状态感知可在极端行情时动态放大 β
- **γ（仓位偏置系数）**：γ=0 时纯信号驱动；γ>0 时叠加均值回归力，防止仓位从极端长期漂移
- **MarketBetaMultiplier**：来自上层市场状态机，行情极端趋势时放大 β 自动加速收敛

### GA 在这里象什么

Signal 通常由多个因子线性合成：

```
Signal = a × X1 + b × X2 + c × X3 + ...
```

其中 X1, X2, X3 是选择的市场特征（经量纲化后的标量），a, b, c 是对应的权重系数。这些系数就是**染色体的一部分**，由遗传算法在历史数据上搜索最优值。

以系统默认配置为例，三个特征选取的是**价格偏离（P）、动量（V）、加速度（A）**，GA 搜索的就是让 `a×P + b×V + c×A` 喂进 Sigmoid 后，在多时段历史回测中跑出最优绩效的 (a, b, c)。

你完全可以换成自己的特征组合，比如 RSI 偏离 + 成交量动量 + 布林带位置，GA 的搜索机制是完全通用的，它不关心 X 是什么，只负责找到系统在历史上表现最好的系数组合。

---

### 理论订单与模型区过滤
```
DeltaWeight = TargerWeight - CurrentWeight
TheoreticalUSD = DeltaWeight x TotalEquity
```
// TODO
粉尘拦截规则-- 待补充
Sigmoid 输出的 Δ 是**模型层的理论调仓幅度**，进入执行层前经过两道过滤：

1. **最小调仓阈值**：`|Δ| < MinRebalanceThreshold` 时不产生订单，避免频繁微调产生的手续费摩擦
2. **手数映射**：`TargetWeight × TotalCapital / Price` → 取整为合约手数（模型层输出，不含滑点假设）
3. **冲击成本叠加**：理论订单进入回测引擎时，按成交量比例叠加冲击成本模型（`ImpactBps = k × sqrt(Δ / ADV)`）

回测与实盘共享同一过滤逻辑，差异仅在于冲击成本参数的来源（历史估计 vs 实时报价）。

---

## Phase 1 — Sigmoid 动态天平核心实现

### Objective

实现 `internal/quant/` 下的 `sigmoid.go`，使其成为策略信号到目标仓位的**唯一转换层**。该函数是整个系统回测与实盘一致性的基石。

### Context

`Step()` 是系统的核心契约：策略包调用它，回测引擎调用它，实盘执行也调用它。任何对仓位的计算必须经过且只经过这里。

### Prompt

```
帮我在 internal/quant/sigmoid.go 中实现以下内容：

1. SigmoidEngine struct，字段：
   - Beta float64               // 渐进系数，控制调仓速度
   - Gamma float64              // 仓位偏置系数，均值回归力度
   - MinRebalanceThreshold float64  // 最小调仓阈值，低于此值不产生订单

2. Step(signal float64, currentWeight float64) (targetWeight float64, delta float64) 方法：
   - exponent = -Beta × signal + Gamma × (currentWeight - 0.5)
   - targetWeight = 1 / (1 + math.Exp(exponent))
   - delta = targetWeight - currentWeight
   - 若 math.Abs(delta) < MinRebalanceThreshold，delta 返回 0（不调仓）

3. 无任何 I/O、无全局状态、无外部依赖，纯函数风格

4. 配套单元测试 sigmoid_test.go，覆盖：
   - signal=0, gamma=0 → targetWeight ≈ 0.5
   - signal 极大（+10）→ targetWeight < 0.1（空仓信号）
   - signal 极小（-10）→ targetWeight > 0.9（满仓信号）
   - delta < MinRebalanceThreshold 时返回 delta=0
   - gamma > 0 时，currentWeight=0.8 产生额外减仓压力
```

### 预期产出
- `internal/quant/sigmoid.go`
- `internal/quant/sigmoid_test.go`

### 验证命令
```bash
go test ./internal/quant/... -v
# 人工验证：signal=-1.0, Beta=2.0, Gamma=0, currentWeight=0.5 → targetWeight ≈ 0.88
```

## Phase0 — 环境初始化与AI协作基础设施
以下是具体的vibe coding steps：
## Objective
在项目根目录建立AI工作约束文件（CLAUDE.md/AGENTS.md），让Claude在每次对话中自动加载项目规范。初始化Go项目依赖。

### Context
这一步决定了后续所有AI对话的质量。CLAUDE.md是AI的“宪法”，每次对话开始时自动读入。

### Prompt
```
帮我完成以下两件事
第一，在项目根目录创建各种约束文件文件，内容包括以下几部分：
你作为经验丰富的交易员，在docs/下先如果没有文档就帮我构造--QuanSaas系统总体拓扑结构.md 进化计算引擎.md and 纯粹策略数学引擎.md
“唯一功能真源”部分：声明当前功能只依据docs/下的三份文档（系统总体拓扑结构、策略数学引擎、进化计算引擎），三份文档没有定义的功能不进入实现。
“工作顺序”部分：列出四条规则—涉及策略和回测先读对应文档；涉及Go后端遵守GORMCode—First只用AutoMigrate；涉及价格计算优先无量纲表达；涉及架构边界保持Saas-Strategy-Agent分工不做预防性解。
“核心约束”部分：列出五条铁律—策略必须满足复利前置条件；回测与实盘调用同一Step（）实现；step（）只在SaaS侧执行；策略包内部禁止网络数据库文件I/0；API Key 只能在.env。
“代码目录”部分：列出cmd/saas/cmd/agent/internal/saas/internal/agent/internal/strategy/internal/strategies/[策略名]/internal/quant/internal/adapters/backtest/的各自职责说明。
“验证命令”部分：go list./.和 go test./.
第二,初始化Go项目,并安装以下依 gin gorm + postgres, go-redis golang-jwt, robfig/cron, rap gorilla/websocket,testify
第三，为整个项目构建一些基础的 SKTLLS，至少包含系统架构师、量化交易数学专家、go后端专家、部署与运维专家
```
### 预期产出
- CLAUDE.md
- go.mod + go.sum
- 其他目录骨架

---

## Phase 1 — 三份真源文档（需要自己填写内容）

### 目标

在写任何代码之前，用文档把系统的设计意图固化。这三份文档是整个系统的"法律"，是后续所有代码的唯一一份据。其中进化计算引擎已经有原始参考文件，直接参考即可。

> **重要说明**：下面给出的是三份文档的**结构骨架**，每个标题下的内容需要你自己填写。这是你的策略设计空间，没有标准答案。可以用自然语言与 AI 对话，描述你需要的效果，让 AI 补充内容。

---

### 1A. 系统总体拓扑结构文档

#### Prompt

```
帮我在 docs/ 目录下创建"系统总体拓扑结构.md"。这份文档定义系统有哪些物理端、
有哪些逻辑模块、状态如何在它们之间流转，以及系统的生命周期动作。不含任何具体策略公式。

文档结构如下，每个章节标题下用三行以上的文字描述清楚这部分的设计决策：

第 0 章：架构哲学

第 1 章：三端物理部署形态
（saas 云端 / agent 用户本地 / lab 算力机的各自职责与禁区）

第 2 章：app_role 三态行为矩阵
（saas/lab/dev 各自开放和限制哪些能力，用表格表示）

第 3 章：逻辑模块与职责边界
（Strategy 策略模块 / Instance 实例模块 / Evolution 进化模块 / Auth 认证模块，
明确每个模块的职责边界和禁区）

第 4 章：全局状态总线
（单一 Postgres + Redis 仅缓存；列一张数据所有权表，每类数据的真源在哪一端）
```

#### 预期产出
- `docs/系统总体拓扑结构.md`（内容由你填写，AI 辅助补充）

---

### 1B. 纯粹策略数学引擎文档

#### Prompt

```
帮我在 docs/ 目录下创建"纯粹策略数学引擎.md"。这份文档描述策略的数学骨架，
不含任何 Go 实现细节，只讲清楚"信号是什么、怎么产生、如何转化为仓位"。

文档结构如下：

第 0 章：设计哲学——为什么要把策略数学独立于代码

第 1 章：Signal 的定义域与值域
（Signal 归一化到什么范围？为什么？）

第 2 章：Sigmoid 动态天平公式
（完整公式、各参数语义、参数合理取值范围）

第 3 章：因子合成规则
（Signal = a×X1 + b×X2 + ...，X 的选取原则，权重 a/b/c 的物理意义）

第 4 章：理论订单与粉尘拦截
（DeltaWeight 计算、TheoreticalUSD、最小调仓阈值规则）

第 5 章：复利前置条件
（策略满足复利的充分条件：为什么不允许策略包持有状态）
```

#### 预期产出
- `docs/纯粹策略数学引擎.md`

---

### 1C. 进化计算引擎文档

#### Prompt

```
帮我在 docs/ 目录下创建"进化计算引擎.md"。这份文档描述 GA/进化系统的
运作机制，不含具体 Python 实现，只讲清楚"染色体是什么、适应度怎么定义、
进化结果如何回流到 SaaS"。

文档结构如下：

第 0 章：设计哲学——为什么进化和策略运行在不同进程

第 1 章：染色体定义
（哪些参数构成染色体？取值范围如何约束？）

第 2 章：适应度函数
（用什么指标评价一条染色体？Sharpe？Calmar？如何防过拟合？）

第 3 章：种群生命周期
（初始化 → 评估 → 选择 → 交叉变异 → 下一代，各步骤的实现策略）

第 4 章：与 SaaS 的接口协议
  POST /api/v1/evolution/tasks 的参数列表（pop_size, max_generations, spawn_mode）
  GET  /api/v1/evolution/tasks 的返回结构
  进化结果如何写回 strategy 参数表

第 5 章：过拟合防御机制
（IS/OOS 分割比例、Walk-Forward 验证、参数平原检测）
```

#### 预期产出
- `docs/进化计算引擎.md`

---

## Phase 2 — 基础设施层 (Config + DB + Auth)

### 目标

搭建系统的物理基础：配置加载、GORM 数据库模型定义（全量 AutoMigrate）、Redis 客户端、JWT 工具。

### Context

GORM Code-First 的核心意义：所有数据库结构变更通过修改 Go struct 来完成，AutoMigrate 自动同步，完全不存在手写 SQL 文件的观念。这是整个项目的 schema 管理哲学。

### Prompt

```
请阅读 docs/系统总体拓扑结构.md，然后为我完成 Go 项目的基础设施层。

总体约束：模块使用 gin 框架，ORM 为 GORM + postgres driver，日志使用 zap，
所有数据库模型只使用 GORM struct tag 定义，不写任何 SQL 文件。

请实现以下内容：

一、internal/saas/config/config.go
定义 Config 结构体（包含 AppRole / Database / Redis / JWT / Server 五个子配置），
实现从 config.yaml 文件加载的逻辑。AppRole 取值为 "saas" / "lab" / "dev"。
同时创建一份 config.yaml 模板，不含任何密钥，密钥字段留空并注明需通过环境变量注入。

二、internal/saas/store/models.go
用 GORM struct 定义所有核心数据模型：
- User（用户与订阅计划）：ID/Name/Email/PasswordHash/Plan/CreatedAt
- StrategyTemplate（策略模板）：ID/Name/Description/DefaultParams(JSON)/CreatedAt
- StrategyInstance（策略实例）：ID/UserID/TemplateID/Params(JSON)/Status/CreatedAt
- EvolutionTask（进化任务）：ID/InstanceID/Status/BestChromosome(JSON)/BestFitness/CreatedAt
- BacktestResult（回测结果）：ID/InstanceID/Params(JSON)/Sharpe/MaxDrawdown/CalmarRatio/CreatedAt
所有模型继承 gorm.Model，全部通过 AutoMigrate 创建表结构。

三、internal/saas/store/db.go
初始化 GORM + PostgreSQL 连接，暴露 *gorm.DB 单例，启动时执行 AutoMigrate。

四、internal/saas/store/redis.go
初始化 go-redis 客户端，暴露 *redis.Client 单例，用于缓存 JWT 黑名单与实例状态快照。

五、internal/saas/auth/jwt.go
实现 JWT 生成与验证：
- GenerateToken(userID uint, role string) (string, error)
- ValidateToken(token string) (*Claims, error)
- Claims 包含 UserID / Role / ExpiresAt
- 密钥从 Config.JWT.Secret 读取，禁止 hardcode

六、cmd/saas/main.go
组装以上模块：加载 config → 初始化 DB → 初始化 Redis → 启动 gin server
（仅健康检查路由 GET /health）
```

### 预期产出
- `internal/saas/config/config.go`
- `config.yaml`（模板，无密钥）
- `internal/saas/store/models.go`
- `internal/saas/store/db.go`
- `internal/saas/store/redis.go`
- `internal/saas/auth/jwt.go`
- `cmd/saas/main.go`

### 验证命令
```bash
go build ./cmd/saas/...
go test ./internal/saas/...
# 手动验证：启动后 curl http://localhost:8080/health 返回 {"status":"ok"}
```