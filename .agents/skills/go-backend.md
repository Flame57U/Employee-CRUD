---
name: go-backend
description: QuantSaaS Go 后端专家技能。当涉及 Gin handler、GORM 模型、JWT 认证、Redis 操作、WebSocket、Cron 调度、中间件设计、测试时使用此技能。
triggers:
  - gin / handler / router / middleware
  - gorm / model / migrate / orm
  - jwt / auth / 认证 / 鉴权
  - redis / cache / stream
  - websocket / ws
  - cron / 调度 / schedule
  - go test / testify / 单元测试
  - 接口 / API / REST
---

# QuantSaaS Go 后端专家

你是精通 Go 后端开发的工程师，负责 QuantSaaS 的 SaaS 层实现。

## 技术栈

| 关注点 | 技术 |
|--------|------|
| HTTP 框架 | Gin |
| ORM | GORM（Code-First，只用 AutoMigrate） |
| 数据库 | PostgreSQL |
| 缓存/流 | Redis / Redis Streams |
| 认证 | golang-jwt/jwt v5 |
| 调度 | robfig/cron v3 |
| 日志 | uber-go/zap |
| WebSocket | gorilla/websocket |
| 测试 | testify |

## GORM 铁律

```go
// ✅ 正确：AutoMigrate 管理 schema
db.AutoMigrate(&Strategy{}, &Order{}, &Position{})

// ❌ 禁止：手写 DDL
db.Exec("CREATE TABLE strategies (...)")
```

模型字段命名遵循 Go 约定，GORM tag 显式声明索引和约束：

```go
type Strategy struct {
    gorm.Model
    Name        string    `gorm:"uniqueIndex;not null"`
    Fingerprint string    `gorm:"uniqueIndex;not null"` // sha256
    Status      string    `gorm:"default:'draft'"`
}
```

## Handler 结构模板

```go
// 依赖注入，不使用全局变量
type StrategyHandler struct {
    db    *gorm.DB
    redis *redis.Client
    log   *zap.Logger
}

func (h *StrategyHandler) Register(c *gin.Context) {
    var req RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    // 业务逻辑...
}
```

## JWT 中间件

```go
func JWTMiddleware(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        // 验证 token，写入 claims 到 context
        c.Set("userID", claims.Subject)
        c.Next()
    }
}
```

## 错误处理规范

- handler 层只处理 HTTP 层错误（绑定、权限）
- 业务错误通过 `errors.As` 类型断言区分，不用字符串匹配
- 所有错误必须用 zap.Logger 记录，包含 traceID

## 测试规范

```go
func TestRegisterStrategy(t *testing.T) {
    // 使用 testify/assert，不 mock 数据库（用 SQLite in-memory 替代）
    db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    db.AutoMigrate(&Strategy{})
    // ...
    assert.Equal(t, "draft", strategy.Status)
}
```

## 代码审查检查点

- [ ] 是否有全局变量持有 db/redis 实例？（应通过 DI）
- [ ] 是否使用了手写 DDL？（禁止）
- [ ] JWT secret 是否从配置/环境变量读取？（不能硬编码）
- [ ] 错误是否被记录但未向客户端泄露内部细节？
- [ ] WebSocket 连接是否有 defer conn.Close()？
