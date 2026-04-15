# Patch 001: Yggdrasil Token 命名修正与登录凭证对齐

> 状态：待执行
> 创建日期：2026-04-15
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联进度：`dev/tracking/progress.md`、`dev/tracking/process.md`

---

## 一、缺陷背景

Phase 0 实施完成后经审查发现两处设计与现有系统不匹配的问题，需要修正。

---

## 二、缺陷 #1：Token 实体命名应改为 GameToken

### 问题描述

Yggdrasil 协议要求服务端自行管理 accessToken / clientToken 的完整生命周期（签发、验证、刷新、吊销、角色绑定），这确实需要一个持久化实体。但当前命名为 `Token` 与项目现有体系中的 OAuth2/SSO access token 概念混淆。

项目现有两套 token 概念：

```
OAuth2/SSO token:  外部 beacon-sso 签发 → AccessUserCache(Redis) 缓存 → middleware/user.go 验证
Yggdrasil token:   本服务自行签发 → 数据库持久化 → middleware/yggdrasil_auth.go 验证
```

### 改动要求

**重命名**：`Token` → `GameToken`，所有关联类型、变量、注释同步调整。

#### 涉及文件及具体改动

**1. `internal/entity/token.go` → 重命名为 `internal/entity/game_token.go`（文件名也改）**

```go
// 改前
type TokenStatus uint8
const ( TokenStatusValid ... )
type Token struct { ... }
func (_ *Token) GetGene() xSnowflake.Gene { return bConst.GeneForToken }

// 改后
type GameTokenStatus uint8
const ( GameTokenStatusValid ... )
type GameToken struct { ... }  // 注释中所有 "令牌" 改为 "游戏令牌"
func (_ *GameToken) GetGene() xSnowflake.Gene { return bConst.GeneForGameToken }
```

**2. `internal/constant/gene_number.go`**

```go
// 改前
GeneForToken xSnowflake.Gene = 40

// 改后
GeneForGameToken xSnowflake.Gene = 40 // Yggdrasil 游戏令牌
```

**3. `internal/app/startup/startup_database.go`**

```go
// 改前
&entity.Token{},

// 改后
&entity.GameToken{},
```

**4. `internal/repository/token.go` → 重命名为 `internal/repository/game_token.go`（文件名也改）**

```go
// 改前
type TokenRepo struct { ... }
func NewTokenRepo(db *gorm.DB) *TokenRepo { ... }
// 所有方法参数 *entity.Token → *entity.GameToken
// 所有 entity.TokenStatus → entity.GameTokenStatus

// 改后
type GameTokenRepo struct { ... }
func NewGameTokenRepo(db *gorm.DB) *GameTokenRepo { ... }
```

方法名改动清单：
| 改前 | 改后 |
|------|------|
| `TokenRepo` | `GameTokenRepo` |
| `NewTokenRepo` | `NewGameTokenRepo` |
| 所有 `entity.Token` | `entity.GameToken` |
| 所有 `entity.TokenStatusValid` | `entity.GameTokenStatusValid` |
| 所有 `entity.TokenStatusInvalid` | `entity.GameTokenStatusInvalid` |

**5. `internal/logic/yggdrasil/logic.go`**

```go
// 改前
tokenRepo *repository.TokenRepo
repository.NewTokenRepo(db)

// 改后
gameTokenRepo *repository.GameTokenRepo
repository.NewGameTokenRepo(db)
```

**6. `internal/logic/yggdrasil/auth.go`**

所有引用从 `Token` / `TokenRepo` / `entity.Token` / `entity.TokenStatus` 改为 `GameToken` / `GameTokenRepo` / `entity.GameToken` / `entity.GameTokenStatus`。

**7. `internal/app/middleware/yggdrasil_auth.go`**

```go
// 改前
token, found, xErr := yggLogic.ValidateToken(...)
if !found || token.Status != entity.TokenStatusValid { ... }
bConst.CtxYggdrasilToken

// 改后
gameToken, found, xErr := yggLogic.ValidateGameToken(...)
if !found || gameToken.Status != entity.GameTokenStatusValid { ... }
// context key 也建议改名：CtxYggdrasilToken → CtxYggdrasilGameToken
```

**8. `internal/constant/context.go`**

```go
// 改前
CtxYggdrasilToken xCtx.ContextKey = "yggdrasil_token"

// 改后
CtxYggdrasilGameToken xCtx.ContextKey = "yggdrasil_game_token"
```

**9. `dev/tracking/progress.md`**

任务描述中所有 "Token" 改为 "GameToken"。

---

## 三、缺陷 #2：登录凭证字段注释错误 + 缺少 UserRepo 查询方法

### 问题描述

Yggdrasil 的 `authenticate` 和 `signout` 接口中 `username` 字段映射到的应该是 `entity.User` 的 `Email` 或 `Phone`（不是角色名，也不是 `Username` 字段）。`password` 字段映射到 `entity.User.GamePassword`。

当前 `UserRepo` 只有按 SnowflakeID 查询的方法，缺少按 Email / Phone 查询的方法。

### 改动要求

#### 3.1 修正 DTO 注释

**`api/yggdrasil/request.go`**

```go
// 改前
Username string `json:"username" binding:"required"` // 邮箱或角色名
// 和
Username string `json:"username" binding:"required"` // 邮箱

// 改后
Username string `json:"username" binding:"required"` // 邮箱或手机号
// 和
Username string `json:"username" binding:"required"` // 邮箱或手机号
```

**`api/yggdrasil/response.go`** — 无需改动

#### 3.2 新增 UserRepo 查询方法

**`internal/repository/user.go`** 中新增：

```go
// GetByEmail 根据邮箱查询用户。
func (r *UserRepo) GetByEmail(ctx context.Context, tx *gorm.DB, email string) (*entity.User, bool, *xError.Error)

// GetByPhone 根据手机号查询用户。
func (r *UserRepo) GetByPhone(ctx context.Context, tx *gorm.DB, phone string) (*entity.User, bool, *xError.Error)
```

遵循现有模式：`pickDB(ctx, tx)` + `(result, found, error)` 三返回值。

#### 3.3 Yggdrasil 认证逻辑中查找用户的方法

**`internal/logic/yggdrasil/auth.go`** 中的 `AuthenticateUser` 实现时，查找用户流程：

```
1. 尝试 GetByEmail(username) → 找到 → 返回 User
2. 尝试 GetByPhone(username) → 找到 → 返回 User
3. 都未找到 → 返回 InvalidCredentials 错误
4. 找到 User 后 → bcrypt.Compare(User.GamePassword, password)
```

需要在 `yggdrasilRepo` 中新增 `userRepo *repository.UserRepo` 字段，或在 YggdrasilLogic 中直接注入。

#### 3.4 `dev/tracking/progress.md` 新增任务

在 Phase 0 或 Phase 2 前置中增加：
- 新增 `GetByEmail` / `GetByPhone` Repository 方法

---

## 四、执行顺序建议

1. 先执行缺陷 #2（新增 UserRepo 查询方法），因为无破坏性
2. 再执行缺陷 #1（Token → GameToken 重命名），需要全量替换

---

## 五、验证标准

- [ ] `go build ./...` 编译通过
- [ ] 全局搜索 `entity.Token` 结果为 0（全部改为 `entity.GameToken`）
- [ ] 全局搜索 `TokenRepo` 结果为 0（全部改为 `GameTokenRepo`）
- [ ] 全局搜索 `GeneForToken` 结果为 0（全部改为 `GeneForGameToken`）
- [ ] `api/yggdrasil/request.go` 中无「角色名」字样
- [ ] `UserRepo` 包含 `GetByEmail` 和 `GetByPhone` 方法
