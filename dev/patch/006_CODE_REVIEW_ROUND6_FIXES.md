# Patch 006: 代码审查第六轮修复 — 路由中间件污染、令牌竞态条件、安全加固

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：Patch 001 ~ Patch 005
> 基于审查：Yggdrasil 协议实现完整代码审查报告（4 Agent 并行深度审查）

---

## 一、缺陷背景

基于对 Yggdrasil 协议实现（Patch 001-005 已执行完毕）的第五轮深度审查，
发现 **2 个 Critical** 和 **3 个 Important** 级别缺陷。其中 C-1 为 Patch 005
声称已修复但实际未正确落地的问题（回归），C-2 为事务层原子性残留漏洞。

---

## 二、Critical 级别修复（2 项）

### 缺陷 C-1 [置信度: 98]：Bearer Auth 中间件作用域污染导致 #9 #10 接口不可用

#### 问题描述

Gin 的 `group.Use(middleware)` **原地修改** `group.Handlers` 并返回同一个 `*RouteGroup` 指针。
Patch 005 虽然创建了 `authRequired` 子组用于 #11 #12，但 #9 (ProfileQuery) 和 #10 (ProfilesBatchLookup)
仍然注册在已被 `Use()` 污染的 `yggGroup` 根组上，导致这两个本不需要认证的接口被强制要求 Bearer Token。

**实际影响**：
- Minecraft 服务端调用 `hasJoined` 成功后查询玩家皮肤属性 (#9) → 因缺少 Authorization 头被 **401 拒绝**
- Minecraft 服务端批量查询玩家 UUID (#10) → 同样被 **401 拒绝**
- **后果：多人服务器所有玩家的皮肤/披风无法正常加载**

**规范依据**：
- §5.9 `GET .../profile/{uuid}` — 消费者为 Minecraft 服务端/客户端，**无需认证**
- §5.10 `POST .../profiles/minecraft` — 消费者为 Minecraft 服务端，**无需认证**

#### 改动

**文件**：`internal/app/route/route_yggdrasil.go`

```go
// 改前（Patch 005 错误修复）：#9 #10 在 Use() 之后注册，继承 Bearer Auth 中间件 ❌
authRequired := yggGroup.Use(yggmiddleware.YggdrasilBearerAuth(r.context))
{
    yggGroup.GET("/sessionserver/session/minecraft/profile/:uuid", serverHandler.ProfileQuery)     // ⚠️
    yggGroup.POST("/api/profiles/minecraft", serverHandler.ProfilesBatchLookup)                    // ⚠️
    authRequired.PUT("/api/user/profile/:uuid/:textureType", shareHandler.UploadTexture)
    authRequired.DELETE("/api/user/profile/:uuid/:textureType", shareHandler.DeleteTexture)
}

// 改后：#9 #10 在 Use() 之前注册到 yggGroup，不受 Bearer Auth 影响 ✅
// #9: 查询角色属性（无需认证 — 必须在 Bearer Auth 中间件挂载之前注册）
yggGroup.GET("/sessionserver/session/minecraft/profile/:uuid", serverHandler.ProfileQuery)

// #10: 批量查询角色（无需认证 — 同上）
yggGroup.POST("/api/profiles/minecraft", serverHandler.ProfilesBatchLookup)

// 需 Bearer Token 认证的路由组（#11, #12 通过 Authorization 头认证）
// 注意：Gin 的 group.Use() 原地修改中间件链，此后注册到 yggGroup 的路由都会继承该中间件
authRequired := yggGroup.Use(yggmiddleware.YggdrasilBearerAuth(r.context))
{
    authRequired.PUT("/api/user/profile/:uuid/:textureType", shareHandler.UploadTexture)
    authRequired.DELETE("/api/user/profile/:uuid/:textureType", shareHandler.DeleteTexture)
}
```

---

### 缺陷 C-2 [置信度: 92]：RevokeOldestByUserID 存在 TOCTOU 竞态条件

#### 问题描述

`RevokeOldestByUserID` 方法执行两步非原子操作：

```
Step 1: SELECT id ... ORDER BY created_at ASC LIMIT 1   -- 查询最早令牌 ID
Step 2: UPDATE ... SET status = Invalid WHERE id = ?     -- 更新该令牌状态
```

虽然此方法被 `CreateWithQuotaCheck` 包裹在 `db.Transaction()` 内，
但 **SELECT 和 UPDATE 之间仍存在竞态窗口**。高并发下两个请求可能同时读到同一条"最早"记录，
最终导致令牌数量超出 `maxPerUser` 限制。

**攻击场景**：

```
时间线:
T1(请求A): COUNT → count=10 (达到上限)
T1(请求A): SELECT → 找到 token_id=100 (最早)
T2(请求B): COUNT → count=10 (A 未 COMMIT，B 读到相同快照)
T2(请求B): SELECT → 也找到 token_id=100 (同一条!)
T1(请求A): UPDATE token_id=100 → Invalid ✅
T1(请求A): CREATE new_token_A → 成功 ✅
T1(请求A): COMMIT ✅
T2(请求B): UPDATE token_id=100 → Invalid (重复更新)
T2(请求B): CREATE new_token_B → 成功
T2(请求B): COMMIT
最终结果: 原有 10 个 - token_1失效 + new_token_A + new_token_B = 11 个有效令牌!
```

#### 改动

**文件**：`internal/repository/game_token.go`

```go
// 改前：两步非原子操作（SELECT + UPDATE），存在 TOCTOU 窗口 ❌
func (r *GameTokenRepo) RevokeOldestByUserID(...) *xError.Error {
    var oldest entity.GameToken
    err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
        Select("id").
        Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
        Order("created_at ASC").
        First(&oldest).Error
    // ... 错误处理 ...
    if err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
        Where("id = ?", oldest.ID).
        Update("status", entity.GameTokenStatusInvalid).Error; err != nil {
        // ...
    }
    return nil
}

// 改后：单条原子 SQL，数据库引擎内完成查找+锁定+更新 ✅
func (r *GameTokenRepo) RevokeOldestByUserID(...) *xError.Error {
    result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
        Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
        Order("created_at ASC").
        Limit(1).
        Update("status", entity.GameTokenStatusInvalid)
    if result.Error != nil {
        return xError.NewError(ctx, xError.DatabaseError, "撤销最早游戏令牌失败", true, result.Error)
    }
    // RowsAffected == 0 表示该用户无有效令牌，属于正常情况
    return nil
}
```

**附带改动**：方法注释更新为说明使用单条原子操作的策略。

---

## 三、Important 级别修复（3 项）

### 缺陷 I-1 [置信度: 90]：SignoutUser 缺少恒定时间 bcrypt 比较（时序侧信道风险）

#### 问题描述

`AuthenticateUser` (auth.go:86) 在用户不存在时执行了 dummy bcrypt 比较（~100ms 延迟）以防止时序侧信道攻击。
但 `SignoutUser` 在同样场景下没有此保护。两者返回的错误消息相同，但在高精度时序测量下缺少 ~100ms
的计算延迟可被检测到差异，从而枚举账号存在性。

> 注：Patch 005 I-2 声称已修复此项，但代码审查确认 SignoutUser 中仍未添加 dummy bcrypt。
> 本次修复确保防护到位。

#### 改动

**文件**：`internal/logic/yggdrasil/auth.go:305-308`

```go
// 改前
if !found {
    return xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
}

// 改后
if !found {
    // 恒定时间比较：即使用户不存在也执行 bcrypt 比较，防止时序侧信道泄露账号存在性
    _ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashvaluefortimingattackprevention"), []byte(password))
    return xError.NewError(ctx, xError.ParameterError, "Invalid credentials. Invalid username or password.", true)
}
```

---

### 缺陷 I-2 [置信度: 88]：RefreshToken selectedProfileID 未做格式预校验

#### 问题描述

当 `selectedProfileID != ""` 时，代码直接将其传入 `GetByUserIDAndUUID` 进行数据库查询，
但没有先校验该字符串是否为合法的无符号 UUID 格式（32 位十六进制字符串）。
畸形输入会直达数据库层，可能产生异常查询行为或意外错误信息泄露。

#### 改动

**文件**：`internal/logic/yggdrasil/auth.go:198-202`

```go
// 改前
if selectedProfileID != "" {
    // 原令牌已绑定角色时不能再选择
    if oldToken.BoundProfileID != nil {

// 改后
if selectedProfileID != "" {
    // 预校验 selectedProfileID 是否为合法的无符号 UUID 格式
    if _, decodeErr := DecodeUnsignedUUID(selectedProfileID); decodeErr != nil {
        return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
    }

    // 原令牌已绑定角色时不能再选择
    if oldToken.BoundProfileID != nil {
```

---

### 缺陷 I-3 [置信度: 85]：JoinServer profileUUID 未做格式预校验

#### 问题描述

与 I-2 相同的问题模式。`JoinServer` 接收 `profileUUID` 参数后直接用于：
1. 字符串比较 (`EncodeUnsignedUUID(boundProfile.UUID) != profileUUID`)
2. 写入 Redis (`SessionData.ProfileUUID`)
3. 数据库查询 (`GetByUUIDUnsignedWithTextures`)

如果 `profileUUID` 不是合法的无符号 UUID 格式，这些操作的行为取决于下游容错能力。

#### 改动

**文件**：`internal/logic/yggdrasil/session.go:93-98`

```go
// 改前
func (l *YggdrasilLogic) JoinServer(...) *xError.Error {
    l.log.Info(ctx, "JoinServer - 记录客户端加入服务器会话")

    // 验证令牌有效性 ...

// 改后
func (l *YggdrasilLogic) JoinServer(...) *xError.Error {
    l.log.Info(ctx, "JoinServer - 记录客户端加入服务器会话")

    // 预校验 profileUUID 是否为合法的无符号 UUID 格式
    if _, decodeErr := DecodeUnsignedUUID(profileUUID); decodeErr != nil {
        return xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
    }

    // 验证令牌有效性 ...
```

---

## 四、修改文件清单

| 文件路径 | 缺陷 | 改动类型 |
|----------|------|----------|
| `internal/app/route/route_yggdrasil.go` | C-1 | 路由注册顺序调整（#9 #10 移至 Use() 前） |
| `internal/repository/game_token.go` | C-2 | RevokeOldestByUserID 重写为单条原子 SQL |
| `internal/logic/yggdrasil/auth.go` | I-1 + I-2 | SignoutUser 添加 dummy bcrypt + RefreshToken 添加 UUID 预校验 |
| `internal/logic/yggdrasil/session.go` | I-3 | JoinServer 入口添加 UUID 格式预校验 |

**无新建文件。**

---

## 五、执行顺序说明

所有 5 个缺陷互相独立，无执行顺序依赖。本次修复按 Critical → Important 顺序依次实施。

---

## 六、验证标准

- [x] `go build ./...` 编译通过（零错误零警告）
- [x] `route_yggdrasil.go` 中 #9 #10 路由注册位于 `yggGroup.Use()` **之前**
- [x] `route_yggdrasil.go` 中仅 #11 #12 注册在 `authRequired` 子组上
- [x] `RevokeOldestByUserID` 方法体为单条 `Update(...Order...Limit...)` 调用（无 SELECT+UPDATE 两步）
- [x] 全局搜索 `Select("id").*Order("created_at ASC").*First(&oldest)` 在 game_token.go 中结果为 0
- [x] `SignoutUser` 的 `!found` 分支包含 `bcrypt.CompareHashAndPassword` dummy 调用
- [x] `RefreshToken` 在 `selectedProfileID != ""` 后立即调用 `DecodeUnsignedUUID` 预校验
- [x] `JoinServer` 方法入口处包含 `DecodeUnsignedUUID(profileUUID)` 预校验
- [x] 规范合规性矩阵中 #9 #10 状态恢复为「可用」（无认证阻断）

---

## 七、与 Patch 005 的关系说明

| Patch 005 缺陷 | 状态 | 说明 |
|---------------|------|------|
| C-3 Bearer Auth 中间件激活 | **本次修正** | Patch 005 声称已修复但实际未落地（#9 #10 仍在污染后的 yggGroup 上） |
| I-2 认证接口时序侧信道 | **本次确认修复** | Patch 005 声称已修复但 SignoutUser 中实际缺失 |
| 其余 11 项 | 无需变更 | Patch 005 修复均已正确生效 |

---

## 八、审查覆盖范围声明

本轮审查基于 **4 路 Agent 并行深度审查**，覆盖以下全部代码层：

| Agent | 审查范围 | 发现数 |
|-------|----------|--------|
| Agent #1 | Logic 层（6 个文件） | I-1, I-2, I-3, M-2, M-3 |
| Agent #2 | Handler 层 + 中间件（8 个文件） | 0 新增（全部通过） |
| Agent #3 | Repository 层 + Entity 层（6 个文件） | **C-2**, I-1(Repo), M-1 |
| Agent #4 | 基础设施层（11 个文件） | **C-1** |

总计发现 **2 Critical + 3 Important + 2 Medium**，本次修复全部 Critical 和 Important，
Medium 级别因影响较低延后处理。
