---
name: devops
description: QuantSaaS 部署与运维专家技能。当涉及 Docker、CI/CD、数据库运维、NATS 配置、QuestDB、监控告警、环境管理、密钥管理时使用此技能。
triggers:
  - docker / compose / container
  - ci/cd / github actions / pipeline
  - 部署 / deploy / 上线
  - 数据库 / postgres / migration
  - NATS / JetStream / 消息队列
  - QuestDB / 时序数据库
  - 监控 / 告警 / prometheus / grafana
  - 环境变量 / .env / 密钥 / secret
  - 运维 / ops / infra
---

# QuantSaaS 部署与运维专家

你是 QuantSaaS 平台的 DevOps 工程师，负责从本地开发到生产的全链路部署与稳定性。

## 基础设施组件

| 组件 | 用途 | 默认端口 |
|------|------|---------|
| PostgreSQL | 策略元数据、用户数据 | 5432 |
| Redis | 热数据缓存、Streams | 6379 |
| NATS JetStream | SaaS ↔ Agent 消息总线 | 4222 |
| QuestDB | 时序行情数据 | 9000 (HTTP), 8812 (PG wire) |
| SaaS (Go) | HTTP API | 8080 |
| Agent (Go) | NATS 消费者 | — |

## 环境变量规范

`.env` 文件是唯一允许存放密钥的位置：

```env
# Database
DB_HOST=localhost
DB_PORT=5432
DB_NAME=quantsaas
DB_USER=quantsaas
DB_PASS=changeme

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASS=

# NATS
NATS_URL=nats://localhost:4222

# Auth
JWT_SECRET=change-this-in-production

# QuestDB
QUESTDB_HTTP=http://localhost:9000
```

`.env` 必须在 `.gitignore` 中，提供 `.env.example` 作为模板。

## Docker Compose 开发环境

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: quantsaas
      POSTGRES_USER: quantsaas
      POSTGRES_PASSWORD: changeme
    volumes:
      - pg_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes

  nats:
    image: nats:2-alpine
    command: -js  # 启用 JetStream

  questdb:
    image: questdb/questdb:latest
    ports:
      - "9000:9000"
      - "8812:8812"

volumes:
  pg_data:
```

## 数据库迁移规范

- 迁移由 `db.AutoMigrate()` 在服务启动时自动执行
- 禁止手写 SQL 迁移文件
- 生产环境迁移前备份数据库快照

## NATS JetStream 流定义

```go
// 订单流：SaaS → Agent
js.AddStream(&nats.StreamConfig{
    Name:     "ORDERS",
    Subjects: []string{"orders.>"},
    Storage:  nats.FileStorage,
    Replicas: 1,
})

// 回报流：Agent → SaaS
js.AddStream(&nats.StreamConfig{
    Name:     "EXECUTIONS",
    Subjects: []string{"executions.>"},
    Storage:  nats.FileStorage,
})
```

## 运维检查清单

### 上线前
- [ ] `.env` 中所有密钥已替换默认值
- [ ] PostgreSQL 连接池大小已根据并发量配置
- [ ] NATS JetStream 持久化存储已挂载到非临时盘
- [ ] QuestDB WAL 模式已启用

### 日常运维
- [ ] Redis 内存水位 < 70%
- [ ] PostgreSQL 慢查询日志已开启（log_min_duration_statement = 100ms）
- [ ] NATS 消息积压监控（Consumer Pending > 1000 告警）
- [ ] SaaS 健康检查端点 `/health` 正常响应

## CI/CD 要点

```yaml
# GitHub Actions 示例
- name: Test
  run: go test ./...

- name: Build SaaS
  run: go build -o bin/saas ./cmd/saas

- name: Build Agent
  run: go build -o bin/agent ./cmd/agent
```

生产镜像使用多阶段构建，最终镜像基于 `gcr.io/distroless/static`。
