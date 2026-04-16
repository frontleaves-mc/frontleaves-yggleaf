# User Response Extend（account_ready）设计方案

> **文档版本**: v1.0 | **创建日期**: 2026-04-16 | **作者**: 筱锋

---

## 一、背景与动机

### 1.1 当前问题

当前 `GET /user/info` 接口（`UserCurrent` handler）直接返回 `entity.User` 实体，存在以下问题：

1. **缺少账户完善度信息** — 前端无法判断用户是否已完成所有必要信息的填写
2. **Entity 直接暴露** — `GamePassword` 字段标记为 `json:"-"` 不参与序列化，但整体结构仍为原始 entity
3. **无扩展字段** — 无法在不修改 entity 的前提下附加业务计算字段

### 1.2 设计目标

- 构建**独立的 UserCurrentResponse DTO**，封装用户信息 + 扩展状态
- 新增 `extend.account_ready` 字段，用于标识用户账户的完善状态
- 当前阶段仅检查 `game_password` 是否已填写，后续可扩展更多检查项

---

## 二、核心设计

### 2.1 响应结构

```json
{
  "code": 0,
  "message": "获取用户信息成功",
  "data": {
    "user": { /* entity.User 序列化结果 */ },
    "extend": {
      "account_ready": "ready" | "game_password"
    }
  }
}
```

### 2.2 account_ready 语义

| 值 | 含义 | 触发条件 |
|----|------|---------|
| `"ready"` | 账户信息已全部完善 | 所有检查项均通过 |
| `"game_password"` | 游戏密码未填写 | `User.GamePassword` 为空字符串 |

### 2.3 判断逻辑

```go
// 判断 account_ready 的伪代码
func determineAccountReady(user *entity.User) string {
    if user.GamePassword == "" {
        return "game_password"  // 返回缺失的字段名，提示前端引导用户填写
    }
    return "ready"
}
```

**设计说明**：
- 返回缺失字段的名称（如 `"game_password"`）而非布尔值，便于前端**精确展示**需要补全的内容
- 当未来新增检查项时（如绑定手机、实名认证等），可直接扩展判断逻辑
- 多项缺失时可返回首个缺失项，或考虑改为数组形式（当前阶段仅单项检查）

---

## 三、DTO 设计

### 3.1 UserCurrentResponse

**文件**: `api/user/current.go`（新建）

```go
package user

import (
    "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
)

// UserExtend 用户信息扩展字段。
//
// 包含账户完善度等计算型数据，不存储于数据库，
// 由 Logic 层在响应构建时动态计算。
type UserExtend struct {
    // AccountReady 账户完善状态。
    //
    //   - "ready":          所有必要信息已填写完毕
    //   - "game_password":  游戏密码未设置（返回缺失字段名）
    AccountReady string `json:"account_ready"` // 账户完善状态
}

// UserCurrentResponse 用户当前信息响应 DTO。
//
// 将 entity.User 包装为嵌套结构，并附带 extend 扩展信息。
// 遵循项目 DTO 模式：Handler 不再直接返回 entity。
type UserCurrentResponse struct {
    User   entity.User  `json:"user"`   // 用户实体信息
    Extend UserExtend   `json:"extend"` // 扩展信息（含账户完善状态）
}
```

---

## 四、各层变更

### 4.1 API 层（新增文件）

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/user/current.go` | **新增** | 定义 `UserExtend` + `UserCurrentResponse` DTO |

### 4.2 Logic 层变更

**文件**: `internal/logic/user.go`

#### 4.2.1 新增方法

```go
// GetUserCurrent 获取当前用户的完整信息（含扩展状态）。
//
// 在 TakeUser 的基础上，额外计算账户完善度信息，
// 构建包含 extend 字段的 UserCurrentResponse DTO。
func (l *UserLogic) GetUserCurrent(ctx context.Context, userinfo *bSdkModels.OAuthUserinfo) (*apiUser.UserCurrentResponse, *xError.Error) {
    l.log.Info(ctx, "GetUserCurrent - 获取用户完整信息")

    // 复用现有 TakeUser 逻辑获取/创建用户
    user, xErr := this.TakeUser(ctx, userinfo)
    if xErr != nil {
        return nil, xErr
    }

    // 构建响应 DTO
    return &apiUser.UserCurrentResponse{
        User:   *user,
        Extend: apiUser.UserExtend{
            AccountReady: this.determineAccountReady(user),
        },
    }, nil
}

// determineAccountReady 根据用户实体判断账户完善状态。
//
// 当前检查项：game_password 是否已填写。
// 未来可在该方法中追加更多检查项。
func (_ *UserLogic) determineAccountReady(user *entity.User) string {
    if user.GamePassword == "" {
        return "game_password"
    }
    return "ready"
}
```

#### 4.2.2 方法签名对照

| 场景 | 变更前 | 变更后 |
|------|--------|--------|
| Handler 调用 | `TakeUser` → `*entity.User` | `GetUserCurrent` → `*UserCurrentResponse` |
| `TakeUser` | 保持不变（内部复用） | 保持不变 |

### 4.3 Handler 层变更

**文件**: `internal/handler/user.go`

```go
// 变更前
func (h *UserHandler) UserCurrent(ctx *gin.Context) {
    h.log.Info(ctx, "UserCurrent - 获取用户信息")

    userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
    if xErr != nil {
        _ = ctx.Error(xErr)
        return
    }
    getUser, xErr := h.service.userLogic.TakeUser(ctx.Request.Context(), userinfo)
    if xErr != nil {
        _ = ctx.Error(xErr)
        return
    }

    xResult.SuccessHasData(ctx, "测试", getUser)
}

// 变更后
func (h *UserHandler) UserCurrent(ctx *gin.Context) {
    h.log.Info(ctx, "UserCurrent - 获取用户信息")

    userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
    if xErr != nil {
        _ = ctx.Error(xErr)
        return
    }
    response, xErr := h.service.userLogic.GetUserCurrent(ctx.Request.Context(), userinfo)
    if xErr != nil {
        _ = ctx.Error(xErr)
        return
    }

    xResult.SuccessHasData(ctx, "获取用户信息成功", response)
}
```

### 4.4 Swagger 注解更新

```go
// @Success     200   {object}  xBase.BaseResponse{data=apiUser.UserCurrentResponse}	"获取成功"
```

---

## 五、数据流

```
┌─────────────┐     Authorization      ┌──────────────┐
│   Client    │ ──────────────────────> │  UserHandler │
│             │                         │  .UserCurrent│
└─────────────┘                         └──────┬───────┘
                                               │
                                               v
                                        ┌──────────────┐
                                        │  UserLogic    │
                                        │ .GetUserCurrent│
                                        └──────┬───────┘
                                               │
                                    ┌──────────┼──────────┐
                                    v                     v
                            ┌──────────────┐     ┌──────────────────┐
                            │  TakeUser()  │     │determineAccountReady│
                            │ (已有逻辑)    │     │ (新增：检查密码)    │
                            └──────┬───────┘     └────────┬─────────┘
                                   v                       v
                           ┌──────────────┐       ┌────────────┐
                           │ *entity.User │       │ "ready" /  │
                           │              │       │"game_password"│
                           └──────┬───────┘       └─────┬──────┘
                                  │                     │
                                  v                     v
                          ┌─────────────────────────────────┐
                          │   UserCurrentResponse           │
                          │  {                              │
                          │    user:   entity.User,         │
                          │    extend: { account_ready }    │
                          │  }                              │
                          └─────────────────┬───────────────┘
                                            │
                                            v
                                   JSON Response → Client
```

---

## 六、API 响应示例

### 6.1 密码未设置（需引导）

```json
{
  "code": 0,
  "message": "获取用户信息成功",
  "data": {
    "user": {
      "id": 7123456789012345,
      "username": "PlayerOne",
      "email": "player@example.com",
      "phone": null,
      "role_name": "player",
      "has_ban": false,
      "jailed_at": null
    },
    "extend": {
      "account_ready": "game_password"
    }
  }
}
```

### 6.2 账户已完善

```json
{
  "code": 0,
  "message": "获取用户信息成功",
  "data": {
    "user": {
      "id": 7123456789012345,
      "username": "PlayerOne",
      "email": "player@example.com",
      "phone": null,
      "role_name": "player",
      "has_ban": false,
      "jailed_at": null
    },
    "extend": {
      "account_ready": "ready"
    }
  }
}
```

---

## 七、扩展性设计

### 7.1 未来可追加的检查项

`determineAccountReady` 方法预留了扩展能力。当需要增加新的完善度检查时：

```go
// 未来扩展示例
func (_ *UserLogic) determineAccountReady(user *entity.User) string {
    // 检查 1：游戏密码
    if user.GamePassword == "" {
        return "game_password"
    }
    // 检查 2：手机号绑定（未来）
    // if user.Phone == nil || *user.Phone == "" {
    //     return "phone"
    // }
    // 检查 3：邮箱验证（未来）
    // ...

    return "ready"
}
```

### 7.2 多缺失项场景（可选升级）

若未来需要同时返回多项缺失信息，可将 `account_ready` 从 `string` 升级为 `[]string`：

```go
type UserExtend struct {
    AccountReady []string `json:"account_ready"` // 空数组 = ready
}
```

当前阶段保持 `string` 类型，满足最小需求。

---

## 八、文件变更清单

### 新增文件（1 个）

| 路径 | 用途 |
|------|------|
| `api/user/current.go` | `UserExtend` + `UserCurrentResponse` DTO 定义 |

### 修改文件（2 个）

| 路径 | 变更说明 |
|------|---------|
| `internal/logic/user.go` | 新增 `GetUserCurrent` + `determineAccountReady` 方法 |
| `internal/handler/user.go` | `UserCurrent` 改用 `GetUserCurrent` 并返回新 DTO |

---

## 九、实施阶段建议

| 阶段 | 内容 | 风险 |
|------|------|------|
| **Phase 1** | 新增 `api/user/current.go` DTO | 低 — 仅新增类型定义 |
| **Phase 2** | Logic 层新增方法 | 低 — 纯新增，不修改现有逻辑 |
| **Phase 3** | Handler 层适配 | 低 — 替换调用方法 + 更新 Swagger 注解 |
