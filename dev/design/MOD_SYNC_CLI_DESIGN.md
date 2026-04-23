# MC 模组客户端同步器设计方案

> **文档版本**: v2.0 | **创建日期**: 2026-04-22 | **作者**: 筱锋

---

## 一、背景与动机

### 1.1 当前问题

MC 模组服的模组和配置文件经常需要增删改，每次更新后玩家需要手动同步才能正常进入服务器。当前流程存在以下问题：

1. **玩家无法感知更新** — 服务端增删模组后，玩家不知道自己缺少或多了哪些文件
2. **手动同步困难** — 玩家需要自行比对服务端文件列表，逐个下载或删除
3. **版本不一致导致无法进入** — 模组版本不匹配直接导致客户端崩溃或被服务端拒绝

### 1.2 设计目标

- 构建一个 **Go TUI 终端应用**，玩家运行后通过交互式引导完成同步
- 使用 **bubbletea** 框架提供渐进式引导体验，而非传统命令行参数
- 支持 **多选同步类型**（mods / config），通过空格勾选
- 通过 SHA-256 哈希对比实现 **增量同步**，只下载有变化的文件
- 服务器地址硬编码，MC 目录自动检测，无需用户配置

---

## 二、服务端 API 接口

同步器对接的服务端 API 由 `frontleaves-yggleaf` 项目提供，基础路径为 `/api/v1/sync`。

### 2.1 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| Mods 元数据 | `GET` | `/sync/mods/metadata` | 扫描服务端 mods 目录，返回所有 .jar 文件元数据 |
| Config 元数据 | `GET` | `/sync/config/metadata` | 递归扫描服务端 config 目录，返回所有文件元数据 |
| 文件下载 | `GET` | `/sync/download?path=<相对路径>` | 根据相对路径下载指定文件 |

### 2.2 元数据响应结构

两个元数据接口返回结构一致：

```json
{
  "code": 0,
  "message": "获取成功",
  "data": {
    "files": [
      {
        "path": "mods/jei-1.20.1.jar",
        "name": "jei-1.20.1.jar",
        "hash": "sha256:abc123def456...",
        "size": 1234567
      }
    ],
    "total": 42,
    "scanned_at": "2026-04-22T10:00:00Z"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `path` | string | 文件相对于 `minecraft_client/` 的相对路径 |
| `name` | string | 文件名（不含路径） |
| `hash` | string | SHA-256 哈希值，格式为 `sha256:<hex>` |
| `size` | int64 | 文件大小（字节） |
| `total` | int | 文件总数 |
| `scanned_at` | string | 服务端扫描时间（ISO 8601） |

### 2.3 文件下载接口

**请求**：

```
GET /api/v1/sync/download?path=mods/jei-1.20.1.jar
GET /api/v1/sync/download?path=config/jei/jei.cfg
```

**成功响应**：

- `Content-Type: application/octet-stream`
- `Content-Disposition: attachment; filename="<文件名>"`
- `Content-Length: <文件大小>`
- Body 为文件二进制流

**失败响应**：

```json
{ "code": 40000, "message": "非法路径" }
{ "code": 40400, "message": "文件不存在" }
```

### 2.4 安全约束

- 路径参数禁止包含 `..`（防路径遍历）
- 路径必须以 `mods/` 或 `config/` 开头
- 不暴露服务端绝对路径

---

## 三、TUI 交互设计

### 3.1 技术栈

| 库 | 用途 |
|---|---|
| `github.com/charmbracelet/bubbletea` | TUI 框架核心（Elm 架构：Model → Update → View） |
| `github.com/charmbracelet/bubbles` | 组件：list（多选列表）、spinner（加载动画）、progress（进度条） |
| `github.com/charmbracelet/lipgloss` | 样式引擎（布局、颜色、对齐） |

### 3.2 硬编码配置

```go
const (
    serverBaseURL = "https://yggleaf.frontleaves.com/api/v1"  // 服务端地址（锁定）
    mcDirName     = ".minecraft"                                 // MC 目录名（锁定）
)
```

程序必须放置在与 `.minecraft/` 同级的目录下运行。

### 3.3 引导流程

```
启动
 │
 ▼
① 欢迎界面（FrontLeaves Logo + 版本信息）
 │  按任意键继续
 ▼
② 环境检查
 │  检测当前目录下是否存在 .minecraft/
 │  不存在 → 显示错误提示，按键退出
 │  存在 → 继续下一步
 ▼
③ 选择同步类型（多选）
 │  空格勾选：☑ Mods 同步  ☑ Config 同步
 │  至少选一项，回车确认
 ▼
④ 获取元数据 + 差异计算（spinner 动画）
 │  根据选择拉取对应元数据
 │  扫描本地文件并计算哈希
 │  生成差异列表
 ▼
⑤ 预览差异
 │  展示新增/更新/删除文件列表
 │  [确认同步] / [取消] 按钮
 ▼
⑥ 执行同步（进度条）
 │  并发下载，实时进度显示
 │  文件哈希校验
 ▼
⑦ 完成界面
   统计：新增/更新/删除/失败
   按任意键退出
```

### 3.4 各步骤界面

**① 欢迎界面**

```
┌──────────────────────────────────┐
│                                  │
│     FrontLeaves 模组同步器       │
│         v1.0.0                   │
│                                  │
│   按任意键开始...                │
│                                  │
└──────────────────────────────────┘
```

居中显示项目名称和版本号，按任意键进入下一步。

**② 环境检查**

```
  检查运行环境...

  ✅ .minecraft/ 目录已找到
  ✅ mods/ 目录可用
  ✅ config/ 目录可用

  按任意键继续...
```

失败时：

```
  检查运行环境...

  ❌ 未找到 .minecraft/ 目录

  请将本程序放置在与 .minecraft/ 同级的目录下运行。

  按任意键退出...
```

**③ 选择同步类型（多选）**

使用 `bubbles/list` 多选模式，空格键勾选，回车确认：

```
  选择同步内容（空格勾选，回车确认）

  ☑ 📦 Mods 同步        同步模组文件
  ☑ 📄 Config 同步      同步配置文件

  已选择: 2 项
```

- 空格键切换勾选状态
- 至少选择一项才能按回车确认
- 选择后只拉取选中项的元数据并计算差异

**④ 获取元数据 + 差异计算**

显示 spinner 动画：

```
  正在获取服务端文件列表...  ⠋
```

根据用户选择动态拉取：只勾选 mods 则只拉取 mods 元数据，只勾选 config 则只拉取 config，两者都勾选则都拉取。

**⑤ 预览差异**

展示差异列表，底部 `[确认同步]` / `[取消]` 按钮：

```
  同步预览

  ✅ 新增文件 (3)
     mods/create-1.20.1.jar          2.1 MB
     mods/cfg-1.20.1.jar             1.8 MB
     config/jei/jei.cfg              2.1 KB

  🔄 更新文件 (2)
     mods/jei-1.20.1.jar             3.4 MB
     config/neat/neat.cfg            1.2 KB

  ❌ 将删除 (1)
     mods/old-mod-1.20.1.jar

  ⏭️ 未变更: 164 个文件

  [确认同步]  [取消]
```

- Tab 键在按钮间切换
- 回车执行当前选中按钮

**⑥ 执行同步**

使用 `bubbles/progress` 组件：

```
  正在同步...

  mods/create-1.20.1.jar          ████████████████████░░░░  82%  1.7/2.1 MB

  已完成: 1/5  |  速度: 2.3 MB/s
```

**⑦ 完成界面**

```
  同步完成！

  ✅ 新增: 3 个文件
  🔄 更新: 2 个文件
  ❌ 删除: 1 个文件
  ⚠️ 失败: 0 个文件

  按任意键退出...
```

失败时附加失败文件列表：

```
  ⚠️ 失败: 2 个文件
     mods/failed-mod-1.20.1.jar  — 网络超时
     config/error.cfg            — 校验失败
```

---

## 四、架构设计

### 4.1 项目结构

```
frontleaves-sync/
├── main.go                  # 入口，启动 TUI
├── internal/
│   ├── app.go               # 主状态机，管理所有步骤的切换
│   ├── client.go            # HTTP 客户端，封装 API 调用
│   ├── sync.go              # 同步引擎（哈希对比、文件操作）
│   ├── check.go             # 环境检查逻辑
│   └── tui/
│       ├── welcome.go       # ① 欢迎界面
│       ├── check.go         # ② 环境检查
│       ├── select.go        # ③ 同步类型多选
│       ├── diff.go          # ④⑤ 差异计算与预览
│       ├── progress.go      # ⑥ 同步进度
│       ├── done.go          # ⑦ 完成界面
│       └── style.go         # lipgloss 全局样式定义
├── go.mod
└── go.sum
```

### 4.2 状态机模型

每个界面步骤实现 bubbletea 的 `tea.Model` 接口（`Init`/`Update`/`View`），由 `app.go` 统一管理步骤切换。

```go
type step int

const (
    stepWelcome step = iota  // 欢迎界面
    stepCheck                 // 环境检查
    stepSelect                // 选择同步类型（多选）
    stepDiff                  // 差异计算 + 预览
    stepSync                  // 执行同步
    stepDone                  // 完成界面
)
```

### 4.3 步骤间数据传递

```go
type appModel struct {
    currentStep step

    // 步骤间共享数据
    syncTypes   []string          // 用户选择的同步类型 ["mods", "config"]
    remoteMods  []FileMetadata    // 服务端 mods 元数据（仅勾选 mods 时有值）
    remoteCfg   []FileMetadata    // 服务端 config 元数据（仅勾选 config 时有值）
    diffResult  *DiffResult       // 差异计算结果
    syncResult  *SyncResult       // 同步执行结果
}

type DiffResult struct {
    ToAdd     []FileMetadata  // 新增
    ToUpdate  []FileMetadata  // 更新
    ToDelete  []FileMetadata  // 删除
    Unchanged int             // 未变更数量
}

type SyncResult struct {
    Downloaded int
    Updated    int
    Deleted    int
    Failed     []FailedFile  // 失败文件及原因
}
```

### 4.4 差异计算

| 本地状态 | 服务端状态 | 操作 |
|---------|-----------|------|
| 不存在 | 存在 | 下载（新增） |
| 存在，哈希不同 | 存在 | 下载覆盖（更新） |
| 存在，哈希相同 | 存在 | 跳过（未变更） |
| 存在 | 不存在 | 删除 |

### 4.5 HTTP 客户端

```go
type SyncClient struct {
    baseURL    string       // 硬编码的服务端地址
    httpClient *http.Client
}

func (c *SyncClient) GetModsMetadata(ctx context.Context) (*SyncMetadataResponse, error)
func (c *SyncClient) GetConfigMetadata(ctx context.Context) (*SyncMetadataResponse, error)
func (c *SyncClient) DownloadFile(ctx context.Context, relPath string) (io.ReadCloser, int64, error)
```

响应解析：先解析 `code` 字段（`code == 0` 为成功），再从 `data` 提取具体数据。

### 4.6 文件操作

**下载流程**：

1. 根据 `path` 确定本地目标路径：`.minecraft/<path>`
2. `os.MkdirAll` 确保目标目录存在
3. 创建临时文件 `<目标路径>.tmp`
4. 流式写入临时文件
5. 计算临时文件 SHA-256 哈希，与服务端对比校验
6. 校验通过后原子重命名为目标文件

**删除流程**：

1. 根据差异计算得到 `toDelete` 列表
2. 逐个删除文件
3. 向上递归删除空目录

**并发下载**：

- goroutine pool，默认 4 并发
- 每个文件独立下载
- 失败不中断其他下载，最终统一报告

---

## 五、错误处理

| 阶段 | 错误场景 | 处理 |
|------|---------|------|
| 环境检查 | `.minecraft/` 不存在 | 显示错误界面，提示正确位置，按键退出 |
| 获取元数据 | 网络不可达 | 显示错误提示，按键返回选择界面或退出 |
| 获取元数据 | 服务端返回错误 | 显示服务端错误信息，按键返回 |
| 下载 | 单个文件失败 | 记录失败原因，继续下载其他文件 |
| 下载 | 哈希校验失败 | 删除临时文件，记入失败列表 |
| 下载 | 磁盘不足 | 停止下载，显示已完成和失败统计 |
