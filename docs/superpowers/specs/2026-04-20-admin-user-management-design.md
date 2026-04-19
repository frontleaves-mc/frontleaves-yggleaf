# 管理员用户管理接口设计文档

> 日期: 2026-04-20
> 状态: 已批准
> 相关: [管理员配额调整设计](./2026-04-19-admin-game-profile-quota-design.md)

## 概述

为管理员提供用户管理能力，包含两个核心接口：
1. **用户分页列表** — 支持多条件筛选和搜索的用户列表查询
2. **用户详情** — 聚合展示用户完整信息，含账户配额、资源库配额及皮肤/披风资源列表

## 接口设计

### 1. 用户分页列表

**端点:** `GET /admin/users`

**中间件链:** `CheckAuth` → `User` → `Admin`

#### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| page | int | 否 | 1 | 页码 |
| page_size | int | 否 | 20 | 每页条数（最大 100） |
| role | string | 否 | - | 角色筛选: `SUPER_ADMIN` / `ADMIN` / `PLAYER` |
| keyword | string | 否 | - | 关键词搜索（匹配用户名/邮箱） |
| start_time | string | 否 | - | 注册时间起始 (RFC3339) |
| end_time | string | 否 | - | 注册时间截止 (RFC3339) |

#### 响应结构

```go
// AdminUserListResponse 管理员用户列表响应
type AdminUserListResponse struct {
    List  []AdminUserItem `json:"list"`   // 用户列表
    Total int64          `json:"total"`   // 总记录数
    Page  int            `json:"page"`    // 当前页码
    Size  int            `json:"size"`    // 每页大小
}

// AdminUserItem 列表项（精简信息）
type AdminUserItem struct {
    ID        string    `json:"id"`         // 用户 ID
    Username  string    `json:"username"`    // 用户名
    Email     *string   `json:"email"`       // 邮箱
    RoleName  *string   `json:"role_name"`   // 角色
    HasBan    bool      `json:"has_ban"`     // 是否封禁
    CreatedAt time.Time `json:"created_at"`  // 注册时间
}
```

#### 查询逻辑

```sql
SELECT id, username, email, role_name, has_ban, created_at
FROM users
WHERE 1=1
  AND (:role = '' OR role_name = :role)
  AND (:keyword = '' OR username ILIKE '%' || :keyword || '%' OR email ILIKE '%' || :keyword || '%')
  AND (:start_time = '' OR created_at >= :start_time)
  AND (:end_time = '' OR created_at <= :end_time)
ORDER BY created_at DESC
LIMIT :page_size OFFSET :offset
```

---

### 2. 用户详情

**端点:** `GET /admin/users/:user_id`

**中间件链:** `CheckAuth` → `User` → `Admin`

#### 路径参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_id | string | 是 | 目标用户 ID（Snowflake ID） |

#### 响应结构

```go
// AdminUserDetailResponse 管理员用户详情响应
type AdminUserDetailResponse struct {
    User         AdminUserBasic        `json:"user"`           // 基本信息
    GameProfile  *GameProfileQuotaInfo  `json:"game_profile"`   // 游戏档案配额
    LibraryQuota *LibraryQuotaInfo     `json:"library_quota"`  // 资源库配额
    SkinList     []AdminSkinItem       `json:"skin_list"`      // 皮肤列表
    CapeList     []AdminCapeItem       `json:"cape_list"`      // 披风列表
}

// AdminUserBasic 用户基本信息（脱敏）
type AdminUserBasic struct {
    ID        string     `json:"id"`
    Username  string     `json:"username"`
    Email     *string    `json:"email"`
    Phone     *string    `json:"phone"`
    RoleName  *string    `json:"role_name"`
    HasBan    bool       `json:"has_ban"`
    JailedAt  *time.Time `json:"jailed_at"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}

// GameProfileQuotaInfo 游戏档案配额信息
type GameProfileQuotaInfo struct {
    Total int64 `json:"total"` // 总额度
    Used  int64 `json:"used"`  // 已使用额度
}

// LibraryQuotaInfo 资源库配额信息（皮肤 + 披风）
type LibraryQuotaInfo struct {
    SkinsPrivateTotal int32 `json:"skins_private_total"` // 私有皮肤总额度
    SkinsPublicTotal  int32 `json:"skins_public_total"`  // 公开皮肤总额度
    SkinsPrivateUsed  int32 `json:"skins_private_used"`  // 私有皮肤已用
    SkinsPublicUsed   int32 `json:"skins_public_used"`   // 公开皮肤已用
    CapesPrivateTotal int32 `json:"capes_private_total"` // 私有披风总额度
    CapesPublicTotal  int32 `json:"capes_public_total"`  // 公开披风总额度
    CapesPrivateUsed  int32 `json:"capes_private_used"`  // 私有披风已用
    CapesPublicUsed   int32 `json:"capes_public_used"`   // 公开披风已用
}

// AdminSkinItem 皮肤条目（含纹理 URL）
type AdminSkinItem struct {
    ID         string    `json:"id"`          // 皮肤 ID
    Name       string    `json:"name"`        // 皮肤名称
    Model      string    `json:"model"`       // 模型类型 (STEVE/ALEX)
    IsPublic   bool      `json:"is_public"`   // 是否公开
    TextureURL string    `json:"texture_url"` // 纹理下载链接
    CreatedAt  time.Time `json:"created_at"`  // 创建时间
}

// AdminCapeItem 披风条目（含纹理 URL）
type AdminCapeItem struct {
    ID         string    `json:"id"`          // 披风 ID
    Name       string    `json:"name"`        // 披风名称
    IsPublic   bool      `json:"is_public"`   // 是否公开
    TextureURL string    `json:"texture_url"` // 纹理下载链接
    CreatedAt  time.Time `json:"created_at"`  // 创建时间
}
```

#### 数据获取策略

```
                    ┌──────────────────────┐
                    │   Handler 层         │
                    │ GetAdminUserDetail() │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │    Logic 层          │
                    │ GetAdminUserDetail() │
                    │                      │
                    │  1. 并行获取:         │
                    │   ├─ User 基础信息    │
                    │   ├─ GameProfileQuota │
                    │   ├─ LibraryQuota     │
                    │   ├─ SkinLibrary 列表 │
                    │   └─ CapeLibrary 列表 │
                    │                      │
                    │  2. 批量解析纹理 URL   │
                    │   resolveTextureURLs  │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │   Repository 层      │
                    │ 各实体独立查询方法     │
                    └──────────────────────┘
```

**并行获取**: 使用 `errgroup` 或 `sync.WaitGroup` 并行查询 5 个数据源，减少延迟。

**批量纹理解析**: 收集所有 Skin/Cape 的 Texture ID，调用 `resolveTextureURLsBatch` 一次性获取所有下载链接。

---

## 分层职责

### Handler 层 (`internal/handler/user.go` — 新增方法)

| 方法 | 职责 |
|------|------|
| `ListAdminUsers()` | 绑定查询参数，调用 Logic 分页查询 |
| `GetAdminUserDetail()` | 解析路径参数 user_id，调用 Logic 聚合查询 |

### Logic 层 (`internal/logic/user.go` — 新增方法)

| 方法 | 职责 |
|------|------|
| `ListAdminUsers(ctx, req)` | 构建动态查询条件，执行分页查询 |
| `GetAdminUserDetail(ctx, userID)` | 并行聚合用户+配额+资源数据，批量解析纹理 URL |

### Repository 层 (`internal/repository/user.go` — 新增方法)

| 方法 | 职责 |
|------|------|
| `List(ctx, filters)` | 动态条件分页查询用户 |
| `GetWithQuotasAndResources(ctx, userID)` | 获取用户详情聚合数据 |

### API DTO 层 (`api/admin/`)

| 文件 | 内容 |
|------|------|
| `admin_user_list.go` | 列表请求/响应结构体 |
| `admin_user_detail.go` | 详情响应结构体 |

---

## 路由注册

**文件:** `internal/app/route/route_admin.go`（新建）

```go
func (r *route) adminRouter(route gin.IRouter) {
    userHandler := handler.NewHandler[handler.UserHandler](r.context, "UserHandler")

    adminGroup := route.Group("/admin/users")
    adminGroup.Use(bSdkMiddle.CheckAuth(r.context))
    adminGroup.Use(middleware.User(r.context))
    adminGroup.Use(middleware.Admin(r.context))
    {
        adminGroup.GET("", userHandler.ListAdminUsers)         // 用户列表
        adminGroup.GET("/:user_id", userHandler.GetAdminUserDetail) // 用户详情
    }
}
```

在 `route.go` 的 `NewRoute` 中添加 `r.adminRouter(apiRouter)` 调用。

---

## 错误处理

| 场景 | 错误码 | HTTP 状态码 | 说明 |
|------|--------|-------------|------|
| user_id 格式错误 | ParameterError | 400 | 无效的 Snowflake ID |
| 用户不存在 | ResourceNotFound | 404 | 目标用户未找到 |
| 权限不足 | PermissionDenied | 403 | 非 SUPER_ADMIN/ADMIN 角色（中间件拦截） |
| 未授权 | Unauthorized | 401 | 未登录或 Token 过期（中间件拦截） |

---

## 安全考虑

1. **密码脱敏**: User 实体的 `GamePassword` 字段已标记 `json:"-"`，不会序列化返回
2. **权限控制**: 所有接口必须通过 Admin 中间件校验
3. **分页限制**: page_size 最大 100，防止大量数据查询
4. **SQL 注入防护**: 使用 GORM 参数化查询，避免拼接 SQL

---

## 文件变更清单

| 操作 | 文件路径 | 说明 |
|------|----------|------|
| 新建 | `api/admin/admin_user_list.go` | 列表 DTO |
| 新建 | `api/admin/admin_user_detail.go` | 详情 DTO |
| 修改 | `internal/handler/user.go` | 新增 ListAdminUsers / GetAdminUserDetail |
| 修改 | `internal/logic/user.go` | 新增对应业务方法 |
| 修改 | `internal/repository/user.go` | 新增分页查询和聚合查询方法 |
| 新建 | `internal/app/route/route_admin.go` | 管理员路由注册 |
| 修改 | `internal/app/route/route.go` | 注册 admin 路由 |
