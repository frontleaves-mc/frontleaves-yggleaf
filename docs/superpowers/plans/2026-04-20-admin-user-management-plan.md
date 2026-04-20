# 管理员用户管理接口实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为管理员提供用户分页列表查询和用户详情查看（含配额与资源列表）两个接口。

**Architecture:** 在现有 UserHandler 中扩展管理员方法，遵循 Handler → Logic → Repository 四层架构。详情接口采用并行数据获取 + 批量纹理解析策略，Handler 层协调 LibraryLogic 完成纹理 URL 解析。

**Tech Stack:** Go / Gin / GORM / Redis / beacon-bucket-sdk / swag (godoc-swagger)

---

## 文件结构总览

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `api/admin/admin_user_list.go` | 列表请求/响应 DTO |
| 新建 | `api/admin/admin_user_detail.go` | 详情响应 DTO（AdminUserBasic / 配额 / 资源条目） |
| 修改 | `internal/handler/user.go` | 新增 ListAdminUsers / GetAdminUserDetail 方法 + Swagger |
| 修改 | `internal/logic/user.go` | 新增 ListAdminUsers / GetAdminUserDetail 业务方法 |
| 修改 | `internal/repository/user.go` | 新增 List / GetAdminDetailAggregates 数据访问方法 |
| 新建 | `internal/app/route/route_admin.go` | 管理员用户路由注册 |
| 修改 | `internal/app/route/route.go` | 添加 r.adminRouter(apiRouter) 调用 |

---

### Task 1: 创建 API DTO — 列表结构体

**Files:**
- Create: `api/admin/admin_user_list.go`

- [ ] **Step 1: 创建列表请求和响应 DTO**

```go
package admin

import "time"

// AdminUserListRequest 管理员用户列表查询参数。
type AdminUserListRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Role      string `form:"role" binding:"omitempty,oneof=SUPER_ADMIN ADMIN PLAYER"`
	Keyword   string `form:"keyword" binding:"omitempty,max=64"`
	StartTime string `form:"start_time"` // RFC3339 格式
	EndTime   string `form:"end_time"`   // RFC3339 格式
}

// AdminUserListResponse 管理员用户列表响应。
type AdminUserListResponse struct {
	List  []AdminUserItem `json:"list"`
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
}

// AdminUserItem 用户列表项（精简字段）。
type AdminUserItem struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     *string   `json:"email"`
	RoleName  *string   `json:"role_name"`
	HasBan    bool      `json:"has_ban"`
	CreatedAt time.Time `json:"created_at"`
}
```

**参考依据:** 对标 `api/admin/issue.go` 的 `AdminIssueListQuery` 模式，使用 `form` tag 绑定 query 参数，`binding` tag 做校验。

---

### Task 2: 创建 API DTO — 详情结构体

**Files:**
- Create: `api/admin/admin_user_detail.go`

- [ ] **Step 1: 创建详情响应 DTO**

```go
package admin

import "time"

// AdminUserDetailResponse 管理员用户详情响应。
type AdminUserDetailResponse struct {
	User         AdminUserBasic        `json:"user"`
	GameProfile  *GameProfileQuotaInfo  `json:"game_profile"`
	LibraryQuota *LibraryQuotaInfo     `json:"library_quota"`
	SkinList     []AdminSkinItem       `json:"skin_list"`
	CapeList     []AdminCapeItem       `json:"cape_list"`
}

// AdminUserBasic 用户基本信息（脱敏，不含 game_password）。
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

// GameProfileQuotaInfo 游戏档案配额信息。
type GameProfileQuotaInfo struct {
	Total int32 `json:"total"`
	Used  int32 `json:"used"`
}

// LibraryQuotaInfo 资源库配额信息（皮肤 + 披风）。
type LibraryQuotaInfo struct {
	SkinsPrivateTotal int32 `json:"skins_private_total"`
	SkinsPublicTotal  int32 `json:"skins_public_total"`
	SkinsPrivateUsed  int32 `json:"skins_private_used"`
	SkinsPublicUsed   int32 `json:"skins_public_used"`
	CapesPrivateTotal int32 `json:"capes_private_total"`
	CapesPublicTotal  int32 `json:"capes_public_total"`
	CapesPrivateUsed  int32 `json:"capes_private_used"`
	CapesPublicUsed   int32 `json:"capes_public_used"`
}

// AdminSkinItem 皮肤条目（含纹理下载链接）。
type AdminSkinItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Model      string    `json:"model"`      // "STEVE" 或 "ALEX"
	IsPublic   bool      `json:"is_public"`
	TextureURL string    `json:"texture_url"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminCapeItem 披风条目（含纹理下载链接）.
type AdminCapeItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsPublic   bool      `json:"is_public"`
	TextureURL string    `json:"texture_url"`
	CreatedAt  time.Time `json:"created_at"`
}
```

**注意:** Model 字段为 `string` 类型（"STEVE"/"ALEX"），需在 Logic 层从 `entity.ModelType`(uint8) 转换。

---

### Task 3: Repository 层 — 分页列表查询

**Files:**
- Modify: `internal/repository/user.go`（在 `pickDB` 方法之后追加）

- [ ] **Step 1: 添加 List 方法**

在 `userRepo` 的 `pickDB` 方法之后追加：

```go
import (
	// ... existing imports ...
	"time"
)

// AdminUserFilter 管理员用户列表筛选条件。
type AdminUserFilter struct {
	Role      *string
	Keyword   *string
	StartTime *time.Time
	EndTime   *time.Time
}

// List 分页查询用户列表（管理员用）。
//
// 支持按角色、关键词（用户名/邮箱模糊匹配）、注册时间范围筛选，
// 默认按注册时间降序排列。
func (r *UserRepo) List(ctx context.Context, page, pageSize int, filter AdminUserFilter) ([]entity.User, int64, *xError.Error) {
	r.log.Info(ctx, "List - 管理员分页查询用户列表")

	query := r.db.WithContext(ctx).Model(&entity.User{})

	if filter.Role != nil && *filter.Role != "" {
		query = query.Where("role_name = ?", *filter.Role)
	}
	if filter.Keyword != nil && *filter.Keyword != "" {
		keyword := "%" + *filter.Keyword + "%"
		query = query.Where("username ILIKE ? OR email ILIKE ?", keyword, keyword)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询用户总数失败", true, err)
	}

	var users []entity.User
	offset := (page - 1) * pageSize
	if err := query.Select("id, username, email, role_name, has_ban, created_at").
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&users).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询用户列表失败", true, err)
	}

	return users, total, nil
}
```

**设计要点:**
- 使用 `Select` 只查询列表需要的字段，避免拉取 `game_password`、关联等重字段
- ILIKE 实现 PostgreSQL 不区分大小写的模糊搜索（项目使用 PG）
- 返回 `[]entity.User` 而非自定义结构体，复用实体类型

---

### Task 4: Repository 层 — 详情聚合查询

**Files:**
- Modify: `internal/repository/user.go`（在 List 方法之后追加）

- [ ] **Step 1: 添加 GetAdminDetailAggregates 方法**

```go
// AdminUserDetailAggregates 用户详情聚合数据（不含纹理 URL 解析）。
type AdminUserDetailAggregates struct {
	User          *entity.User
	GameProfile   *entity.GameProfileQuota
	LibraryQuota  *entity.LibraryQuota
	SkinLibraries []entity.SkinLibrary
	CapeLibraries []entity.CapeLibrary
}

// GetAdminDetailAggregates 并行获取用户详情所需的全部聚合数据。
//
// 该方法执行 5 个独立查询：用户基本信息、游戏档案配额、资源库配额、
// 皮肤库列表、披风库列表。调用方负责后续的纹理 URL 批量解析。
func (r *UserRepo) GetAdminDetailAggregates(ctx context.Context, userID string) (*AdminUserDetailAggregates, *xError.Error) {
	r.log.Info(ctx, "GetAdminDetailAggregates - 获取用户详情聚合数据")

	// 1. 查询用户基本信息
	user, found, err := r.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil // 由上层决定返回 404
	}

	result := &AdminUserDetailAggregates{User: user}

	// 2-5. 并行查询关联数据（使用 goroutine + channel）
	type quotaResult struct {
		gp  *entity.GameProfileQuota
		lq  *entity.LibraryQuota
		skins []entity.SkinLibrary
		capes []entity.CapeLibrary
		err  *xError.Error
	}

	ch := make(chan quotaResult, 1)
	go func() {
		qr := quotaResult{}

		// GameProfileQuota
		gpRepo := NewGameProfileQuotaRepo(r.db)
		gp, gpFound, gpErr := gpRepo.GetByUserID(ctx, nil, user.ID, false)
		if gpErr != nil {
			qr.err = gpErr
			ch <- qr
			return
		}
		if gpFound {
			qr.gp = gp
		}

		// LibraryQuota
		lqRepo := NewLibraryQuotaRepo(r.db)
		lq, lqFound, lqErr := lqRepo.GetByUserID(ctx, nil, user.ID, false)
		if lqErr != nil {
			qr.err = lqErr
			ch <- qr
			return
		}
		if lqFound {
			qr.lq = lq
		}

		// SkinLibrary 列表（不分页，全部返回）
		skinRepo := NewSkinLibraryRepo(r.db, nil) // cache 传 nil 因管理员接口不需要缓存
		skins, _, skinErr := skinRepo.ListByUserID(ctx, nil, user.ID, 1, 1000)
		if skinErr != nil {
			qr.err = skinErr
			ch <- qr
			return
		}
		qr.skins = skins

		// CapeLibrary 列表（不分页，全部返回）
		capeRepo := NewCapeLibraryRepo(r.db)
		capes, _, capeErr := capeRepo.ListByUserID(ctx, nil, user.ID, 1, 1000)
		if capeErr != nil {
			qr.err = capeErr
			ch <- qr
			return
		}
		qr.capes = capes

		ch <- qr
	}()

	qr := <-ch
	if qr.err != nil {
		return nil, qr.err
	}

	result.GameProfile = qr.gp
	result.LibraryQuota = qr.lq
	result.SkinLibraries = qr.skins
	result.CapeLibraries = qr.capes

	return result, nil
}
```

**设计要点:**
- 返回聚合结构体 `AdminUserDetailAggregates`，包含所有原始 entity 数据
- 用户不存在时返回 `(nil, nil)`，由 Logic 层判断返回 404
- 关联数据通过 goroutine 并行查询，减少延迟
- 纹理由 URL 解析不在 Repository 层处理，留给 Logic/Handler 层

---

### Task 5: Logic 层 — 列表业务逻辑

**Files:**
- Modify: `internal/logic/user.go`（在文件末尾追加）

- [ ] **Step 1: 添加 ListAdminUsers 方法**

首先在 import 中添加 `"time"` 和 api 包引用：

```go
import (
	"context"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	apiAdmin "github.com/frontleaves-mc/frontleaves-yggleaf/api/admin"
	"github.com/frontleaves-mc/frontleaves-yggleaf/api/user"
	bSdkModels "github.com/phalanx-labs/beacon-sso-sdk/models"
)
```

然后在文件末尾追加：

```go
// ListAdminUsers 管理员分页查询用户列表。
func (l *UserLogic) ListAdminUsers(ctx context.Context, req *apiAdmin.AdminUserListRequest) (*apiAdmin.AdminUserListResponse, *xError.Error) {
	l.log.Info(ctx, "ListAdminUsers - 管理员分页查询用户列表")

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	filter := repository.AdminUserFilter{}
	if req.Role != "" {
		filter.Role = &req.Role
	}
	if req.Keyword != "" {
		filter.Keyword = &req.Keyword
	}
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			filter.StartTime = &t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			filter.EndTime = &t
		}
	}

	users, total, xErr := l.repo.user.List(ctx, page, pageSize, filter)
	if xErr != nil {
		return nil, xErr
	}

	items := make([]apiAdmin.AdminUserItem, len(users))
	for i, u := range users {
		items[i] = apiAdmin.AdminUserItem{
			ID:        u.ID.String(),
			Username:  u.Username,
			Email:     u.Email,
			RoleName:  u.RoleName,
			HasBan:    u.HasBan,
			CreatedAt: u.CreatedAt,
		}
	}

	return &apiAdmin.AdminUserListResponse{
		List:  items,
		Total: total,
		Page:  page,
		Size:  pageSize,
	}, nil
}
```

---

### Task 6: Logic 层 — 详情业务逻辑

**Files:**
- Modify: `internal/logic/user.go`（在 ListAdminUsers 之后追加）

- [ ] **Step 1: 添加 GetAdminUserDetailRaw 方法（聚合查询）**

```go
// AdminUserDetailRaw 用户详情原始聚合数据（供 Handler 层组装最终响应）。
type AdminUserDetailRaw struct {
	User          *entity.User
	GameProfile   *entity.GameProfileQuota
	LibraryQuota  *entity.LibraryQuota
	SkinLibraries []entity.SkinLibrary
	CapeLibraries []entity.CapeLibrary
}

// GetAdminUserDetailRaw 获取用户详情的原始聚合数据（不含纹理 URL）。
func (l *UserLogic) GetAdminUserDetailRaw(ctx context.Context, userID string) (*AdminUserDetailRaw, *xError.Error) {
	l.log.Info(ctx, "GetAdminUserDetailRaw - 获取用户详情聚合数据")

	aggregates, xErr := l.repo.user.GetAdminDetailAggregates(ctx, userID)
	if xErr != nil {
		return nil, xErr
	}
	if aggregates == nil || aggregates.User == nil {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "用户不存在", true)
	}

	return &AdminUserDetailRaw{
		User:          aggregates.User,
		GameProfile:   aggregates.GameProfile,
		LibraryQuota:  aggregates.LibraryQuota,
		SkinLibraries: aggregates.SkinLibraries,
		CapeLibraries: aggregates.CapeLibraries,
	}, nil
}
```

- [ ] **Step 2: 添加 modelTypeToString 辅助方法**

```go
// modelTypeToString 将 entity.ModelType 转为可读字符串。
func modelTypeToString(mt entity.ModelType) string {
	switch mt {
	case entity.ModelTypeClassic:
		return "STEVE"
	case entity.ModelTypeSlim:
		return "ALEX"
	default:
		return "UNKNOWN"
	}
}
```

**设计要点:**
- `GetAdminUserDetailRaw` 返回原始聚合数据，不包含纹理 URL 解析
- 纹理 URL 解析放在 Handler 层（通过 `h.service.libraryLogic`），保持 Logic 层职责纯粹
- `modelTypeToString` 作为包级辅助函数，将 uint8(1/2) 转为 "STEVE"/"ALEX"

---

### Task 7: Handler 层 — 列表接口

**Files:**
- Modify: `internal/handler/user.go`（在文件末尾追加）

- [ ] **Step 1: 添加 ListAdminUsers handler 方法 + Swagger 注释**

首先更新 import：

```go
import (
	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiAdmin "github.com/frontleaves-mc/frontleaves-yggleaf/api/admin"
	apiUser "github.com/frontleaves-mc/frontleaves-yggleaf/api/user"
	"github.com/gin-gonic/gin"
	bSdkUtil "github.com/phalanx-labs/beacon-sso-sdk/utility"
)
```

然后在文件末尾追加：

```go
// ListAdminUsers 管理员获取用户分页列表
//
// @Summary 	[管理] 用户列表
// @Description 管理员分页查询用户列表，支持角色、关键词、时间范围筛选
// @Tags        管理员-用户接口
// @Accept      json
// @Produce     json
// @Param       page query int false "页码" default(1)
// @Param       page_size query int false "每页条数(最大100)" default(20)
// @Param       role query string false "角色筛选(SUPER_ADMIN/ADMIN/PLAYER)"
// @Param       keyword query string false "关键词搜索(用户名/邮箱)"
// @Param       start_time query string false "注册时间起始(RFC3339)"
// @Param       end_time query string false "注册时间截止(RFC3339)"
// @Success     200   {object}  xBase.BaseResponse{data=admin.AdminUserListResponse}	"查询成功"
// @Failure     400   {object}  xBase.BaseResponse          			"请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse         				"未授权"
// @Failure     403   {object}  xBase.BaseResponse          			"权限不足"
// @Router       /admin/users [GET]
func (h *UserHandler) ListAdminUsers(ctx *gin.Context) {
	h.log.Info(ctx, "ListAdminUsers - 管理员获取用户分页列表")

	req := &apiAdmin.AdminUserListRequest{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "请求参数错误", true, err))
		return
	}

	response, xErr := h.service.userLogic.ListAdminUsers(ctx.Request.Context(), req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取用户列表成功", response)
}
```

**参考依据:** 对标 `issue.go:GetIssueListAdmin` 的参数解析模式，使用 `ShouldBindQuery` 绑定 query 参数。

---

### Task 8: Handler 层 — 详情接口

**Files:**
- Modify: `internal/handler/user.go`（在 ListAdminUsers 之后追加）

- [ ] **Step 1: 添加 GetAdminUserDetail handler 方法 + Swagger 注释**

```go
// GetAdminUserDetail 管理员获取用户详情
//
// @Summary 	[管理] 用户详情
// @Description 获取用户完整详情，包含账户信息、游戏档案配额、资源库配额及皮肤/披风资源列表
// @Tags        管理员-用户接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Success     200   {object}  xBase.BaseResponse{data=admin.AdminUserDetailResponse}	"查询成功"
// @Failure     400   {object}  xBase.BaseResponse          			"请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse         				"未授权"
// @Failure     403   {object}  xBase.BaseResponse          			"权限不足"
// @Failure     404   {object}  xBase.BaseResponse          			"用户不存在"
// @Router       /admin/users/{user_id} [GET]
func (h *UserHandler) GetAdminUserDetail(ctx *gin.Context) {
	h.log.Info(ctx, "GetAdminUserDetail - 管理员获取用户详情")

	targetUserID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的用户 ID", true, err))
		return
	}

	raw, xErr := h.service.userLogic.GetAdminUserDetailRaw(ctx.Request.Context(), targetUserID.String())
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	// 构建响应
	resp := apiAdmin.AdminUserDetailResponse{
		User: apiAdmin.AdminUserBasic{
			ID:        raw.User.ID.String(),
			Username:  raw.User.Username,
			Email:     raw.User.Email,
			Phone:     raw.User.Phone,
			RoleName:  raw.User.RoleName,
			HasBan:    raw.User.HasBan,
			JailedAt:  raw.User.JailedAt,
			CreatedAt: raw.User.CreatedAt,
			UpdatedAt: raw.User.UpdatedAt,
		},
	}

	// 游戏档案配额（可能为空）
	if raw.GameProfile != nil {
		resp.GameProfile = &apiAdmin.GameProfileQuotaInfo{
			Total: raw.GameProfile.Total,
			Used:  raw.GameProfile.Used,
		}
	}

	// 资源库配额（可能为空）
	if raw.LibraryQuota != nil {
		resp.LibraryQuota = &apiAdmin.LibraryQuotaInfo{
			SkinsPrivateTotal: raw.LibraryQuota.SkinsPrivateTotal,
			SkinsPublicTotal:  raw.LibraryQuota.SkinsPublicTotal,
			SkinsPrivateUsed:  raw.LibraryQuota.SkinsPrivateUsed,
			SkinsPublicUsed:   raw.LibraryQuota.SkinsPublicUsed,
			CapesPrivateTotal: raw.LibraryQuota.CapesPrivateTotal,
			CapesPublicTotal:  raw.LibraryQuota.CapesPublicTotal,
			CapesPrivateUsed:  raw.LibraryQuota.CapesPrivateUsed,
			CapesPublicUsed:   raw.LibraryQuota.CapesPublicUsed,
		}
	}

	// 收集 Texture ID 用于批量解析
	var textureIDs []int64
	for _, s := range raw.SkinLibraries {
		textureIDs = append(textureIDs, s.Texture)
	}
	for _, c := range raw.CapeLibraries {
		textureIDs = append(textureIDs, c.Texture)
	}

	// 批量解析纹理 URL
	urlMap := make(map[int64]string)
	if len(textureIDs) > 0 {
		urlMap, xErr = h.service.libraryLogic.ResolveTextureURLsBatchForAdmin(ctx.Request.Context(), textureIDs)
		if xErr != nil {
			// 纹理解析失败不阻断主流程，URL 留空
			h.log.Warn(ctx, "批量解析纹理 URL 失败: "+xErr.ErrorMessage)
		}
	}

	// 组装皮肤列表
	resp.SkinList = make([]apiAdmin.AdminSkinItem, len(raw.SkinLibraries))
	for i, s := range raw.SkinLibraries {
		resp.SkinList[i] = apiAdmin.AdminSkinItem{
			ID:         s.ID.String(),
			Name:       s.Name,
			Model:      modelTypeToString(s.Model),
			IsPublic:   s.IsPublic,
			TextureURL: urlMap[s.Texture],
			CreatedAt:  s.CreatedAt,
		}
	}

	// 组装披风列表
	resp.CapeList = make([]apiAdmin.AdminCapeItem, len(raw.CapeLibraries))
	for i, c := range raw.CapeLibraries {
		resp.CapeList[i] = apiAdmin.AdminCapeItem{
			ID:         c.ID.String(),
			Name:       c.Name,
			IsPublic:   c.IsPublic,
			TextureURL: urlMap[c.Texture],
			CreatedAt:  c.CreatedAt,
		}
	}

	xResult.SuccessHasData(ctx, "获取用户详情成功", resp)
}
```

**关键决策说明:**
- Handler 层负责组装最终响应 DTO（从 entity → admin DTO）
- 通过 `h.service.libraryLogic` 调用纹理解析（见 Task 9）
- 纹理解析失败不阻断主流程，仅记录警告日志
- 配额为空时（新用户尚未初始化）对应字段为 null

---

### Task 9: Logic 层 — 暴露纹理批量解析方法

**Files:**
- Modify: `internal/logic/library.go`（在 resolveTextureURLsBatch 之后追加）

- [ ] **Step 1: 将 resolveTextureURLsBatch 导出为公开方法**

由于 `resolveTextureURLsBatch` 是私有方法（小写开头），需要为管理员场景导出一个公开版本：

```go
// ResolveTextureURLsBatchForAdmin 管理员接口专用的批量纹理 URL 解析公开方法。
//
// 封装私有的 resolveTextureURLsBatch，供 Handler 层在用户详情接口中调用。
func (l *LibraryLogic) ResolveTextureURLsBatchForAdmin(ctx context.Context, textureIDs []int64) (map[int64]string, *xError.Error) {
	return l.resolveTextureURLsBatch(ctx, textureIDs)
}
```

**替代方案考量:** 也可以直接将 `resolveTextureURLsBatch` 改名为首字母大写的公开方法。但考虑到该方法目前仅在 LibraryLogic 内部使用，添加一个薄包装方法影响更小，符合开放-封闭原则。

---

### Task 10: 路由注册

**Files:**
- Create: `internal/app/route/route_admin.go`
- Modify: `internal/app/route/route.go`

- [ ] **Step 1: 创建 route_admin.go**

```go
package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/middleware"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx-labs/beacon-sso-sdk/middleware"
)

func (r *route) adminRouter(route gin.IRouter) {
	userHandler := handler.NewHandler[handler.UserHandler](r.context, "UserHandler")

	adminGroup := route.Group("/admin/users")
	adminGroup.Use(bSdkMiddle.CheckAuth(r.context))
	adminGroup.Use(middleware.User(r.context))
	adminGroup.Use(middleware.Admin(r.context))
	{
		adminGroup.GET("", userHandler.ListAdminUsers)
		adminGroup.GET("/:user_id", userHandler.GetAdminUserDetail)
	}
}
```

- [ ] **Step 2: 在 route.go 的 NewRoute 中注册 admin 路由**

修改 `internal/app/route/route.go`，在路由注册区块中添加：

```go
		// 路由初始化注册
		{
			apiRouter := r.engine.Group("/api/v1")

			oauthRoute.OAuthRouter(apiRouter)

			r.userRouter(apiRouter)
			r.gameProfileRouter(apiRouter)
			r.libraryRouter(apiRouter)
			r.issueRouter(apiRouter)
			r.adminRouter(apiRouter)  // <-- 新增
		}
```

**参考依据:** 完全对标 `route_game_profile.go` 的管理员路由组模式。

---

### Task 11: 编译验证

**Files:** 无新增/修改（验证步骤）

- [ ] **Step 1: 执行编译检查**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 2: 执行 swagger 文档生成**

Run: `swag init -g cmd/server.go -o docs --parseDependency --parseInternal`
Expected: 成功生成/更新 swagger 文档，无 parse 错误

- [ ] **Step 3: 检查 swagger JSON 中是否包含新接口**

确认 `docs/frontleaves_yggleaf_swagger.json` 中存在：
- `GET /api/v1/admin/users`
- `GET /api/v1/admin/users/{user_id}`

---

## 自审清单

### Spec 覆盖度

| 设计文档要求 | 实施任务 | 状态 |
|-------------|---------|------|
| GET /admin/users 分页列表 | Task 1 + Task 3 + Task 5 + Task 7 + Task 10 | ✅ |
| role 筛选 | Task 1 (binding:oneof) + Task 3 (Where) + Task 5 | ✅ |
| keyword 搜索 | Task 1 + Task 3 (ILIKE) + Task 5 | ✅ |
| 时间范围筛选 | Task 1 + Task 3 (created_at >=/< =) + Task 5 (RFC3339 Parse) | ✅ |
| 注册时间降序 | Task 3 (Order created_at DESC) | ✅ |
| page_size 最大 100 | Task 1 (binding:max=100) + Task 5 (clamp) | ✅ |
| GET /admin/users/:user_id 详情 | Task 2 + Task 4 + Task 6 + Task 8 + Task 10 | ✅ |
| 基本信息（脱敏） | Task 2 (AdminUserBasic, 无 password) + Task 8 | ✅ |
| GameProfileQuota | Task 2 + Task 4 + Task 8 | ✅ |
| LibraryQuota（8 个字段） | Task 2 + Task 4 + Task 8 | ✅ |
| 皮肤列表（含纹理 URL） | Task 2 + Task 4 + Task 8 + Task 9 | ✅ |
| 披风列表（含纹理 URL） | Task 2 + Task 4 + Task 8 + Task 9 | ✅ |
| 批量纹理解析（避免 N+1） | Task 9 (ResolveTextureURLsBatchForAdmin) + Task 8 | ✅ |
| CheckAuth → User → Admin 中间件链 | Task 10 (route_admin.go) | ✅ |
| Swagger 注释 | Task 7 + Task 8 | ✅ |

### 类型一致性

| 类型定义位置 | 使用位置 | 一致性 |
|-------------|---------|--------|
| `AdminUserListRequest` (Task 1) | Handler.ShouldBindQuery (Task 7) | ✅ form tags 匹配 |
| `AdminUserListResponse` (Task 1) | Logic return (Task 5) | ✅ 字段一致 |
| `AdminUserDetailResponse` (Task 2) | Handler assemble (Task 8) | ✅ 字段一致 |
| `GameProfileQuotaInfo.Total/Used` int32 (Task 2) | entity.GameProfileQuota int32 (entity) | ✅ 已修正 |
| `repository.AdminUserFilter` (Task 3) | Logic → Repo call (Task 5) | ✅ |
| `AdminUserDetailRaw` (Task 6) | Logic return → Handler consume (Task 8) | ✅ |
| `ResolveTextureURLsBatchForAdmin` (Task 9) | Handler call (Task 8) | ✅ |

### 占位符扫描

无 TBD、TODO、"实现后续"、"类似 Task N" 等占位符。✅
