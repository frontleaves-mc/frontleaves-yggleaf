# Patch 007: 安全加固与数据正确性修复

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：Patch 001 ~ Patch 006
> 基于审查：Yggdrasil 协议实现第七轮代码审查（5 路 Agent 并行深度审查）

---

## 一、缺陷背景

基于对 Yggdrasil 协议实现（Patch 001-006 已执行完毕）的第七轮深度审查，
在补丁覆盖范围外新发现 **2 个 Critical** 和 **6 个 Important** 级别缺陷。
其中 C-1 为性能与维护性风险，C-2 为并发数据一致性漏洞，其余为安全加固与可用性优化。

---

## 二、缺陷清单与修复

### 缺陷 C-1 [Critical]: JoinServer Handler 与 Logic 双重验证

#### 问题描述

`JoinServer` Handler（`client.go:274-299`）和 Logic 层（`session.go:100-121`）
对同一组条件执行了**完全重复的验证**：

| 验证项 | Handler 层 | Logic 层 | 重复度 |
|--------|-----------|----------|--------|
| ValidateGameToken | ✅ 274 行 | ✅ 101 行 | **100%** |
| BoundProfileID nil 检查 | ✅ 285 行 | ✅ 110 行 | **100%** |
| Profile 一致性比对 | ✅ 291-299 行 | ✅ 115-121 行 | **100%** |

每次 join 请求产生 **4 次 DB 查询**（Handler 2 次 + Logic 2 次），实际只需 2 次。
更关键的是**维护风险**：两处验证逻辑需同步修改，未来极易出现一处更新而另一处遗漏。

#### 改动

**文件**: `internal/logic/yggdrasil/session.go`

删除 Logic 层 `JoinServer` 方法中第 100-121 行的重复验证代码块
（ValidateGameToken + BoundProfileID 检查 + GetByID + UUID 比对），
仅保留 UUID 预校验（防御性编程，防止非法 UUID 写入 Redis）和 Redis Set 写入。

**决策说明**: Handler 层必须保留其验证逻辑。原因：
Handler 需精确控制 Yggdrasil 错误响应格式（403 `ForbiddenOperationException` vs 500 `InternalServerError`），
Logic 层只能返回通用 `*xError.Error` 无法区分错误类型。

```go
// 改后 JoinServer 方法体（session.go:92-118）
func (l *YggdrasilLogic) JoinServer(...) *xError.Error {
    // 预校验 profileUUID 格式
    if _, decodeErr := DecodeUnsignedUUID(profileUUID); decodeErr != nil { ... }

    // Handler 层已完成令牌有效性、角色绑定、一致性验证（Yggdrasil 错误格式控制）
    // Logic 层仅负责 Redis 会话写入。

    sessionData := &cache.SessionData{ ... }
    if err := l.repo.sessionCache.Set(ctx, serverId, sessionData); err != nil { ... }
    return nil
}
```

---

### 缺陷 C-2 [Critical]: RefreshToken TOCTOU 竞态条件

#### 问题描述

RefreshToken 流程中，令牌状态检查 `oldToken.Status != Valid` 在**事务外**执行（`auth.go:172-175`），
`RevokeAndCreate` 在**事务内**执行。高并发下两个请求可能同时看到 Valid 状态：

```
T1: 检查 token_A = Valid ✅ → 进入事务 → 吊销A + 创建 new_A → COMMIT ✅
T2: 检查 token_A = Valid ✅（T1 未 COMMIT）→ 进入事务 → 吊销A(重复) + 创建 new_B → COMMIT
结果: 原有 N 个 - 1失效 + new_A + new_B = N+1 个有效令牌!
```

#### 三步修复

##### 步骤 1: 新增 `RevokeValidOrTempInvalid` 方法

**文件**: `internal/repository/game_token.go`（新增方法，位于 `UpdateBoundProfile` 之后）

```go
func (r *GameTokenRepo) RevokeValidOrTempInvalid(
    ctx context.Context, tx *gorm.DB, tokenID xSnowflake.SnowflakeID,
) (int64, *xError.Error) {
    result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
        Where("id = ? AND status IN (?, ?)",
            tokenID, entity.GameTokenStatusValid, entity.GameTokenStatusTempInvalid).
        Update("status", entity.GameTokenStatusInvalid)
    if result.Error != nil {
        return 0, xError.NewError(ctx, xError.DatabaseError, "条件性吊销令牌失败", true, result.Error)
    }
    return result.RowsAffected, nil
}
```

设计参考同文件 `RevokeOldestByUserID`（Patch 006 已改为单条原子 UPDATE SQL）的模式。

##### 步骤 2: 修改 `RevokeAndCreate` 使用条件更新

**文件**: `internal/repository/txn/game_token.go`

将事务内 Step 1 从 `UpdateStatus` 改为 `RevokeValidOrTempInvalid`，
新增 `RowsAffected == 0` 检测——此时说明令牌已被其他并发请求吊销：

```go
// 事务内 Step 1（改前）
_, updateErr := t.gameTokenRepo.UpdateStatus(ctx, tx, oldTokenID, Invalid)

// 事务内 Step 1（改后）
rowsAffected, revokeErr := t.gameTokenRepo.RevokeValidOrTempInvalid(ctx, tx, oldTokenID)
if revokeErr != nil { bizErr = revokeErr; return revokeErr }
if rowsAffected == 0 {
    bizErr = xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
    return bizErr
}
```

##### 步骤 3: 移除事务外状态检查

**文件**: `internal/logic/yggdrasil/auth.go:172-175`

删除以下代码块：
```go
// 删除（已移入 RevokeAndCreate 事务内部）
if oldToken.Status != entity.GameTokenStatusValid &&
   oldToken.Status != entity.GameTokenStatusTempInvalid {
    return "", "", nil, nil, xError.NewError(...)
}
```

状态检查已移入事务内部，`RowsAffected == 0` 时返回 `"Invalid token."`，
Handler 将此 `ParameterError`(40011) 映射为 403 `ForbiddenOperationException`，符合 Yggdrasil 规范。

---

### 缺陷 I-1 [Important]: RSA 私钥文件权限不安全

#### 问题描述

`os.Create()` 创建私钥文件时未指定权限，默认受 umask 影响（通常为 `0644`，world-readable）。
同机其他用户可读取 RSA 私钥 → 伪造 textures 签名 → 注入恶意材质 URL。

#### 改动

**文件**: `internal/app/startup/prepare/prepare_rsa.go`

| 行号 | 改前 | 改后 |
|------|------|------|
| 76 | `os.Create(privKeyPath)` | `os.OpenFile(privKeyPath, os.O_RDWR\|os.O_CREATE\|os.O_TRUNC, 0600)` |
| 90 | `os.Create(pubKeyPath)` | `os.OpenFile(pubKeyPath, os.O_RDWR\|os.O_CREATE\|os.O_TRUNC, 0644)` |

---

### 缺陷 I-2 [Important]: bcrypt Dummy Hash 格式非法导致时序防护失效

#### 问题描述

AuthenticateUser（第 86 行）和 SignoutUser（第 307 行）使用的 dummy hash：
```
$2a$10$dummyhashvaluefortimingattackprevention
```
salt 部分（`dummyhashvaluefortimingattackprevention`）为 **53 字符**，超过 bcrypt 要求的 **22 字符**。
Go `golang.org/x/crypto/bcrypt` 库因格式非法**立即 O(1) 返回**，无法达到恒定时间比较目的。
攻击者可通过响应时间差异枚举账号存在性。

#### 改动

**文件**: `internal/logic/yggdrasil/auth.go`（2 处替换）

```
改前: $2a$10$dummyhashvaluefortimingattackprevention  (非法格式, O(1))
改后: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy  (合法 60 字符 hash)
```

替换值为 bcrypt 官方测试向量（对应空密码），确保执行完整恒定时间比较流程。

---

### 缺陷 I-3 [Important]: 认证接口缺少输入长度限制（DoS 向量）

#### 问题描述

`AuthenticateRequest` / `SignoutRequest` 的 Username / Password 字段仅有 `binding:"required"` 无长度限制。
攻击者可发送超长密码（100KB+），即使密码错误服务端仍需执行完整 bcrypt 计算（cost>=10），
少量并发请求即可耗尽 CPU 资源造成 DoS。

#### 改动

**文件**: `api/yggdrasil/request.go`（6 个 DTO 共 12 个字段）

| DTO 字段 | 改前 | 改后 | 依据 |
|----------|------|------|------|
| AuthenticateRequest.Username | `binding:"required"` | `binding:"required,max=320"` | RFC 5321 email max |
| AuthenticateRequest.Password | `binding:"required"` | `binding:"required,max=128"` | 合理密码上限 |
| SignoutRequest.Username | `binding:"required"` | `binding:"required,max=320"` | 同上 |
| SignoutRequest.Password | `binding:"required"` | `binding:"required,max=128"` | 同上 |
| JoinServerRequest.AccessToken | `binding:"required"` | `binding:"required,max=368"` | UUID 36 + 余量 |
| JoinServerRequest.SelectedProfile | `binding:"required"` | `binding:"required,max=32"` | 无符号 UUID 固定 32 |
| JoinServerRequest.ServerID | `binding:"required"` | `binding:"required,max=256"` | 服务端随机串 |
| RefreshRequest.AccessToken | `binding:"required"` | `binding:"required,max=368"` | 同上 |
| RefreshRequest.ClientToken | （无 tag） | `,max=368` | 同上 |
| ValidateRequest.AccessToken | `binding:"required"` | `binding:"required,max=368"` | 同上 |
| ValidateRequest.ClientToken | （无 tag） | `,max=368` | 同上 |
| InvalidateRequest.AccessToken | `binding:"required"` | `binding:"required,max=368"` | 同上 |
| InvalidateRequest.ClientToken | （无 tag） | `,max=368` | 同上 |

参考先例：`api/library/skin.go:14` 使用 `binding:"required,oneof=1 2"` 验证枚举值。

---

### 缺陷 I-4 [Important]: HasJoined 删除失败行为过严

#### 问题描述

HasJoined 验证通过后 Redis Delete 失败时返回错误（`CacheError` → HTTP 500），
导致玩家无法进服。但会话 TTL 仅 30 秒自然过期已是安全兜底，
Redis 临时故障（网络抖动/主从切换）不应阻止正常业务流程。

#### 改动

**文件**: `internal/logic/yggdrasil/session.go:67-71`

```go
// 改前:
if delErr := l.repo.sessionCache.Delete(ctx, serverId); delErr != nil {
    return nil, false, xError.NewError(ctx, xError.CacheError, "删除会话缓存失败", true, delErr)
}

// 改后:
if delErr := l.repo.sessionCache.Delete(ctx, serverId); delErr != nil {
    l.log.Warn(ctx, fmt.Sprintf("删除会话缓存失败（TTL 兜底仍生效）: %v", delErr))
}
```

---

### 缺陷 I-5 [Important]: ProfileQuery 缺少 UUID 格式预检

#### 问题描述

JoinServer（`session.go:96`）和 RefreshToken（`auth.go:200`）都正确调用了 `DecodeUnsignedUUID` 做格式预检，
但 ProfileQuery → `QueryProfile` 路径上**没有**此校验。任意非 32 十六进制字符字符串会穿透到数据库查询层。

#### 改动

**文件**: `internal/handler/yggdrasil/server/server.go:97-106`

在空值检查之后插入十六进制字符校验（避免跨包依赖 `DecodeUnsignedUUID` 函数）：

```go
// 预校验 UUID 格式（与 JoinServer/RefreshToken 保持一致）
// 无符号 UUID 为 32 个十六进制字符（0-9, a-f, A-F）
if len(uuid) != 32 {
    ctx.Status(http.StatusNoContent)
    return
}
for _, c := range uuid {
    if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
        ctx.Status(http.StatusNoContent)
        return
    }
}
```

返回 204 而非 400：Yggdrasil 规范规定查询不存在的角色返回 204，非法 UUID 语义等价于"不存在的角色"。

---

### 缺陷 I-6 [Important]: 材质上传无谓 I/O 与安全标记缺失

#### 问题 1: 功能未实现时仍执行文件 I/O

UploadTexture 当前返回 501 NotImplemented，但在返回前仍执行了：
1. `ctx.Request.FormFile("file")` — 读取上传文件到内存
2. `header.Size > 1MB` 大小检查
3. `header.Header.Get("Content-Type")` Content-Type 检查

功能未实现时这些操作都是无谓的 I/O 开销。

#### 问题 2: TODO 安全要求标记不足

Phase 3 实现材质上传时需满足的安全要求（规范 §11.1 标记为"**必须**"）仅在普通 TODO 注释中，
容易被后续实现者遗漏，造成 RCE 或 PNG Bomb 漏洞。

#### 改动

**文件**: `internal/handler/yggdrasil/share/share.go:96-111`

两处改动：

1. **501 返回提前至文件 I/O 之前**：将 501 响应移至 `verifyOwnership` 之后、`FormFile` 之前
2. **TODO 升级为 `[SECURITY-MUST]` 标记**：添加详细安全注释清单

```go
// 验证角色归属
if _, ok := h.verifyOwnership(ctx, gameToken, uuid); !ok { return }

// [SECURITY-MUST] Phase 3 实现时必须满足以下安全要求：
//   1. 必须使用 magic number (89 50 4E 47 0D 0A 1A 0A) 或 http.DetectContentType
//      （不可仅信任客户端声明的 Content-Type header）
//   2. 必须解析 PNG 并去除非位图数据（嵌入的 tEXt/iTXt/zTXt chunk 可被滥用）
//   3. 必须验证像素尺寸并限制解压后内存（防 PNG Bomb DoS）
//   4. 文件大小建议收紧至 256KB 以内
//
// 当前提前返回 501 以避免功能未实现时执行无谓的文件 I/O
apiYgg.AbortYggError(ctx, http.StatusNotImplemented, "NotImplemented", "材质上传功能尚未实现")
```

---

## 三、修改文件清单

| 文件路径 | 缺陷 | 改动类型 |
|----------|------|----------|
| `internal/logic/yggdrasil/session.go` | C-1, I-4 | 删除 ~22 行重复验证 + 删除失败容错 |
| `internal/logic/yggdrasil/auth.go` | C-2, I-2 | 移除事务外状态检查 + 替换 dummy hash（2 处） |
| `internal/repository/game_token.go` | C-2 | 新增 `RevokeValidOrTempInvalid` 方法（~15 行） |
| `internal/repository/txn/game_token.go` | C-2 | `RevokeAndCreate` 改用条件更新 + RowsAffected 检查 |
| `api/yggdrasil/request.go` | I-3 | 6 个 DTO 共 12 个字段补充 `max=` binding tag |
| `internal/handler/yggdrasil/server/server.go` | I-5 | ProfileQuery 添加十六进制 UUID 格式预检 |
| `internal/handler/yggdrasil/share/share.go` | I-6 | 501 提前返回 + SECURITY-MUST 安全标记 + 移除无谓 I/O |
| `internal/app/startup/prepare/prepare_rsa.go` | I-1 | `os.Create` → `os.OpenFile(0600/0644)` |

**新增方法**: 1 个 (`GameTokenRepo.RevokeValidOrTempInvalid`)
**新建文件**: 无
**删除代码行**: ~25 行
**新增代码行**: ~60 行

---

## 四、执行顺序说明

所有 8 个缺陷按 Critical → Important 排列。实施分三阶段：

```
Phase 1（可并行，无依赖）:
  ├── C-1: session.go 删除重复验证          [Logic 层简化]
  ├── I-1: prepare_rsa.go 文件权限           [基础设施]
  ├── I-2: auth.go 替换 dummy hash       [安全加固]
  ├── I-3: request.go 补充 binding tag         [DTO 校验]
  ├── I-5: server.go UUID 预检              [输入校验]
  └── I-6: share.go 501 提前 + 安全标记     [安全设计]

Phase 2（独立，可与 Phase 1 并行）:
  └── I-4: session.go HasJoined 容错          [可用性]

Phase 3（C-2 三步顺序执行）:
  ├── game_token.go: 新增 RevokeValidOrTempInvalid
  ├── txn/game_token.go: RevokeAndCreate 条件更新
  └── auth.go: 删除事务外状态检查
```

注意：C-1 和 I-4 都修改 `session.go` 但不同方法（JoinServer vs HasStarted），无冲突。
C-2 和 I-2 都修改 `auth.go` 但不同方法（RefreshToken vs AuthenticateUser/SignoutUser），建议顺序实施。

---

## 五、验证标准

- [x] `go build ./...` 编译通过（零错误零警告）
- [x] C-1: `session.go` JoinServer 方法体仅含 UUID 预校验 + Redis Set（无 ValidateGameToken/GetByID 调用）
- [x] C-2: `game_token.go` 含 `RevokeValidOrTempInvalid` 方法（WHERE status IN + RowsAffected 返回）
- [x] C-2: `txn/game_token.go` RevokeAndCreate 使用 `RevokeValidOrTempInvalid` 替代 `UpdateStatus`
- [x] C-2: `auth.go:172-175` 无 Status 检查代码（已移入事务内）
- [x] I-1: `prepare_rsa.go` 仅使用 `os.OpenFile`（无 `os.Create`）
- [x] I-2: 全局搜索 `dummyhashvaluefortimingattackprevention` 结果为 0
- [x] I-3: 所有 Yggdrasil Request DTO 的敏感字段含 `max=` 限制
- [x] I-4: `session.go:69-71` 为 Warn 日志 + 无 return error
- [x] I-5: `server.go` ProfileQuery 含 `len(uuid) != 32` 十六进制校验
- [x] I-6: `share.go` UploadTexture 在 FormFile 之前返回 501 + 含 `[SECURITY-MUST]` 注释
