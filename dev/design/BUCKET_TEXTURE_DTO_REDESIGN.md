# Beacon Bucket Texture URL 解析与 DTO 重构设计方案

> **文档版本**: v1.0 | **创建日期**: 2026-04-15 | **作者**: 筱锋

---

## 一、背景与动机

### 1.1 当前架构问题

当前 Library 系统中，皮肤/披风纹理文件通过 `beacon-bucket-sdk` 上传至对象存储服务后，SDK 返回一个 `FileId`（字符串形式的雪花 ID），该 ID 经 `strconv.ParseInt` 转换为 `int64` 后存入 `SkinLibrary.Texture` / `CapeLibrary.Texture` 字段。

在数据读取阶段（列表、详情、装备等接口），entity 直接通过 Go struct embedding 暴露给前端序列化，导致前端拿到的 `texture` 字段是一个无意义的雪花 ID 数字（如 `360607182437229568`），而非实际可用的纹理下载链接。

**具体问题**：

1. **前端无法直接使用 Texture ID** — 前端需要自行拼接或额外请求来获取纹理文件 URL
2. **Entity 直接暴露内部实现** — `Texture int64` 是存储层的内部标识，不应泄漏到 API 响应
3. **Response DTO 缺乏转换层** — 当前 `SkinResponse` / `CapeResponse` 通过 embedding 直接复用 entity 的 json 序列化，无法对字段做任何变换
4. **GameProfile 同样受影响** — 详情/列表/装备接口通过 GORM Preload 嵌套的 `SkinLibrary` / `CapeLibrary` 也携带了原始 Texture ID

### 1.2 设计目标

- 构建**独立的 Response DTO**，不再通过 Go embedding 直接复用 entity
- Logic 层在返回数据前调用 `bucket.Normal.Get()` 解析 Texture ID → 下载链接
- `SkinResponse` / `CapeResponse` / `GameProfile` 响应中 `texture` 字段从 `int64` 变更为 `string` 类型的下载链接
- 统一处理所有受影响的接口（Library + GameProfile）

### 1.3 SDK Get 方法

`beacon-bucket-sdk` 的 `INormalUpload` 接口提供了 `Get` 方法：

```go
Get(ctx context.Context, req *api.GetRequest) (*api.GetResponse, error)
```

| 请求字段 | 类型 | 说明 |
|---------|------|------|
| `FileId` | string | 必填，要查询的文件 ID |

| 响应字段 | 类型 | 说明 |
|---------|------|------|
| `Obj.Link` | string | 文件下载链接 |
| `Obj.ObjectKey` | string | 对象存储键 |
| `Obj.FullKey` | string | 完整对象键 |
| `FileName` | string | 文件名 |
| `Size` | int64 | 文件大小 |
| `MimeType` | string | MIME 类型 |

本次设计仅需使用 `Obj.Link` 字段。

---

## 二、核心变更概览

### 2.1 变更前后对比

| 层级 | 变更前 | 变更后 |
|------|--------|--------|
| **Entity** | 不变 | 不变（`Texture int64` 仍为存储层标识） |
| **Response DTO** | 嵌入 entity（零拷贝） | **显式字段定义**，`texture` → `texture_url` (string) |
| **Logic 返回值** | `*entity.SkinLibrary` | `*apiLibrary.SkinResponse` |
| **Logic 行为** | 获取 entity → 直接返回 | 获取 entity → 调用 bucket.Get → 构建 DTO → 返回 |
| **Handler 行为** | 转换 entity → Response | 直接返回 Logic 的 DTO |

### 2.2 数据流变更

```
变更前：
  DB Entity(Texture: int64) ──直接序列化──> JSON {"texture": 360607182437229568}

变更后：
  DB Entity(Texture: int64) ──bucket.Get(FileId)──> GetResponse.Obj.Link
                            ──构建 DTO────────────> JSON {"texture_url": "https://..."}
```

---

## 三、DTO 设计

### 3.1 SkinResponse 重构

**文件**: `api/library/skin.go`

```go
package library

import (
    xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
    "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
    entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
)

// CreateSkinRequest 创建皮肤请求
type CreateSkinRequest struct {
    Name     string `json:"name" binding:"required"`            // 皮肤名称
    Model    uint8  `json:"model" binding:"required,oneof=1 2"` // 皮肤模型 (1=classic, 2=slim)
    Texture  string `json:"texture" binding:"required"`         // 皮肤纹理文件 base64
    IsPublic *bool  `json:"is_public,omitempty"`                // 是否公开（可选，默认 false）
}

// UpdateSkinRequest 更新皮肤请求
type UpdateSkinRequest struct {
    Name     *string `json:"name,omitempty"`      // 皮肤名称（可选）
    IsPublic *bool   `json:"is_public,omitempty"` // 是否公开（可选）
}

// SkinResponse 皮肤响应 DTO。
//
// 不再嵌入 entity.SkinLibrary，改为显式字段定义。
// Texture 字段从数据库的 int64 文件 ID 变更为 beacon-bucket 返回的下载链接。
type SkinResponse struct {
    ID            xSnowflake.SnowflakeID    `json:"id"`                       // 皮肤库记录 ID
    UserID        *xSnowflake.SnowflakeID   `json:"user_id,omitempty"`        // 创建者/上传者用户 ID
    Name          string                    `json:"name"`                     // 皮肤名称
    TextureURL    string                    `json:"texture_url"`              // 纹理文件下载链接（由 bucket.Get 解析）
    TextureHash   string                    `json:"texture_hash"`             // 纹理 SHA256 哈希
    Model         entity.ModelType          `json:"model"`                    // 皮肤模型 (1=classic, 2=slim)
    IsPublic      bool                      `json:"is_public"`                // 是否公开
    AssignmentType entityType.AssignmentType `json:"assignment_type,omitempty"` // 关联类型（mine 模式下返回）
}

// SkinListResponse 皮肤列表响应
type SkinListResponse struct {
    Total int64          `json:"total"` // 总数
    Items []SkinResponse `json:"items"` // 皮肤列表
}
```

### 3.2 CapeResponse 重构

**文件**: `api/library/cape.go`

```go
package library

import (
    xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
    entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
)

// CreateCapeRequest 创建披风请求
type CreateCapeRequest struct {
    Name     string `json:"name" binding:"required"`    // 披风名称
    Texture  string `json:"texture" binding:"required"` // 披风纹理文件 base64
    IsPublic *bool  `json:"is_public,omitempty"`        // 是否公开（可选，默认 false）
}

// UpdateCapeRequest 更新披风请求
type UpdateCapeRequest struct {
    Name     *string `json:"name,omitempty"`      // 披风名称（可选）
    IsPublic *bool   `json:"is_public,omitempty"` // 是否公开（可选）
}

// CapeResponse 披风响应 DTO。
//
// 不再嵌入 entity.CapeLibrary，改为显式字段定义。
// Texture 字段从数据库的 int64 文件 ID 变更为 beacon-bucket 返回的下载链接。
type CapeResponse struct {
    ID            xSnowflake.SnowflakeID    `json:"id"`                       // 披风库记录 ID
    UserID        *xSnowflake.SnowflakeID   `json:"user_id,omitempty"`        // 创建者/上传者用户 ID
    Name          string                    `json:"name"`                     // 披风名称
    TextureURL    string                    `json:"texture_url"`              // 纹理文件下载链接（由 bucket.Get 解析）
    TextureHash   string                    `json:"texture_hash"`             // 纹理 SHA256 哈希
    IsPublic      bool                      `json:"is_public"`                // 是否公开
    AssignmentType entityType.AssignmentType `json:"assignment_type,omitempty"` // 关联类型（mine 模式下返回）
}

// CapeListResponse 披风列表响应
type CapeListResponse struct {
    Total int64          `json:"total"` // 总数
    Items []CapeResponse `json:"items"` // 披风列表
}
```

### 3.3 GameProfile DTO 新增

**文件**: `api/user/game_profile.go`

```go
package user

import (
    "time"

    xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
    apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
)

type AddGameProfileRequest struct {
    Name string `json:"name" binding:"required"`
}

type ChangeUsernameRequest struct {
    NewName string `json:"new_name" binding:"required"`
}

// GameProfileResponse 游戏档案响应 DTO。
//
// 不再嵌入 entity.GameProfile，改为显式字段定义。
// 嵌套的 Skin/Cape 使用 library 包的 DTO（已包含 texture_url），
// 不再使用 entity 的原始 Texture int64 字段。
type GameProfileResponse struct {
    ID            xSnowflake.SnowflakeID  `json:"id"`                        // 档案 ID
    UserID        xSnowflake.SnowflakeID  `json:"user_id"`                   // 关联用户 ID
    UUID          string                  `json:"uuid"`                      // UUIDv7 标识
    Name          string                  `json:"name"`                      // 档案用户名
    SkinLibraryID *xSnowflake.SnowflakeID `json:"skin_library_id,omitempty"` // 装备的皮肤库 ID
    CapeLibraryID *xSnowflake.SnowflakeID `json:"cape_library_id,omitempty"` // 装备的披风库 ID
    Skin          *apiLibrary.SkinResponse `json:"skin,omitempty"`            // 装备的皮肤信息（含 texture_url）
    Cape          *apiLibrary.CapeResponse `json:"cape,omitempty"`            // 装备的披风信息（含 texture_url）
}

// GameProfileListResponse 游戏档案列表响应
type GameProfileListResponse struct {
    Items []GameProfileResponse `json:"items"` // 游戏档案列表
}
```

---

## 四、Logic 层变更

### 4.1 新增 Texture URL 解析方法

在 `LibraryLogic` 中新增以下辅助方法：

```go
// resolveTextureURL 通过 beacon-bucket SDK 的 Get 方法将 Texture ID 解析为下载链接。
//
// 参数说明:
//   - ctx: 请求上下文
//   - textureID: 数据库中存储的 int64 纹理文件 ID
//
// 返回值:
//   - string: 文件下载链接
//   - *xError.Error: 当 Bucket 服务不可用或文件不存在时返回错误
func (l *LibraryLogic) resolveTextureURL(ctx context.Context, textureID int64) (string, *xError.Error) {
    fileID := strconv.FormatInt(textureID, 10)
    resp, err := l.helper.bucket.Normal.Get(ctx, &bBucketApi.GetRequest{
        FileId: fileID,
    })
    if err != nil {
        return "", xError.NewError(ctx, xError.ServerInternalError, "获取纹理文件信息失败", true, err)
    }
    if resp.GetObj() == nil {
        return "", xError.NewError(ctx, xError.ServerInternalError, "纹理文件元数据缺失", true)
    }
    link := resp.GetObj().GetLink()
    if link == "" {
        return "", xError.NewError(ctx, xError.ServerInternalError, "纹理文件下载链接为空", true)
    }
    return link, nil
}
```

### 4.2 新增 Entity → DTO 转换方法

```go
// convertSkinEntity 将 SkinLibrary 实体转换为 SkinResponse DTO。
//
// 在转换过程中调用 bucket.Get 解析 Texture ID 为下载链接。
// 若 Bucket 服务不可用，直接返回错误（不降级）。
func (l *LibraryLogic) convertSkinEntity(ctx context.Context, skin *entity.SkinLibrary) (*apiLibrary.SkinResponse, *xError.Error) {
    url, xErr := l.resolveTextureURL(ctx, skin.Texture)
    if xErr != nil {
        return nil, xErr
    }
    return &apiLibrary.SkinResponse{
        ID:          skin.ID,
        UserID:      skin.UserID,
        Name:        skin.Name,
        TextureURL:  url,
        TextureHash: skin.TextureHash,
        Model:       skin.Model,
        IsPublic:    skin.IsPublic,
    }, nil
}

// convertCapeEntity 将 CapeLibrary 实体转换为 CapeResponse DTO。
func (l *LibraryLogic) convertCapeEntity(ctx context.Context, cape *entity.CapeLibrary) (*apiLibrary.CapeResponse, *xError.Error) {
    url, xErr := l.resolveTextureURL(ctx, cape.Texture)
    if xErr != nil {
        return nil, xErr
    }
    return &apiLibrary.CapeResponse{
        ID:          cape.ID,
        UserID:      cape.UserID,
        Name:        cape.Name,
        TextureURL:  url,
        TextureHash: cape.TextureHash,
        IsPublic:    cape.IsPublic,
    }, nil
}
```

### 4.3 新增批量转换方法

```go
// convertSkinEntities 批量将 SkinLibrary 实体列表转换为 SkinResponse DTO 列表。
//
// 逐个调用 resolveTextureURL 解析纹理链接。若任一解析失败，整个批次中止并返回错误。
// 未来可优化为并发调用或引入缓存机制。
func (l *LibraryLogic) convertSkinEntities(ctx context.Context, skins []entity.SkinLibrary) ([]apiLibrary.SkinResponse, *xError.Error) {
    responses := make([]apiLibrary.SkinResponse, len(skins))
    for i, skin := range skins {
        resp, xErr := l.convertSkinEntity(ctx, &skin)
        if xErr != nil {
            return nil, xErr
        }
        responses[i] = *resp
    }
    return responses, nil
}

// convertCapeEntities 批量将 CapeLibrary 实体列表转换为 CapeResponse DTO 列表。
func (l *LibraryLogic) convertCapeEntities(ctx context.Context, capes []entity.CapeLibrary) ([]apiLibrary.CapeResponse, *xError.Error) {
    responses := make([]apiLibrary.CapeResponse, len(capes))
    for i, cape := range capes {
        resp, xErr := l.convertCapeEntity(ctx, &cape)
        if xErr != nil {
            return nil, xErr
        }
        responses[i] = *resp
    }
    return responses, nil
}
```

### 4.4 新增关联实体转换方法（mine 模式）

```go
// convertUserSkinAssociations 将 UserSkinLibrary 关联列表转换为 SkinResponse DTO 列表。
//
// 从关联实体中提取 SkinLibrary（GORM Preload）和 AssignmentType，
// 并调用 bucket.Get 解析纹理链接。
func (l *LibraryLogic) convertUserSkinAssociations(ctx context.Context, associations []entity.UserSkinLibrary) ([]apiLibrary.SkinResponse, *xError.Error) {
    responses := make([]apiLibrary.SkinResponse, len(associations))
    for i, assoc := range associations {
        resp := apiLibrary.SkinResponse{
            AssignmentType: assoc.AssignmentType,
        }
        if assoc.SkinLibrary != nil {
            url, xErr := l.resolveTextureURL(ctx, assoc.SkinLibrary.Texture)
            if xErr != nil {
                return nil, xErr
            }
            resp.ID = assoc.SkinLibrary.ID
            resp.UserID = assoc.SkinLibrary.UserID
            resp.Name = assoc.SkinLibrary.Name
            resp.TextureURL = url
            resp.TextureHash = assoc.SkinLibrary.TextureHash
            resp.Model = assoc.SkinLibrary.Model
            resp.IsPublic = assoc.SkinLibrary.IsPublic
        }
        responses[i] = resp
    }
    return responses, nil
}

// convertUserCapeAssociations 将 UserCapeLibrary 关联列表转换为 CapeResponse DTO 列表。
func (l *LibraryLogic) convertUserCapeAssociations(ctx context.Context, associations []entity.UserCapeLibrary) ([]apiLibrary.CapeResponse, *xError.Error) {
    responses := make([]apiLibrary.CapeResponse, len(associations))
    for i, assoc := range associations {
        resp := apiLibrary.CapeResponse{
            AssignmentType: assoc.AssignmentType,
        }
        if assoc.CapeLibrary != nil {
            url, xErr := l.resolveTextureURL(ctx, assoc.CapeLibrary.Texture)
            if xErr != nil {
                return nil, xErr
            }
            resp.ID = assoc.CapeLibrary.ID
            resp.UserID = assoc.CapeLibrary.UserID
            resp.Name = assoc.CapeLibrary.Name
            resp.TextureURL = url
            resp.TextureHash = assoc.CapeLibrary.TextureHash
            resp.IsPublic = assoc.CapeLibrary.IsPublic
        }
        responses[i] = resp
    }
    return responses, nil
}
```

### 4.5 Logic 方法签名变更

以下方法的**返回值类型**从 entity 变更为 DTO：

| 方法 | 旧签名 | 新签名 |
|------|--------|--------|
| `CreateSkin` | `(*entity.SkinLibrary, *xError.Error)` | `(*apiLibrary.SkinResponse, *xError.Error)` |
| `UpdateSkin` | `(*entity.SkinLibrary, *xError.Error)` | `(*apiLibrary.SkinResponse, *xError.Error)` |
| `ListSkins` | `([]entity.SkinLibrary, int64, *xError.Error)` | `([]apiLibrary.SkinResponse, int64, *xError.Error)` |
| `ListMySkins` | `([]entity.UserSkinLibrary, int64, *xError.Error)` | `([]apiLibrary.SkinResponse, int64, *xError.Error)` |
| `CreateCape` | `(*entity.CapeLibrary, *xError.Error)` | `(*apiLibrary.CapeResponse, *xError.Error)` |
| `UpdateCape` | `(*entity.CapeLibrary, *xError.Error)` | `(*apiLibrary.CapeResponse, *xError.Error)` |
| `ListCapes` | `([]entity.CapeLibrary, int64, *xError.Error)` | `([]apiLibrary.CapeResponse, int64, *xError.Error)` |
| `ListMyCapes` | `([]entity.UserCapeLibrary, int64, *xError.Error)` | `([]apiLibrary.CapeResponse, int64, *xError.Error)` |
| `ListUserSkins` | `([]entity.UserSkinLibrary, int64, *xError.Error)` | `([]apiLibrary.SkinResponse, int64, *xError.Error)` |
| `ListUserCapes` | `([]entity.UserCapeLibrary, int64, *xError.Error)` | `([]apiLibrary.CapeResponse, int64, *xError.Error)` |

### 4.6 GameProfile Logic 变更

**文件**: `internal/logic/game_profile.go`

GameProfileLogic 需要引入 `libraryHelper` 来调用纹理解析方法。两种可行方案：

**方案 A（推荐）: 注入 LibraryLogic**

在 `GameProfileLogic` 中引用 `LibraryLogic` 实例，复用其 `convertSkinEntity` / `convertCapeEntity` 方法。

```go
type GameProfileLogic struct {
    logic
    repo          gameProfileRepo
    libraryLogic  *LibraryLogic  // 新增：复用纹理解析能力
}
```

在 `NewGameProfileLogic` 中接收 `LibraryLogic` 参数，在 `handler.go` 的 `NewHandler` 中按依赖顺序初始化。

**方案 B: 提取共用 Helper**

将 `resolveTextureURL` 提取为独立 struct，`GameProfileLogic` 和 `LibraryLogic` 共同引用。

> 本设计采用**方案 A**，理由：GameProfile 中纹理解析是辅助功能，直接复用 LibraryLogic 的方法最简洁，避免为单一方法引入额外的抽象层。

**新增 GameProfile 转换方法**：

```go
// convertProfileEntity 将 GameProfile 实体转换为 GameProfileResponse DTO。
//
// 若 profile 关联了 SkinLibrary 或 CapeLibrary（GORM Preload），则调用
// LibraryLogic 的转换方法解析纹理链接。
func (l *GameProfileLogic) convertProfileEntity(ctx context.Context, profile *entity.GameProfile) (*apiUser.GameProfileResponse, *xError.Error) {
    resp := &apiUser.GameProfileResponse{
        ID:            profile.ID,
        UserID:        profile.UserID,
        UUID:          profile.UUID.String(),
        Name:          profile.Name,
        SkinLibraryID: profile.SkinLibraryID,
        CapeLibraryID: profile.CapeLibraryID,
    }

    if profile.SkinLibrary != nil {
        skinResp, xErr := l.libraryLogic.convertSkinEntity(ctx, profile.SkinLibrary)
        if xErr != nil {
            return nil, xErr
        }
        resp.Skin = skinResp
    }

    if profile.CapeLibrary != nil {
        capeResp, xErr := l.libraryLogic.convertCapeEntity(ctx, profile.CapeLibrary)
        if xErr != nil {
            return nil, xErr
        }
        resp.Cape = capeResp
    }

    return resp, nil
}
```

**GameProfileLogic 方法签名变更**：

| 方法 | 旧签名 | 新签名 |
|------|--------|--------|
| `AddGameProfile` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |
| `ChangeUsername` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |
| `GetGameProfileDetail` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |
| `ListGameProfiles` | `([]entity.GameProfile, *xError.Error)` | `([]apiUser.GameProfileResponse, *xError.Error)` |
| `EquipSkin` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |
| `UnequipSkin` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |
| `EquipCape` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |
| `UnequipCape` | `(*entity.GameProfile, *xError.Error)` | `(*apiUser.GameProfileResponse, *xError.Error)` |

### 4.7 Logic 层方法实现示例

以 `CreateSkin` 为例，展示 Logic 层完整变更：

```go
// CreateSkin 创建皮肤。
//
// 变更说明：返回值从 *entity.SkinLibrary 变更为 *apiLibrary.SkinResponse，
// 在创建完成后调用 bucket.Get 解析纹理链接，将 Texture ID 转换为下载 URL。
func (l *LibraryLogic) CreateSkin(ctx context.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateSkinRequest) (*apiLibrary.SkinResponse, *xError.Error) {
    l.log.Info(ctx, "CreateSkin - 创建皮肤")

    // ...参数校验、Base64 解码、SHA256 哈希计算（不变）...

    // 上传到对象存储（事务外执行，避免长事务占用连接）
    uploadResp, err := l.helper.bucket.Normal.Upload(ctx, &bBucketApi.UploadRequest{
        BucketId:      "360607182437229568",
        PathId:        "360607485278626816",
        ContentBase64: req.Texture,
    })
    if err != nil {
        return nil, xError.NewError(ctx, xError.ServerInternalError, "上传皮肤纹理失败", true, err)
    }

    skinId, err := strconv.ParseInt(uploadResp.FileId, 10, 64)
    if err != nil {
        return nil, xError.NewError(ctx, xError.ServerInternalError, "解析纹理文件 ID 失败", true, err)
    }

    skin := &entity.SkinLibrary{ /* ... 构建 entity ... */ }

    // 委托 Repository 层在事务内完成创建
    createdSkin, xErr := l.repo.txn.CreateSkinWithQuota(ctx, skin)
    if xErr != nil {
        return nil, xErr
    }

    // 将 entity 转换为 DTO（含纹理链接解析）
    return l.convertSkinEntity(ctx, createdSkin)
}
```

以 `ListSkins` 为例：

```go
// ListSkins 获取市场公开皮肤列表。
func (l *LibraryLogic) ListSkins(ctx context.Context, page int, pageSize int) ([]apiLibrary.SkinResponse, int64, *xError.Error) {
    l.log.Info(ctx, "ListSkins - 获取公开皮肤列表")

    skins, total, xErr := l.repo.skinRepo.ListPublic(ctx, nil, page, pageSize)
    if xErr != nil {
        return nil, 0, xErr
    }

    responses, xErr := l.convertSkinEntities(ctx, skins)
    if xErr != nil {
        return nil, 0, xErr
    }

    return responses, total, nil
}
```

---

## 五、Handler 层变更

### 5.1 变更概述

Handler 层整体**简化**。由于 Logic 层已返回 DTO，Handler 无需再做 entity → Response 的转换，直接将 Logic 返回值传入 `xResult.SuccessHasData`。

### 5.2 移除 Handler 中的转换方法

以下 4 个方法将被**删除**：

```go
// 以下方法移至 Logic 层，Handler 不再需要
func (h *LibraryHandler) convertSkinEntitiesToResponses(skins []entity.SkinLibrary) []apiLibrary.SkinResponse
func (h *LibraryHandler) convertCapeEntitiesToResponses(capes []entity.CapeLibrary) []apiLibrary.CapeResponse
func (h *LibraryHandler) convertUserSkinLibrariesToResponses(associations []entity.UserSkinLibrary) []apiLibrary.SkinResponse
func (h *LibraryHandler) convertUserCapeLibrariesToResponses(associations []entity.UserCapeLibrary) []apiLibrary.CapeResponse
```

### 5.3 Handler 方法变更示例

以 `CreateSkin` 为例：

```go
// 变更前
func (h *LibraryHandler) CreateSkin(ctx *gin.Context) {
    // ...参数解析、鉴权（不变）...
    skin, xErr := h.service.libraryLogic.CreateSkin(ctx.Request.Context(), userID, req)
    if xErr != nil {
        _ = ctx.Error(xErr)
        return
    }
    xResult.SuccessHasData(ctx, "创建皮肤成功", skin)  // skin 是 *entity.SkinLibrary
}

// 变更后
func (h *LibraryHandler) CreateSkin(ctx *gin.Context) {
    // ...参数解析、鉴权（不变）...
    skin, xErr := h.service.libraryLogic.CreateSkin(ctx.Request.Context(), userID, req)
    if xErr != nil {
        _ = ctx.Error(xErr)
        return
    }
    xResult.SuccessHasData(ctx, "创建皮肤成功", skin)  // skin 是 *apiLibrary.SkinResponse
}
```

Handler 层代码**无实质性变更**，仅是返回值类型随 Logic 层签名变更而自动适配。

以 `ListSkins` 为例：

```go
// 变更前
if mode == "market" {
    skins, total, xErr := h.service.libraryLogic.ListSkins(ctx.Request.Context(), page, pageSize)
    // ...
    response := apiLibrary.SkinListResponse{
        Total: total,
        Items: h.convertSkinEntitiesToResponses(skins),  // Handler 手动转换
    }
    xResult.SuccessHasData(ctx, "获取皮肤列表成功", response)
}

// 变更后
if mode == "market" {
    skins, total, xErr := h.service.libraryLogic.ListSkins(ctx.Request.Context(), page, pageSize)
    // ...
    response := apiLibrary.SkinListResponse{
        Total: total,
        Items: skins,  // 直接使用 Logic 返回的 DTO
    }
    xResult.SuccessHasData(ctx, "获取皮肤列表成功", response)
}
```

### 5.4 GameProfile Handler 变更

```go
// 变更前
func (h *GameProfileHandler) GetGameProfileDetail(ctx *gin.Context) {
    // ...参数解析、鉴权...
    profile, xErr := h.service.gameProfileLogic.GetGameProfileDetail(ctx.Request.Context(), userID, profileID)
    // ...
    xResult.SuccessHasData(ctx, "获取游戏档案详情成功", profile)  // *entity.GameProfile
}

// 变更后
func (h *GameProfileHandler) GetGameProfileDetail(ctx *gin.Context) {
    // ...参数解析、鉴权...
    profile, xErr := h.service.gameProfileLogic.GetGameProfileDetail(ctx.Request.Context(), userID, profileID)
    // ...
    xResult.SuccessHasData(ctx, "获取游戏档案详情成功", profile)  // *apiUser.GameProfileResponse
}
```

---

## 六、API 响应变更

### 6.1 皮肤响应（Breaking Change）

**变更前**：

```json
{
    "code": 0,
    "message": "创建皮肤成功",
    "data": {
        "id": 7234567890123456,
        "user_id": 7123456789012345,
        "name": "我的皮肤",
        "texture": 360607182437229568,
        "texture_hash": "a1b2c3d4...",
        "model": 1,
        "is_public": false
    }
}
```

**变更后**：

```json
{
    "code": 0,
    "message": "创建皮肤成功",
    "data": {
        "id": 7234567890123456,
        "user_id": 7123456789012345,
        "name": "我的皮肤",
        "texture_url": "https://bucket.example.com/skins/360607182437229568.png",
        "texture_hash": "a1b2c3d4...",
        "model": 1,
        "is_public": false
    }
}
```

**Breaking Change 说明**：

| 字段 | 变更前 | 变更后 | 影响 |
|------|--------|--------|------|
| `texture` | `int64`（文件 ID） | **已移除** | 前端需适配 |
| `texture_url` | 不存在 | `string`（下载链接） | 前端需适配 |

### 6.2 披风响应（Breaking Change）

同皮肤响应，`texture` → `texture_url`。

### 6.3 游戏档案详情响应（Breaking Change）

**变更前**：

```json
{
    "data": {
        "id": 7345678901234567,
        "user_id": 7123456789012345,
        "uuid": "019f1234-5678-7abc-def0-123456789abc",
        "name": "Player1",
        "skin_library_id": 7234567890123456,
        "cape_library_id": null,
        "skin_library": {
            "id": 7234567890123456,
            "name": "我的皮肤",
            "texture": 360607182437229568,
            "model": 1,
            ...
        }
    }
}
```

**变更后**：

```json
{
    "data": {
        "id": 7345678901234567,
        "user_id": 7123456789012345,
        "uuid": "019f1234-5678-7abc-def0-123456789abc",
        "name": "Player1",
        "skin_library_id": 7234567890123456,
        "cape_library_id": null,
        "skin": {
            "id": 7234567890123456,
            "name": "我的皮肤",
            "texture_url": "https://bucket.example.com/skins/360607182437229568.png",
            "model": 1,
            ...
        }
    }
}
```

**Breaking Change 说明**：

| 字段 | 变更前 | 变更后 | 影响 |
|------|--------|--------|------|
| `skin_library` | `entity.SkinLibrary`（含 texture int64） | **已移除** | 前端需适配 |
| `cape_library` | `entity.CapeLibrary`（含 texture int64） | **已移除** | 前端需适配 |
| `skin` | 不存在 | `SkinResponse`（含 texture_url string） | 前端需适配 |
| `cape` | 不存在 | `CapeResponse`（含 texture_url string） | 前端需适配 |
| `uuid` | `uuid.UUID`（JSON 格式） | `string` | 格式可能微调 |

### 6.4 受影响接口总览

| 方法 | 路径 | 响应变更 |
|------|------|---------|
| `POST` | `/library/skins` | `texture` → `texture_url` |
| `GET` | `/library/skins` | 同上（market + mine 模式） |
| `PATCH` | `/library/skins/:skin_id` | `texture` → `texture_url` |
| `DELETE` | `/library/skins/:skin_id` | 无变更（无数据返回） |
| `POST` | `/library/capes` | `texture` → `texture_url` |
| `GET` | `/library/capes` | 同上（market + mine 模式） |
| `PATCH` | `/library/capes/:cape_id` | `texture` → `texture_url` |
| `DELETE` | `/library/capes/:cape_id` | 无变更（无数据返回） |
| `GET` | `/library/quota` | 无变更（无 Texture 字段） |
| `POST` | `/library/admin/users/:user_id/skins/gift` | 返回 `UserSkinLibrary`，无变更 |
| `DELETE` | `/library/admin/users/:user_id/skins/:skin_library_id` | 无变更 |
| `POST` | `/library/admin/users/:user_id/capes/gift` | 返回 `UserCapeLibrary`，无变更 |
| `DELETE` | `/library/admin/users/:user_id/capes/:cape_library_id` | 无变更 |
| `POST` | `/library/admin/users/:user_id/quota/sync` | 无变更 |
| `GET` | `/library/admin/users/:user_id/skins` | `texture` → `texture_url` |
| `GET` | `/library/admin/users/:user_id/capes` | `texture` → `texture_url` |
| `POST` | `/game-profile` | `skin_library` → `skin`（含 texture_url） |
| `GET` | `/game-profile` | 同上 |
| `GET` | `/game-profile/:profile_id` | 同上 |
| `PATCH` | `/game-profile/:profile_id/username` | 同上 |
| `POST` | `/game-profile/:profile_id/skin/:skin_library_id` | 同上 |
| `DELETE` | `/game-profile/:profile_id/skin` | 同上 |
| `POST` | `/game-profile/:profile_id/cape/:cape_library_id` | 同上 |
| `DELETE` | `/game-profile/:profile_id/cape` | 同上 |

---

## 七、Handler 构造器变更

### 7.1 handler.go 初始化顺序调整

**文件**: `internal/handler/handler.go`

由于 `GameProfileLogic` 现在依赖 `LibraryLogic`，初始化顺序需调整：

```go
func NewHandler[T IHandler](ctx context.Context, handlerName string) *T {
    // LibraryLogic 先于 GameProfileLogic 初始化
    libraryLogic := logic.NewLibraryLogic(ctx)

    return &T{
        name: handlerName,
        log:  xLog.WithName(xLog.NamedCONT, handlerName),
        service: &service{
            userLogic:        logic.NewUserLogic(ctx),
            gameProfileLogic: logic.NewGameProfileLogic(ctx, libraryLogic),  // 注入 libraryLogic
            libraryLogic:     libraryLogic,
            oauthLogic:       bSdkLogic.NewBusiness(ctx),
        },
    }
}
```

### 7.2 GameProfileLogic 构造器变更

```go
// NewGameProfileLogic 创建游戏档案业务逻辑实例。
//
// 参数说明:
//   - ctx: 上下文对象
//   - libraryLogic: 资源库业务逻辑实例，用于复用纹理链接解析能力
func NewGameProfileLogic(ctx context.Context, libraryLogic *LibraryLogic) *GameProfileLogic {
    db := xCtxUtil.MustGetDB(ctx)
    rdb := xCtxUtil.MustGetRDB(ctx)

    // ...初始化 repos（不变）...

    return &GameProfileLogic{
        logic: logic{
            rdb: rdb,
            log: xLog.WithName(xLog.NamedLOGC, "GameProfileLogic"),
        },
        repo: gameProfileRepo{
            // ...不变...
        },
        libraryLogic: libraryLogic,  // 新增
    }
}
```

---

## 八、错误处理

### 8.1 新增错误场景

| 场景 | HTTP 状态码 | 错误码 | Message | 触发条件 |
|------|------------|--------|---------|---------|
| Bucket Get RPC 失败 | 500 | `ServerInternalError` | "获取纹理文件信息失败" | Bucket 服务不可用/网络超时 |
| 文件元数据缺失 | 500 | `ServerInternalError` | "纹理文件元数据缺失" | GetResponse.Obj 为 nil |
| 下载链接为空 | 500 | `ServerInternalError` | "纹理文件下载链接为空" | GetResponse.Obj.Link 为空字符串 |

### 8.2 错误传播路径

```
bucket.Normal.Get() 返回 error
  → resolveTextureURL() 封装为 *xError.Error
    → convertSkinEntity() 透传
      → Logic 方法返回 *xError.Error
        → Handler 通过 ctx.Error(xErr) 注入
          → ResponseMiddleware 统一处理响应
```

遵循项目现有的错误透传原则：上层直接 return 下层的 xErr，不做二次包装。

---

## 九、性能考量

### 9.1 当期策略（v1）：顺序解析

列表接口中逐个调用 `bucket.Get`。以默认分页 `pageSize=20` 为例：
- 最坏情况：20 次 RPC 调用
- Bucket 使用 Connect RPC (HTTP/2)，连接复用开销低
- 单次 Get 调用预计 < 50ms（内网通信）

### 9.2 未来优化方向

| 方向 | 说明 | 适用场景 |
|------|------|---------|
| **并发解析** | 使用 `sync.WaitGroup` 并发调用 bucket.Get | 列表页 > 10 条时 |
| **本地缓存** | 以 FileID 为 key 缓存 URL，TTL 15min（与 Redis 缓存策略一致） | 高频访问的公开皮肤 |
| **SDK Batch Get** | 推动 beacon-bucket-sdk 新增批量查询接口 | 大规模列表场景 |
| **上传时缓存 URL** | 上传成功后，将 uploadResp.Obj.Link 写入 entity 或缓存，避免后续 Get | 所有写入场景 |

> **建议**: v1 先采用顺序解析，上线后监控 bucket.Get 的 P99 延迟，若超出预期再引入并发或缓存。

---

## 十、文件变更清单

### 修改文件（7 个）

| 路径 | 变更说明 |
|------|---------|
| `api/library/skin.go` | 重构 `SkinResponse`：移除 entity embedding，改为显式字段；`Texture int64` → `TextureURL string` |
| `api/library/cape.go` | 重构 `CapeResponse`：同 skin.go |
| `api/user/game_profile.go` | 重构 `GameProfileDetailResponse` → `GameProfileResponse`：显式字段；嵌套对象使用 library DTO |
| `internal/logic/library.go` | 新增 `resolveTextureURL` / `convertSkinEntity` 等 6 个方法；修改 10 个方法签名；新增 `apiLibrary` import |
| `internal/logic/game_profile.go` | 新增 `libraryLogic` 字段 + `convertProfileEntity` 方法；修改 8 个方法签名 |
| `internal/handler/library.go` | 删除 4 个 `convertXxxToResponses` 方法；简化 Handler 方法中的列表组装逻辑 |
| `internal/handler/handler.go` | 调整初始化顺序：`libraryLogic` 先于 `gameProfileLogic` 创建并注入 |

### 不变的文件

| 路径 | 说明 |
|------|------|
| `internal/entity/*.go` | Entity 层不变，`Texture int64` 仍为存储层标识 |
| `internal/repository/*.go` | Repository 层不变 |
| `internal/repository/txn/*.go` | TxnRepo 层不变 |
| `api/library/gift.go` | Gift 请求结构体不变 |
| `internal/app/route/*.go` | 路由注册不变 |

---

## 十一、实施阶段建议

| 阶段 | 内容 | 风险 | 依赖 |
|------|------|------|------|
| **Phase 1** | 重构 Response DTO（api/ 目录） | 低 — 仅结构体定义变更，不影响运行时 | — |
| **Phase 2** | Logic 层新增 resolve/convert 方法 | 低 — 纯新增方法，不影响现有流程 | Phase 1 |
| **Phase 3** | Logic 层修改返回值签名 + 实现调用 | 中 — 10+ 个方法签名变更，需逐一适配 | Phase 2 |
| **Phase 4** | GameProfileLogic 注入 LibraryLogic | 中 — 构造器链变更，需调整 handler.go | Phase 3 |
| **Phase 5** | Handler 层适配 + 删除旧转换方法 | 低 — 简化逻辑，移除代码 | Phase 4 |
| **Phase 6** | 集成测试 + API 响应验证 | — — 确认所有接口返回正确的 texture_url | Phase 5 |

> **建议**: Phase 1-2 可独立合入（无破坏性变更），Phase 3-5 作为一次 PR 提交（原子性 API Breaking Change），Phase 6 需前后端联调。
