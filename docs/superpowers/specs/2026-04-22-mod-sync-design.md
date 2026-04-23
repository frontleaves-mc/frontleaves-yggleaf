# MC 模组客户端同步器 - 服务端 API 设计

> 日期：2026-04-22
> 状态：已批准

## 背景

MC 模组服的模组和配置文件经常需要增删改，玩家每次手动同步非常麻烦。需要一个客户端同步器 CLI 工具，让玩家一键拉取服务端最新的 mods 和 config 文件。本设计仅覆盖服务端 API 接口部分，CLI 客户端为独立项目。

## 方案选择

**选定方案：目录直读 + 实时哈希**

API 被调用时实时扫描服务端目录，计算文件 SHA-256 哈希，返回元数据。下载接口直接读取文件流式返回。

选择理由：零维护成本，永远反映最新状态，无需数据库表或缓存索引。模组服文件数量通常 < 500，实时扫描性能可接受。

## API 接口设计

### 1. Mods 元数据

```
GET /api/v1/sync/mods/metadata
```

- 认证：无
- 说明：扫描 `minecraft_client/mods/` 目录下所有 `.jar` 文件（不递归子目录），返回元数据列表

响应：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "files": [
      {
        "path": "mods/jei-1.20.1.jar",
        "name": "jei-1.20.1.jar",
        "hash": "sha256:abc123...",
        "size": 1234567
      }
    ],
    "total": 42,
    "scanned_at": "2026-04-22T10:00:00Z"
  }
}
```

### 2. Config 元数据

```
GET /api/v1/sync/config/metadata
```

- 认证：无
- 说明：递归扫描 `minecraft_client/config/` 目录下所有文件，返回元数据列表

响应结构与 mods 一致，`path` 保留 `config/` 前缀的相对路径。

### 3. 文件下载

```
GET /api/v1/sync/download?path=mods/jei-1.20.1.jar
```

- 认证：无
- 成功：返回文件二进制流（`Content-Type: application/octet-stream`）
- 失败：返回标准 `BaseResponse` 错误

请求参数：

| 参数 | 位置 | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| path | query | string | 是 | 文件相对路径，必须以 `mods/` 或 `config/` 开头 |

响应头：

- `Content-Type: application/octet-stream`
- `Content-Disposition: attachment; filename="<文件名>"`

## 文件结构约定

```
<运行目录>/minecraft_client/
├── mods/          # 模组目录（仅扫描 .jar 文件）
│   ├── jei-1.20.1.jar
│   └── ...
└── config/        # 配置目录（递归扫描所有文件）
    ├── jei/
    │   └── jei.cfg
    └── ...
```

程序启动时若 `minecraft_client/` 目录不存在则自动创建。

## 架构设计

### 分层

遵循项目现有 Handler → Logic 架构，但无需 Repository、Entity、Cache 层。

```
Handler (sync.go)  →  Logic (sync.go)  →  文件系统
```

### 新增/修改文件

| 操作 | 文件 | 说明 |
|------|------|------|
| 新增 | `api/sync/model.go` | 请求/响应 DTO |
| 新增 | `internal/handler/sync.go` | SyncHandler（3 个方法） |
| 新增 | `internal/logic/sync.go` | SyncLogic（目录扫描 + 文件哈希 + 文件流读取） |
| 新增 | `internal/app/route/route_sync.go` | 路由注册 |
| 修改 | `internal/handler/handler.go` | 新增 SyncHandler 类型、service 新增 syncLogic 字段 |
| 修改 | `internal/app/route/route.go` | 注册 syncRouter |

### SyncLogic

```go
type SyncLogic struct {
    ctx      context.Context
    basePath string  // "./minecraft_client"
}
```

方法：

- `ScanMods() → []FileMetadata` — 扫描 mods/ 下 .jar 文件，计算 SHA-256
- `ScanConfig() → []FileMetadata` — 递归扫描 config/ 下所有文件，计算 SHA-256
- `DownloadFile(path string) → (io.ReadCloser, FileInfo, error)` — 安全校验后返回文件流

### FileMetadata

```go
type FileMetadata struct {
    Path string `json:"path"`  // 相对路径，如 "mods/jei-1.20.1.jar"
    Name string `json:"name"`  // 文件名，如 "jei-1.20.1.jar"
    Hash string `json:"hash"`  // "sha256:<hex>"
    Size int64  `json:"size"`  // 文件大小（字节）
}
```

### 路由注册

```go
func (r *route) syncRouter(route gin.IRouter) {
    syncGroup := route.Group("/sync")
    {
        syncGroup.GET("/mods/metadata", syncHandler.ModsMetadata)
        syncGroup.GET("/config/metadata", syncHandler.ConfigMetadata)
        syncGroup.GET("/download", syncHandler.Download)
    }
}
```

无中间件、无认证，直接挂载。

## 安全防护

1. **路径遍历检测**：`filepath.Clean` 后检查路径是否包含 `..`
2. **白名单前缀**：只允许 `mods/` 和 `config/` 开头的路径
3. **不暴露绝对路径**：响应中只包含相对路径
4. **流式下载**：不将整个文件缓存到内存

## 不涉及的内容

- 客户端 CLI 工具（独立项目）
- 数据库存储
- Redis 缓存
- 认证鉴权
- 环境变量配置
