---
name: project-style
description: 统一项目多层架构写作风格，明确 handler、logic、repository（含 txn、cache）职责边界与协作规则，避免跨层越界。
argument-hint: [ topic-or-module ]
allowed-tools: Read, Write, Edit, AskUserQuestion
---

# Project Writing Style (project-style)

用于规范本项目在多层架构下的代码写作风格与职责隔离。目标是：

1. 每层只做自己该做的事。
2. 层与层之间只通过稳定接口协作。
3. 禁止“顺手做掉”导致的跨层污染。

---

## 适用范围

- `handler`（接口/传输层）
- `logic`（业务编排层）
- `repository`（数据访问层）
  - `repository/txn`（事务协调子层）
  - `repository/cache`（缓存读写与失效策略子层）

---

## 架构总览

```text
HTTP / RPC Request
   -> handler
   -> logic
   -> repository/txn (事务协调，可选，仅多表写入场景)
   -> repository
   -> DB / Redis / External Service
```

### 依赖方向（只能向下）

- `handler` -> `logic`
- `logic` -> `repository`（含 `repository/txn`）
- `repository/txn` -> `repository`
- `repository` -> `cache/client/db`

禁止反向依赖与旁路依赖：

- `handler` 直接调用 `repository`/`repository/txn`（禁止）
- `handler` 直接操作 `cache`（禁止）
- `logic` 直接解析 HTTP 参数（禁止）
- `logic` 直接操作数据库事务/Transaction（禁止，必须通过 `repository/txn` 子包）
- `repository` 承载业务决策（禁止）
- `repository/txn` 承载纯业务校验（禁止，校验属于 logic 层）

---

## 分层职责定义

## 1) handler 层（接口适配层）

**负责：**

- 接收请求（HTTP/RPC）并做基础绑定与格式校验。
- 将 transport DTO 转换为 logic 入参。
- 调用 logic 并将结果映射为响应 DTO。
- 统一处理错误到协议状态码/错误码。

**不负责：**

- 业务规则判断（如权限矩阵、状态流转、价格计算）。
- 持久化细节或缓存策略。
- 事务控制。

**写作要点：**

- 函数要短，重点是“转换 + 调用 + 映射”。
- 不在 handler 中出现 repository 类型。
- 错误文案面向接口消费者，不泄漏底层细节。

---

## 2) logic 层（业务编排层）

**负责：**

- 承载核心业务用例（use case）。
- 组合多个 repository 完成业务流程。
- 执行业务校验、幂等、事务边界、领域规则。
- 定义稳定的输入输出模型（面向上层/下层）。

**不负责：**

- HTTP 参数提取、响应序列化。
- SQL/Redis 语句细节。
- **框架耦合类型泄漏到方法签名：Logic 及以下所有层的方法签名必须使用标准库 `context.Context`，禁止使用 `*gin.Context`。**
- 数据库事务管理（Transaction 开启/提交/回滚）—— 必须委托给 `txn` 层。
- 直接操作 `*gorm.DB` 执行事务。

**写作要点：**

- 方法名用业务语义（如 `CreateUser`, `BanPlayer`, `RefreshProfile`）。
- **方法签名统一使用 `ctx context.Context` 作为第一个参数，不接受 `*gin.Context`。**
- 返回错误使用可判定类型（业务错误 vs 系统错误）。
- 业务流程可读性优先：按”校验 -> 执行 -> 收敛结果”组织。

---

## 2.5) repository/txn 子层（事务协调层）

**定位：** 位于 `logic` 与 `repository` 之间，作为 `repository` 包的子包，专门处理涉及多表写入的复合事务场景。与 `repository/cache` 同级，同属 repository 下的功能子层。

**负责：**

- 聚合多个 `repository` 实例，在单个数据库事务内完成跨表原子操作。
- 管理事务边界（开启 Transaction、提交、回滚）。
- 封装语义化的复合操作接口（如 `CreateSkinWithQuota`、`AddProfileWithQuota`）。
- 保证多步操作的原子性：任一步骤失败则整体回滚。

**不负责：**

- 业务校验（参数合法性、权限判断等属于 `logic` 层）。
- HTTP/RPC 协议转换。
- 外部服务调用（如 Bucket 上传）—— 此类操作应在 `logic` 层完成后再进入事务。
- 单表 CRUD（直接由 `repository` 处理即可，无需经过 `txn`）。

**写作要点：**

- 每个方法对应一个完整的业务用例（如"创建皮肤并扣减配额"），而非通用模板。
- 方法签名使用 `context.Context` 作为上下文，不耦合 Gin Context。
- 事务内调用 `repository` 的原子方法时传入 `tx *gorm.DB` 参数。
- **外部服务调用必须在事务外完成**，避免长事务占用数据库连接。
- 错误返回使用项目统一的 `*xError.Error` 类型。

**何时需要新建 TxnRepo：**

当一个业务用例需要同时写 入**两张或以上**的表，且这些写入必须原子成功或整体回滚时，应在 `repository/txn` 子包中创建对应的事务协调仓储。单表操作或纯读操作无需经过此层。

---

**负责：**

- 面向实体/聚合提供稳定 CRUD 接口。
- 隔离数据库实现细节（GORM/SQL/索引策略）。
- 提供查询条件、分页、排序等数据访问能力。

**不负责：**

- 业务流程编排。
- 业务策略分支（例如“是否允许创建”）。
- transport DTO 适配。

**写作要点：**

- 输入尽量是明确 query/filter 结构，而非无序参数堆。
- **方法签名统一使用 `ctx context.Context`，禁止使用 `*gin.Context`。**
- repository 返回领域可用的数据模型，不返回 HTTP 语义。
- 错误包装要保留可观测信息（操作、主键、关键参数）。

---

## 4) repository/cache 层（缓存策略子层）

**负责：**

- cache-aside / read-through 等缓存模式落地。
- key 设计、TTL 管理、失效策略。
- 缓存命中与回源逻辑。

**不负责：**

- 业务决策（例如封禁判定、权限判定）。
- HTTP/RPC 协议转换。

**写作要点：**

- Key 命名统一：`<domain>:<entity>:<id|hash>`。
- 所有写操作明确失效点（写后删缓存、双删或事件失效）。
- 允许缓存失败降级，但必须记录观测日志。

---

## 越界禁止清单（Hard Rules）

1. **MUST**: handler 只能依赖 logic，不得直接依赖 repository/txn/cache。
2. **MUST**: logic 只能通过 repository（含 repository/txn）接触数据源，不得拼接 SQL/Redis 命令。
3. **MUST**: logic 不得直接使用 `gorm.Transaction` 或操作 `*gorm.DB` 开启事务（必须通过 `repository/txn` 子包）。
4. **MUST**: repository/txn 内部不承载纯业务校验（参数校验、权限判断属于 logic 层）。
5. **MUST**: repository 不承载业务分支，不做”是否允许”的业务判断。
6. **MUST**: cache 只做性能优化，不得成为业务正确性的唯一来源。
7. **NEVER**: 在下层返回上层协议对象（例如 repository 返回 HTTP 状态码）。
8. **NEVER**: 跨层复用”顺手函数”破坏边界（例如 handler 调 util 直连 DB）。
9. **NEVER**: 在事务内调用外部服务（如 Bucket 上传），避免长事务占用连接。
10. **NEVER**: 在 logic、repository、repository/txn、repository/cache 层的方法签名中使用 `*gin.Context`。这些层必须使用标准库 `context.Context`，由 handler 层通过 `ctx.Request.Context()` 转换后传入。

---

## 分层目录建议

```text
internal/
  handler/
    user_handler.go
  logic/
    user_logic.go
  repository/
    user_repository.go
    txn/
      game_profile.go      # 游戏档案事务协调
      library.go            # 资源库事务协调（皮肤/披风）
    cache/
      user_cache_repository.go
  model/
    dto/
    entity/
```

说明：

- `handler`：协议适配与响应格式。
- `logic`：业务用例编排，纯逻辑校验。
- `repository/txn`：多表写入的事务协调，事务边界管理（与 cache 同级）。
- `repository`：单表持久化与查询。
- `repository/cache`：缓存实现细节。

---

## 示例 1：正确分层（伪代码）

```go
// handler（唯一允许使用 *gin.Context 的层）
func (h *UserHandler) Create(ctx *gin.Context) {
    req := bindCreateUserRequest(ctx)
    out, err := h.userLogic.CreateUser(ctx.Request.Context(), req.ToLogicInput()) // ← 转换为标准 context
    renderCreateUserResponse(ctx, out, err)
}

// logic（使用标准 context.Context）
func (l *UserLogic) CreateUser(ctx context.Context, in CreateUserInput) (CreateUserOutput, error) {
    if err := l.validator.ValidateCreateUser(in); err != nil {
        return CreateUserOutput{}, err
    }
    if exists, _ := l.userRepo.ExistsByUsername(ctx, in.Username); exists {
        return CreateUserOutput{}, ErrUsernameTaken
    }
    user, err := l.userRepo.Create(ctx, in.ToEntity())
    if err != nil {
        return CreateUserOutput{}, err
    }
    return ToCreateUserOutput(user), nil
}

// repository/cache（使用标准 context.Context）
func (r *UserRepository) GetByID(ctx context.Context, id int64) (User, error) {
    if user, ok := r.cache.GetUser(ctx, id); ok {
        return user, nil
    }
    user, err := r.db.GetUserByID(ctx, id)
    if err != nil {
        return User{}, err
    }
    _ = r.cache.SetUser(ctx, id, user)
    return user, nil
}
```

---

## 示例 2：错误分层（反例）

```go
// 错误：handler 直接依赖 repository 并做业务判断
func (h *UserHandler) Ban(ctx *gin.Context) {
    user := h.userRepo.GetByID(ctx, id) // 越界：handler -> repository
    if user.Role != "ADMIN" {          // 越界：业务规则放在 handler
        ctx.JSON(403, ...)
        return
    }
    ...
}
```

---

## 评审检查清单（PR Checklist）

- [ ] handler 仅做协议适配，无业务规则分支。
- [ ] logic 完整表达业务用例，不含 transport/db 框架泄漏，**不直接操作事务**，**方法签名使用 `context.Context` 而非 `*gin.Context`**。
- [ ] repository/txn 仅做多表事务协调，**不含纯业务校验**，事务内无外部服务调用。
- [ ] repository 仅做数据访问，不承载业务决策，**方法签名使用 `context.Context` 而非 `*gin.Context`**。
- [ ] cache 策略与失效点明确，失败可降级。
- [ ] 依赖方向单向向下，无跨层旁路调用。

---

## 何时需要 AskUserQuestion

当以下信息不清晰时必须询问：

1. 该需求属于”新增用例”还是”扩展现有用例”？
2. 是否涉及多表写入需要事务协调（是否需要在 `repository/txn` 子包新建 TxnRepo）？
3. 缓存策略采用 cache-aside 还是 read-through？
4. 错误语义是否有统一业务错误码体系？

---

## 一句话原则

> handler 负责”说人话（协议）”，logic 负责”做决策（业务）”，txn 负责”管事务（原子性）”，repository 负责”拿数据（存储）”，cache 负责”提性能（加速）”。
