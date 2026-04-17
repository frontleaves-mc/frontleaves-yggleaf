# 问题系统 (Issue System) 设计文档

> 日期: 2026-04-18
> 状态: 待审核
> 方案: 方案 A - 标准 Issue Tracker

---

## 1. 概述

### 1.1 目标

构建一个完整的问题反馈系统，允许用户提交问题、管理员进行回复与处理。系统支持问题分类、多状态流转、优先级管理、附件上传和内部备注功能。

### 1.2 核心角色

| 角色 | 权限 |
|------|------|
| **用户** | 提交问题、查看自己的问题列表/详情、追加回复、上传附件 |
| **管理员** | 查看所有问题、回复任意问题、修改状态/优先级/备注、管理问题类型 |

### 1.3 可见性规则

- 用户仅可查看自己提交的问题
- 管理员可查看所有问题
- 内部备注（admin_note）仅管理员可见

---

## 2. 数据库设计

### 2.1 实体关系图

```
fyl_issue_type (1) ──< (N) fyl_issue (1) ──< (N) fyl_issue_reply
                                                    │
                                                    (N)
                                                    │
                                              fyl_issue_attachment
```

### 2.2 fyl_issue_type — 问题类型表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | bigint | PK, Snowflake | 主键 |
| created_at | timestamptz | NOT NULL | 创建时间（BaseEntity） |
| updated_at | timestamptz | NOT NULL | 更新时间（BaseEntity） |
| name | varchar(32) | NOT NULL, UNIQUE | 类型名称 |
| description | varchar(255) | | 类型描述 |
| sort_order | int | DEFAULT 0 | 排序序号 |
| is_enabled | boolean | DEFAULT true | 是否启用 |

**预置数据：**
- Bug 反馈
- 功能建议
- 账号问题
- 其他

### 2.3 fyl_issue — 问题主表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | bigint | PK, Snowflake | 主键 |
| created_at | timestamptz | NOT NULL | 创建时间（BaseEntity） |
| updated_at | timestamptz | NOT NULL | 更新时间（BaseEntity） |
| user_id | bigint | NOT NULL, INDEX, FK→users | 提交者 ID |
| issue_type_id | bigint | NOT NULL, FK→issue_types | 问题类型 ID |
| title | varchar(128) | NOT NULL | 问题标题 |
| content | text | NOT NULL | 问题描述（自由文本） |
| status | varchar(20) | NOT NULL, DEFAULT 'pending' | 当前状态 |
| priority | varchar(10) | NOT NULL, DEFAULT 'medium' | 优先级 |
| admin_note | text | | 内部备注（仅管理员可见） |
| closed_at | timestamptz | | 关闭时间 |

**外键关联：**
- `user_id` → `fyl_users.id` (CASCADE)
- `issue_type_id` → `fyl_issue_types.id` (RESTRICT)

### 2.4 fyl_issue_reply — 回复表

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | bigint | PK, Snowflake | 主键 |
| created_at | timestamptz | NOT NULL | 创建时间（BaseEntity） |
| updated_at | timestamptz | NOT NULL | 更新时间（BaseEntity） |
| issue_id | bigint | NOT NULL, INDEX, FK→issues | 关联问题 ID |
| user_id | bigint | NOT NULL, FK→users | 回复者 ID |
| content | text | NOT NULL | 回复内容 |
| is_admin_reply | boolean | NOT NULL, DEFAULT false | 是否为管理员回复 |

**外键关联：**
- `issue_id` → `fyl_issues.id` (CASCADE)
- `user_id` → `fyl_users.id` (RESTRICT)

### 2.5 fyl_issue_attachment — 附件表

> **设计对齐 Library 模块（SkinLibrary/CapeLibrary）的文件存储模式：**
> 数据库仅存储 Bucket 返回的 **FileId（int64）**，不存储 URL。
> 读取时通过 `bucket.Normal.Get()` / `GetByList()` 动态解析为下载链接。

| 字段 | 类型 | 约束 | 说明 |
|------|------|------|------|
| id | bigint | PK, Snowflake | 主键 |
| created_at | timestamptz | NOT NULL | 创建时间（BaseEntity） |
| updated_at | timestamptz | NOT NULL | 更新时间（BaseEntity） |
| issue_id | bigint | NOT NULL, INDEX, FK→issues | 关联问题 ID |
| file_id | int64 | NOT NULL | 存储桶文件 ID（与 SkinLibrary.Texture 同模式） |
| file_name | varchar(255) | NOT NULL | 原始文件名 |
| file_size | int64 | NOT NULL | 文件大小（字节） |
| mime_type | varchar(64) | | MIME 类型 |

**外键关联：**
- `issue_id` → `fyl_issues.id` (CASCADE)

**与 Library 模块的对齐关系：**

| Library 模式 | Issue 附件模式 | 说明 |
|-------------|---------------|------|
| `Texture int64` | `FileID int64` | 均存储 Bucket 返回的文件 ID |
| `bucket.Normal.Upload(BucketId, PathId, ContentBase64)` | 相同上传方式 | Base64 内容上传 |
| `bucket.Normal.Get(FileId)` → Link | 相同解析方式 | 读取时动态获取下载链接 |
| `bucket.Normal.GetByList(FileIdList)` | 相同批量解析 | 避免 N+1 RPC 调用 |
| `bucket.Normal.CacheVerify(FileId)` | 相同确认永久态 | DB 写入成功后调用 |
| `bucket.Normal.Delete(FileId)` | 相同删除方式 | 删除记录后同步清理

### 2.6 状态常量定义

```go
// internal/constant/issue.go

const (
    // IssueStatus 状态常量
    IssueStatusRegistered = "registered"  // 已登记
    IssueStatusPending    = "pending"     // 待处理
    IssueStatusProcessing = "processing"  // 处理中
    IssueStatusResolved   = "resolved"    // 已处理
    IssueStatusUnplanned  = "unplanned"   // 未计划
    IssueStatusClosed     = "closed"      // 已关闭
)

// IssuePriority 优先级常量
const (
    PriorityLow    = "low"     // 低
    PriorityMedium = "medium"  // 中
    PriorityHigh   = "high"    // 高
    PriorityUrgent = "urgent"  // 紧急
)
```

### 2.7 状态流转规则

```
                    ┌─────────────┐
                    │  已登记      │
                    └──────┬──────┘
                           │
                           ▼
                    ┌─────────────┐
              ┌────▶│  待处理     │◀────┐
              │     └──────┬──────┘     │
              │            │            │
              │            ▼            │
              │     ┌─────────────┐     │
              │     │  处理中     │     │
              │     └──────┬──────┘     │
              │            │            │
              │     ┌──────┴──────┐     │
              │     ▼             ▼     │
              │ ┌─────────┐  ┌────────┐│
              │ │ 已处理   │  │ 未计划 ││
              │ └────┬────┘  └───┬────┘│
              │      │           │     │
              │      ▼           └─────┤
              │ ┌─────────┐          │
              └─│  已关闭  │◀─────────┘
                └─────────┘

任何状态下管理员均可直接关闭。
已关闭的问题不可再修改状态（除非重新打开为「已登记」）。
```

### 2.8 Snowflake Gene 常量

```go
// internal/constant/gene_number.go 追加

GeneForIssue          xSnowflake.Gene = 41 // 问题
GeneForIssueType      xSnowflake.Gene = 42 // 问题类型
GeneForIssueReply     xSnowflake.Gene = 43 // 问题回复
GeneForIssueAttachment xSnowflake.Gene = 44 // 问题附件
```

---

## 3. API 接口设计

### 3.1 用户接口（需 OAuth2 登录）

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| POST | `/api/v1/issue` | 提交问题 | 已登录用户 |
| GET | `/api/v1/issue/list` | 我的问题列表（分页） | 已登录用户 |
| GET | `/api/v1/issue/:id` | 问题详情（含回复+附件） | 本人或管理员 |
| POST | `/api/v1/issue/:id/reply` | 追加回复 | 本人或管理员 |
| POST | `/api/v1/issue/:id/attachment` | 上传附件 | 本人或管理员 |
| DELETE | `/api/v1/issue/attachment/:id` | 删除附件 | 上传者或管理员 |

### 3.2 管理员接口（需 Admin 权限）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/admin/issue/list` | 全部问题列表（支持筛选） |
| PUT | `/api/v1/admin/issue/:id/status` | 修改问题状态 |
| PUT | `/api/v1/admin/issue/:id/priority` | 修改优先级 |
| PUT | `/api/v1/admin/issue/:id/note` | 更新内部备注 |

### 3.3 问题类型接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/issue-type/list` | 获取启用的类型列表 | 公开 |
| POST | `/api/v1/admin/issue-type` | 创建问题类型 | 管理员 |
| PUT | `/api/v1/admin/issue-type/:id` | 编辑问题类型 | 管理员 |
| DELETE | `/api/v1/admin/issue-type/:id` | 删除问题类型 | 管理员 |

### 3.4 请求 DTO 定义

#### CreateIssueRequest（提交问题）
```go
type CreateIssueRequest struct {
    IssueTypeID  SnowflakeID `json:"issue_type_id" binding:"required"`  // 问题类型 ID
    Title        string      `json:"title" binding:"required,max=128"`  // 标题
    Content      string      `json:"content" binding:"required"`         // 内容
    Priority     string      `json:"priority" binding:"omitempty,oneof=low medium high urgent"` // 优先级
}
```

#### ReplyIssueRequest（回复问题）
```go
type ReplyIssueRequest struct {
    Content string `json:"content" binding:"required"` // 回复内容
}
```

#### UpdateIssueStatusRequest（修改状态）
```go
type UpdateIssueStatusRequest struct {
    Status string `json:"status" binding:"required,oneof=registered pending processing resolved unplanned closed"`
}
```

#### UpdateIssuePriorityRequest（修改优先级）
```go
type UpdateIssuePriorityRequest struct {
    Priority string `json:"priority" binding:"required,oneof=low medium high urgent"`
}
```

#### UpdateIssueNoteRequest（更新备注）
```go
type UpdateIssueNoteRequest struct {
    AdminNote string `json:"admin_note"` // 内部备注
}
```

#### IssueListQuery（列表查询参数）
```go
type IssueListQuery struct {
    Page       int    `form:"page" binding:"omitempty,min=1"`       // 页码
    PageSize   int    `form:"page_size" binding:"omitempty,min=1,max=50"` // 每页条数
    Status     string `form:"status" binding:"omitempty,oneof=..."  // 状态筛选
    Priority   string `form:"priority" binding:"omitempty,oneof=low medium high urgent"` // 优先级筛选
    IssueTypeID SnowflakeID `form:"issue_type_id"`                 // 类型筛选
    Keyword    string `form:"keyword" binding:"omitempty,max=64"`  // 关键词搜索（标题）
}
```

### 3.5 响应 DTO 定义

#### IssueDetailResponse（问题详情）
```go
type IssueDetailResponse struct {
    Issue       entity.Issue             `json:"issue"`        // 问题信息
    IssueType   entity.IssueType         `json:"issue_type"`   // 类型信息
    Replies     []IssueReplyItem         `json:"replies"`      // 回复列表
    Attachments []IssueAttachmentItem    `json:"attachments"`   // 附件列表（含解析后的下载链接）
}

type IssueListItem struct {
    entity.Issue
    IssueTypeName string `json:"issue_type_name"` // 类型名称（冗余字段，避免联查）
    ReplyCount    int    `json:"reply_count"`     // 回复数量
}

type IssueReplyItem struct {
    entity.IssueReply
    Username string `json:"username"` // 回复者用户名
}

// IssueAttachmentItem 附件响应项 — FileId 已通过 Bucket SDK 解析为下载链接
type IssueAttachmentItem struct {
    ID        int64     `json:"id"`         // 附件记录 ID
    FileName  string    `json:"file_name"`  // 原始文件名
    FileSize  int64     `json:"file_size"`  // 文件大小（字节）
    MimeType  string    `json:"mime_type"`  // MIME 类型
    FileURL   string    `json:"file_url"`   // 下载链接（由 bucket.Get(FileId) 动态解析）
}
```

---

## 4. 架构分层设计

### 4.1 文件结构

```
internal/
├── entity/
│   ├── issue.go              # Issue 实体
│   ├── issue_type.go         # IssueType 实体
│   ├── issue_reply.go        # IssueReply 实体
│   └── issue_attachment.go   # IssueAttachment 实体
├── repository/
│   ├── issue.go              # IssueRepo (CRUD + 缓存 + 分页)
│   ├── issue_type.go         # IssueTypeRepo
│   ├── issue_reply.go        # IssueReplyRepo
│   ├── issue_attachment.go   # IssueAttachmentRepo
│   └── cache/
│       └── issue.go          # IssueCache (Redis 缓存)
├── repository/txn/
│   └── issue.go              # IssueTxnRepo (事务操作)
├── logic/
│   └── issue.go              # IssueLogic (业务编排)
├── handler/
│   └── issue.go              # IssueHandler (HTTP 处理)
├── app/route/
│   └── route_issue.go        # 路由注册
├── constant/
│   ├── gene_number.go        # 追加 Gene 常量 (#41~44)
│   └── issue.go              # 状态/优先级常量
api/
├── issue/
│   ├── request.go            # 用户端请求 DTO
│   └── response.go           # 用户端响应 DTO
└── admin/
    └── issue.go              # 管理员端请求 DTO
```

### 4.2 各层职责

#### Entity 层
- 4 个实体结构体，嵌入 `xModels.BaseEntity`
- GORM 标签定义列约束、索引、外键
- 每个实体实现 `GetGene()` 返回对应 Snowflake Gene
- 外键关联定义（User、IssueType）

#### Repository 层
- **IssueRepo**: Get/Set/Create/List（分页）、按 user_id 查询、按状态/类型筛选
- **IssueTypeRepo**: CRUD、获取启用列表
- **IssueReplyRepo**: Create/List（按 issue_id 分页）
- **IssueAttachmentRepo**: Create/Delete、按 issue_id 查询
- **IssueCache**: Redis 缓存（Issue 详情缓存，TTL 15 分钟，与 UserCache 一致）
- **IssueTxnRepo**: 提交问题时的事务操作（Issue + Attachment 原子创建）

#### Logic 层
- **IssueLogic**: 业务编排核心（嵌入 `logic` + `repo` + `helper` 三段式，与 LibraryLogic 结构对齐）
  - `CreateIssue()`: 校验类型存在性 → 创建 Issue → 批量上传 Attachment 至 Bucket → CacheVerify → 返回（含上传返回的下载链接）
  - `GetIssueList()`: 权限判断（用户看自己的 / 管理员看全部）→ 分页查询
  - `GetIssueDetail()`: 权限判断 → 查 Issue + Replies + Attachments → 批量解析附件 FileId 为下载链接
  - `ReplyIssue()`: 权限判断 → 创建 Reply → 更新 Issue.updated_at
  - `UpdateStatus()`: 状态流转校验 → 更新 → 若关闭则记录 closed_at
  - `UpdatePriority()`: 校验 → 更新
  - `UpdateNote()`: 更新
  - `UploadAttachment()`: 上传至 Bucket(获取 FileId) → 创建 Attachment 记录(int64) → CacheVerify
  - `DeleteAttachment()`: 权限校验 → 删除 Bucket 文件(FileId) → 删除记录

#### Handler 层
- **IssueHandler**: 处理所有用户端接口
  - OAuth2 认证 → 参数绑定 → 调用 Logic → 响应封装
- 管理员接口在 IssueHandler 中额外增加 Admin 中间件校验

#### Route 层
- **route_issue.go**:
  ```
  /api/v1/issue          → 用户路由组 (CheckAuth + User middleware)
  /api/v1/admin/issue    → 管理路由组 (CheckAuth + User + Admin middleware)
  /api/v1/issue-type     → 公开路由组 (类型列表)
  /api/v1/admin/issue-type → 管理路由组 (Admin middleware)
  ```

### 4.3 存储桶集成（对齐 Library 模式）

**架构模式：** 完全复用 Library 模块的 `libraryHelper` 模式 —— Logic 层通过嵌入 `helper` 结构体持有 `*bBucket.BucketClient`，所有文件操作委托给 Bucket SDK。

**环境变量（追加到 `.env.example`）：**
```bash
# Issue 附件存储桶 ID [必填]
BUCKET_ISSUE_BUCKET_ID=issues
# Issue 附件存储路径 ID [必填]
BUCKET_ISSUE_PATH_ID=attachments
```

**常量定义（追加到 `internal/constant/environment.go`）：**
```go
EnvBucketIssueBucketId xEnv.EnvKey = "BUCKET_ISSUE_BUCKET_ID" // Issue 附件存储桶 ID
EnvBucketIssuePathId   xEnv.EnvKey = "BUCKET_ISSUE_PATH_ID"   // Issue 附件存储路径 ID
```

**文件操作流程（与 LibraryLogic 完全一致）：**

```
上传: bucket.Normal.Upload(BucketId, PathId, ContentBase64) → FileId(string) → parse to int64 → 存入 DB
读取: bucket.Normal.Get(FileId) / GetByList(FileIdList) → .GetObj().GetLink() → 返回下载 URL
确认: bucket.Normal.CacheVerify(FileId) — DB 事务成功后调用，将缓存态转为永久态
删除: bucket.Normal.Delete(FileId) — DB 删除成功后同步清理 Bucket 文件
```

**IssueLogic 层的 helper 结构体：**
```go
type issueHelper struct {
    bucket *bBucket.BucketClient // 对象存储客户端（与 libraryHelper 同模式）
}
```

**附件 URL 解析方法（与 resolveTextureURL / resolveTextureURLsBatch 对齐）：**
- `resolveAttachmentURL(ctx, fileID int64) (string, error)` — 单条解析
- `resolveAttachmentURLsBatch(ctx, fileIDs []int64) (map[int64]string, error)` — 批量解析

**限制：**
- 支持的文件类型: 图片 (jpg/png/gif/webp)、文档 (pdf/txt)、压缩包 (zip/rar)
- 单文件大小限制: 10MB
- 单问题最多附件数: 9

### 4.4 启动迁移

在 `internal/app/startup/startup_database.go` 的 `migrateTables` 中追加:
```go
&entity.IssueType{},
&entity.Issue{},
&entity.IssueReply{},
&entity.IssueAttachment{},
```

### 4.5 Handler Service 注入

在 `internal/handler/handler.go` 的 `service` 结构体和 `NewHandler` 中追加:
```go
issueLogic *logic.IssueLogic
```

---

## 5. 关键业务规则

### 5.1 权限矩阵

| 操作 | 用户（本人） | 用户（他人） | 管理员 |
|------|:-----------:|:-----------:|:------:|
| 查看问题列表 | 仅自己 | ✗ | 全部 |
| 查看问题详情 | 仅自己 | ✗ | 全部 |
| 提交问题 | ✓ | ✗ | ✓ |
| 回复问题 | 仅自己 | ✗ | 任意 |
| 上传附件 | 仅自己 | ✗ | 任意 |
| 删除附件 | 仅自己 | ✗ | 任意 |
| 修改状态 | ✗ | ✗ | ✓ |
| 修改优先级 | ✗ | ✗ | ✓ |
| 修改内部备注 | ✗ | ✗ | ✓ |
| 管理问题类型 | ✗ | ✗ | ✓ |

### 5.2 状态流转校验

| 当前状态 | 允许转换到 |
|---------|-----------|
| registered（已登记） | pending, processing, unplanned, closed |
| pending（待处理） | processing, resolved, unplanned, closed |
| processing（处理中） | pending, resolved, unplanned, registered, closed |
| resolved（已处理） | closed, registered（重新打开） |
| unplanned（未计划） | pending, processing, closed |
| closed（已关闭） | registered（重新打开） |

### 5.3 数据校验规则

- **标题**: 1~128 字符，必填
- **内容**: 1~10000 字符，必填
- **回复内容**: 1~5000 字符，必填
- **内部备注**: 0~2000 字符，选填
- **附件**: 单文件 ≤10MB，单问题 ≤9 个，支持图片/文档/压缩包

---

## 6. 实现顺序建议

1. **Phase 1 - 基础骨架**: Entity → 常量 → 数据库迁移 → Repository（CRUD）
2. **Phase 2 - 核心功能**: Logic → Handler → Route（用户提交+列表+详情+回复）
3. **Phase 3 - 管理功能**: 管理员接口（状态/优先级/备注/全量列表）
4. **Phase 4 - 附件系统**: Bucket 集成 → 附件上传/删除
5. **Phase 5 - 类型管理**: IssueType CRUD 接口 + 预置数据
