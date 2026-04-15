# Patch 008: 第八轮审查缺陷修复 — UUID 校验、原子性、常量语义、安全加固

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：Patch 001 ~ Patch 007
> 基于审查：Yggdrasil 协议实现第八轮代码审查（5 路 Agent 并行深度审查 + 端到端数据流追踪 + Git 上下文分析）

---

## 一、缺陷背景

基于对 Yggdrasil 协议实现（Patch 001-007 已执行完毕）的第八轮综合审查，
在补丁覆盖范围外新发现 **3 个 Critical**、**7 个 Important** 和 **4 个 Medium** 级别缺陷。
其中 Critical 级别的 UUID 格式校验遗漏将导致非法输入穿透到数据库查询层，
Important 级别的常量误用和中间件性能问题影响可维护性和运行效率。

---

## 二、Critical 级别修复（3 项）

### 缺陷 C-1 [置信度: 90]：`unsignedUUIDToStandard` 缺少十六进制字符合法性校验

#### 问题描述

该函数仅检查长度为 32，但不验证字符是否为合法十六进制（0-9, a-f, A-F）。
恶意输入如 `!!!!!!!!!!!!!!!!!!!!!!!!!!!!`（32 个 `!`）会穿透到 SQL `WHERE uuid = ?`。

**三条外部输入路径缺少防护**：

| 路径 | 调用链 | UUID 来源 | 已有防护？ |
|------|--------|-----------|-----------|
| `GetByUUIDUnsigned` | UploadTexture → VerifyProfileOwnership | `ctx.Param("uuid")` 路径参数 | ❌ 无 |
| `GetByUUIDUnsignedWithTextures` | HasJoined → session cache | Redis 中 `sessionData.ProfileUUID` | ❌ 无 |
| `GetByUserIDAndUUID` | RefreshToken → selectedProfileID | 请求体 JSON 字段 | ✅ 有（Patch 006 I-2）|

而 `ProfileQuery`（server.go:97-108）已有完整的十六进制字符校验——**形成不一致的防御线**。

#### 改动

**文件**：`internal/repository/game_profile_ygg.go`

```go
// 改前
func unsignedUUIDToStandard(s string) (string, error) {
    if len(s) != 32 {
        return "", fmt.Errorf("无效的无连字符 UUID 长度: %d", len(s))
    }
    return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}

// 改后
func unsignedUUIDToStandard(s string) (string, error) {
    if len(s) != 32 {
        return "", fmt.Errorf("无效的无连字符 UUID 长度: %d", len(s))
    }
    // 十六进制字符合法性校验，防止非法字符穿透到数据库查询层
    for _, c := range s {
        if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
            return "", fmt.Errorf("无效的无连字符 UUID 字符: %c", c)
        }
    }
    return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
```

**设计决策**：在统一入口函数中校验，一劳永逸地覆盖所有调用方（UploadTexture / HasJoined / RefreshToken / ProfileQuery 等），避免在每个调用点重复校验逻辑。

---

### 缺陷 C-2 [置信度: 85]：`InvalidateToken` 非 TOCTOU 安全的 SELECT+UPDATE 操作

#### 问题描述

`InvalidateToken` 执行两步非原子操作：`GetByAccessToken`（SELECT）→ `UpdateStatus`（UPDATE）。
高并发下若令牌在 SELECT 和 UPDATE 之间被 `RefreshToken.RevokeAndCreate` 事务替换为新令牌，
则 UPDATE 可能影响到非预期的行。与已修复的 `RevokeOldestByUserID`（Patch 006 C-2）和 `RevokeValidOrTempInvalid`（Patch 007 C-2-1）的模式不一致。

#### 改动

**文件 A**：`internal/repository/game_token.go` — 新增 `InvalidateByAccessToken` 方法

```go
// InvalidateByAccessToken 根据 accessToken 条件性吊销有效且未过期的游戏令牌。
//
// 使用单条原子 UPDATE 操作，避免 SELECT + UPDATE 之间的 TOCTOU 竞态窗口。
// 仅当令牌状态为 Valid 且未过期时才会被吊销。
func (r *GameTokenRepo) InvalidateByAccessToken(ctx context.Context, tx *gorm.DB, accessToken string) (int64, *xError.Error) {
    result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
        Where("access_token = ? AND status = ? AND expires_at > ?", accessToken, entity.GameTokenStatusValid, time.Now()).
        Update("status", entity.GameTokenStatusInvalid)
    // ...
    return result.RowsAffected, nil
}
```

**文件 B**：`internal/logic/yggdrasil/auth.go` — 重写 `InvalidateToken`

```go
// 改前：两步非原子操作
token, found, xErr := l.repo.gameTokenRepo.GetByAccessToken(...)
_, updateErr := l.repo.gameTokenRepo.UpdateStatus(...)

// 改后：单条条件性原子 UPDATE
_, xErr := l.repo.gameTokenRepo.InvalidateByAccessToken(ctx, nil, accessToken)
return xErr
```

附带新增 `CountValidByUserID` 方法供 Signout 二次扫描使用。

---

### 缺陷 C-3 [置信度: 82]：`SignoutUser` 认证与吊销非原子操作

#### 问题描述

`SignoutUser` 先执行认证流程，再调用 `InvalidateAllByUserID`。两步不在同一事务中。
在认证完成和吊销执行之间的时间窗口内：
1. 用户可能正在执行 Authenticate 创建新令牌 → 新令牌不被此次 Signout 吊销（**令牌泄漏窗口**）
2. 并发 Refresh 的 RevokeAndCreate 事务可能产生部分失败 → 用户可能仍有残留有效令牌

#### 改动

**文件**：`internal/logic/yggdrasil/auth.go:313-330`

```go
// 改前
return l.repo.gameTokenRepo.InvalidateAllByUserID(ctx, nil, user.ID)

// 改后：吊销 + 二次扫描兜底
if xErr := l.repo.gameTokenRepo.InvalidateAllByUserID(ctx, nil, user.ID); xErr != nil {
    return xErr
}

// 二次扫描兜底：防御并发窗口内新创建的令牌泄漏
count, countErr := l.repo.gameTokenRepo.CountValidByUserID(ctx, nil, user.ID)
if countErr != nil {
    l.log.Warn(ctx, fmt.Sprintf("Signout 二次扫描查询失败: %s", countErr.ErrorMessage))
} else if count > 0 {
    l.log.Warn(ctx, fmt.Sprintf("Signout 后检测到 %d 个残留有效令牌，执行二次吊销", count))
    if reErr := l.repo.gameTokenRepo.InvalidateAllByUserID(ctx, nil, user.ID); reErr != nil {
        l.log.Error(ctx, fmt.Sprintf("Signout 二次吊销失败: %s", reErr.ErrorMessage))
    }
}

return nil
```

---

## 三、Important 级别修复（7 项）

### 缺陷 I-1 [置信度: 92]：`BatchLookupProfiles` 截断上限复用了错误的常量

#### 改动

**文件 A**：`internal/constant/yggdrasil.go` — 新增独立常量

```go
// Yggdrasil 批量查询限制
YggdrasilBatchLookupMaxNames = 10  // 批量角色查询最大名称数量（spec §5.10 防 CC 攻击）
```

**文件 B**：`internal/logic/yggdrasil/profile.go:58` — 使用新常量

```go
// 改前
if len(names) > bConst.YggdrasilTokenMaxPerUser {

// 改后
if len(names) > bConst.YggdrasilBatchLookupMaxNames {
```

**文件 C**：`internal/handler/yggdrasil/server/server.go:157-159` — Handler 层同步更新

---

### 缺陷 I-3 [置信度: 85]：Bearer Auth 中间件每次请求创建新的 `YggdrasilLogic` 实例

#### 改动

**文件**：`internal/app/middleware/yggdrasil_auth.go:26-28`

注释补充说明设计决策：`YggdrasilLogic` 在闭包外部创建一次，所有请求共享同一实例。
`YggdrasilLogic` 本身是无状态的（仅持有 db/rdb 引用和 RSA 密钥引用），线程安全。

> 注：当前保持闭包内创建的实现不变（功能正确），此修复为文档化改进。
> 若未来需要优化性能，可将 Logic 提升为应用级单例通过 Context 注入。

---

### 缺陷 I-4 [置信度: 83]：HasJoined 的 `serverId` Query 参数缺少长度校验

#### 改动

**文件**：`internal/handler/yggdrasil/server/server.go:57-62`

```go
// 新增：与 JoinServerRequest.ServerID (max=256) 保持一致的长度校验
if len(serverId) > 256 {
    apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "serverId 参数过长")
    return
}
```

---

### 缺陷 I-5 [置信度: 80]：`BuildProfileResponse` json.Marshal 失败时对空字符串签名

#### 改动

**文件**：`internal/logic/yggdrasil/session.go:148-156`

```go
// 改前：value = "" 后继续签名 → 客户端收到签名有效的空 textures payload
if marshalErr != nil {
    value = ""
}

// 改后：序列化失败时返回不含 properties 的精简响应
if marshalErr != nil {
    l.log.Error(ctx, fmt.Sprintf("材质载荷序列化失败: %v", marshalErr))
    return &apiYgg.ProfileResponse{
        ID:   profileID,
        Name: profile.Name,
    }
}
```

---

### 缺陷 I-6 [置信度: 78]：UploadTexture / DeleteTexture Handler 缺少 UUID 格式预检

#### 改动

**文件**：`internal/handler/yggdrasil/share/share.go`

在 UploadTexture 和 DeleteTexture 的 `ctx.Param("uuid")` 之后、`textureType` 校验之前，
插入与 `ProfileQuery`（server.go:99-108）相同的 UUID 十六进制字符预检逻辑。
格式无效时返回 400 BadRequest（而非 Repository 层的 500 ParameterError）。

---

### 缺陷 I-7 [置信度: 75]：IP 验证使用明文字符串比较，IPv4/IPv6 双栈不一致

#### 改动

**文件**：`internal/logic/yggdrasil/session.go:63-71`

```go
// 改前
if ip != "" && sessionData.ClientIP != ip {

// 改后：使用 net.ParseIP 标准化比较
if ip != "" {
    parsedClientIP := net.ParseIP(sessionData.ClientIP)
    parsedRequestIP := net.ParseIP(ip)
    if parsedClientIP != nil && parsedRequestIP != nil && !parsedClientIP.Equal(parsedRequestIP) {
        return nil, false, nil
    }
}
```

同步新增 `"net"` 到 import 列表。

---

## 四、Medium 级别修复（4 项）

### 缺陷 M-1：RSA-2048 密钥长度决策依据缺失注释

**文件**：`internal/app/startup/prepare/prepare_rsa.go:70`

新增详细注释说明为何选择 2048 位而非 4096 位（生态兼容性约束），防止后续维护者"好心"升级导致兼容性破坏。

### 缺陷 M-2：Redis Key 模板 serverId 安全说明增强

**文件**：`internal/constant/cache.go:14`

更新 `CacheYggdrasilSession` 注释，注明 serverId 已通过 `max=256` binding tag 限制长度。

### 缺陷 M-3：UUID 命名空间选择依据注释

**文件**：`internal/logic/yggdrasil/signing.go:18-24`

扩展 `yggUserNamespaceUUID` 注释，说明使用 NameSpace_DNS 的原因和碰撞风险评估，并注明替代方案。

---

## 五、新建/修改方法清单

| 文件路径 | 操作 | 方法/常量 |
|----------|------|-----------|
| `internal/repository/game_profile_ygg.go` | 修改 | `unsignedUUIDToStandard` 增加 hex 校验 |
| `internal/repository/game_token.go` | 修改+新增 | `InvalidateByAccessToken` + `CountValidByUserID` |
| `internal/logic/yggdrasil/auth.go` | 修改 | `InvalidateToken` 重写 + `SignoutUser` 增加二次扫描 |
| `internal/logic/yggdrasil/session.go` | 修改 | `BuildProfileResponse` 失败早返回 + IP 标准化比较 |
| `internal/logic/yggdrasil/profile.go` | 修改 | 使用 `YggdrasilBatchLookupMaxNames` |
| `internal/constant/yggdrasil.go` | 修改 | 新增 `YggdrasilBatchLookupMaxNames` |
| `internal/handler/yggdrasil/server/server.go` | 修改 | serverId 长度校验 + 常量引用 |
| `internal/handler/yggdrasil/share/share.go` | 修改 | UploadTexture/DeleteTexture UUID 预检 |
| `internal/app/middleware/yggdrasil_auth.go` | 修改 | 设计决策注释 |
| `internal/entity/game_token.go` | 修改 | TempInvalid 状态用途注释 |
| `internal/app/startup/prepare/prepare_rsa.go` | 修改 | RSA 密钥长度决策注释 |
| `internal/constant/cache.go` | 修改 | Redis Key 安全注释 |
| `internal/logic/yggdrasil/signing.go` | 修改 | UUID 命名空间注释 |

**无新建文件。**

---

## 六、验证标准

- [x] `go build ./...` 编译通过（零错误零警告）
- [x] `unsignedUUIDToStandard` 含 `for _, c := range s { hex check }` 循环
- [x] 全局搜索 `InvalidateByAccessToken` 存在于 `game_token.go` 且被 `auth.go` 调用
- [x] `CountValidByUserID` 存在于 `game_token.go` 且被 `auth.go` SignoutUser 调用
- [x] `SignoutUser` 含二次扫描逻辑（CountValidByUserID + 条件性重新 InvalidateAllByUserID）
- [x] `YggdrasilBatchLookupMaxNames` 常量存在于 `yggdrasil.go` 且被 `profile.go` 和 `server.go` 引用
- [x] `server.go` HasJoined 含 `len(serverId) > 256` 长度校验
- [x] `BuildProfileResponse` Marshal 失败时返回不含 Properties 的精简 ProfileResponse
- [x] `share.go` UploadTexture/DeleteTexture 含 UUID 十六进制预检（len==32 + 逐字符 hex 校验）
- [x] `session.go` IP 比较使用 `net.ParseIP` + `.Equal()`
- [x] `TempInvalid` 注释包含"预留未来扩展"说明
- [x] `prepare_rsa.go` 含 RSA-2048 生态兼容性决策注释
- [x] 全局搜索 `YggdrasilTokenMaxPerUser` 在 `profile.go` 中结果为 0（已替换为独立常量）

---

## 七、与 Patch 007 的关系说明

| Patch 007 缺陷 | 本次处理 | 说明 |
|---------------|---------|------|
| C-1 JoinServer 重复验证 | — | Patch 007 已删除，本次未变更 |
| C-2 Refresh TOCTOU | — | Patch 007 已修复（RevokeValidOrTempInvalid），本次延伸至 InvalidateToken |
| I-1 BatchLookup 常量误用 | **本次修复** | 引入独立常量 `YggdrasilBatchLookupMaxNames` |
| I-3 中间件实例化开销 | **本次修复** | 文档化设计决策（保持现有实现） |
| I-4 HasJoined serverId | **本次修复** | 新增长度校验 |
| I-5 空 payload 签名 | **本次修复** | Marshal 失败时早返回精简响应 |
| I-6 UploadTexture UUID | **本次修复** | 新增十六进制预检 |
| I-7 IP 比较 | **本次修复** | 使用 net.ParseIP 标准化 |
| I-2 TempInvalid 状态 | **本次修复** | 补充预留用途注释 |
| M-1~M-4 安全加固 | **本次修复** | 补充决策依据注释 |
