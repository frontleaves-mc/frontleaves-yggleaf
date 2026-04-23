# API 变更报告：Mods Metadata 接口新增 mode 参数

> 变更日期：2026-04-24
> 影响接口：`GET /api/v1/sync/mods/metadata`

---

## 变更内容

为 mods 元数据接口新增 `mode` 查询参数，支持按 `server`/`client`/`all` 三种模式筛选扫描范围，实现服务端必须模组与客户端推荐模组的拆分管理。

## 接口详情

### 请求

```
GET /api/v1/sync/mods/metadata?mode={mode}
```

| 参数 | 位置 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|------|--------|------|
| `mode` | query | string | 否 | `all` | 扫描模式，可选值：`server`、`client`、`all` |

**mode 取值说明：**

| 值 | 说明 | 扫描目录 |
|----|------|----------|
| `server` | 服务端必须模组 | `mods/server/` |
| `client` | 客户端推荐模组 | `mods/client/` |
| `all` | 全部模组（默认） | `mods/server/` + `mods/client/` |

### 响应

结构体无变化，与原有响应一致：

```json
{
  "code": 200,
  "message": "获取成功",
  "data": {
    "files": [
      {
        "path": "mods/server/jei-1.20.1.jar",
        "name": "jei-1.20.1.jar",
        "hash": "sha256:a1b2c3d4...",
        "size": 1048576
      }
    ],
    "total": 1,
    "scanned_at": "2026-04-24T01:00:00Z"
  }
}
```

**字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `files[].path` | string | 相对路径，如 `mods/server/xxx.jar` 或 `mods/client/xxx.jar` |
| `files[].name` | string | 文件名 |
| `files[].hash` | string | SHA-256 哈希，格式 `sha256:<hex>` |
| `files[].size` | int64 | 文件大小（字节） |
| `total` | int | 文件总数 |
| `scanned_at` | string | 扫描时间（ISO 8601） |

### 错误响应

```json
{
  "code": 400,
  "message": "mode 参数只接受 server、client 或 all"
}
```

## 变更前 vs 变更后

| 项目 | 变更前 | 变更后 |
|------|--------|--------|
| 请求参数 | 无 | 新增 `mode`（可选，默认 `all`） |
| 扫描范围 | `mods/` 顶层所有 `.jar` | `mods/server/` 和 `mods/client/` 子目录 |
| 响应 path 格式 | `mods/xxx.jar` | `mods/server/xxx.jar` 或 `mods/client/xxx.jar` |

## 兼容性说明

- **不传 `mode` 参数**：行为等同于 `mode=all`，合并扫描两个子目录
- **文件下载接口** `GET /api/v1/sync/download?path=...`：无需变更，`mods/server/` 和 `mods/client/` 路径已自动兼容

## 前端对接建议

1. 默认请求可不带 `mode` 参数获取全部模组
2. 需要分类展示时，分别请求 `?mode=server` 和 `?mode=client`
3. 下载文件时使用返回的 `path` 字段直接拼接到 `/api/v1/sync/download?path=` 即可
