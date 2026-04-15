# Yggdrasil 协议实现过程文档

> 本文档用于指导多代理协作开发，确保代码风格统一、顺序可迭代。

---

## 架构决策记录 (ADR)

### ADR-001: Handler 子包独立基类

- **决策**: Yggdrasil handler 使用独立的 `YggdrasilBase` 基类，不复用现有 `handler` 包的未导出类型
- **原因**: Go 子包无法访问父包的未导出类型（`handler`/`service`）；修改现有 `handler.go` 导出这些类型风险过大，可能影响已有的 UserHandler/GameProfileHandler/LibraryHandler
- **影响**: Yggdrasil handler 不走 `NewHandler[T IHandler]` 泛型，而是使用自己的 `NewYggdrasilBase()` 构造函数
- **文件**: `internal/handler/yggdrasil/base.go`

### ADR-002: 统一 YggdrasilLogic

- **决策**: 所有 12 个端点共用一个 `YggdrasilLogic`（而非按 server/client/share 拆分多个 Logic）
- **原因**: 共享 RSA 密钥对、Token CRUD、Session Cache 等基础设施，拆分反而增加状态管理复杂度
- **影响**: `logic/yggdrasil/` 包按功能文件拆分（auth.go, session.go, profile.go, texture.go, signing.go）而非按消费者类型

### ADR-003: Yggdrasil 响应格式独立

- **决策**: 所有 Yggdrasil Handler 使用 `c.JSON()` 直接输出，不经过 `xResult` 包装
- **原因**: Yggdrasil 协议要求特定的 JSON 格式（无 code/message/data 包装），与项目标准响应格式不兼容
- **影响**: 错误响应通过 `api/yggdrasil/error.go` 中的 `AbortYggError()` 统一处理

### ADR-004: User UUID 派生方案

- **决策**: 使用 UUIDv5 从 SnowflakeID 派生确定性 UUID 作为 Yggdrasil 用户 ID
- **原因**: Yggdrasil 要求 `user.id` 为无符号 UUID，但 User 实体使用 SnowflakeID。UUIDv5 无需 schema 变更，且可逆推
- **影响**: 需在 signing.go 中实现 `DeriveUserUUID(snowflakeID)` 函数

---

## 编码规范

### 通用规则
- 遵循项目现有注释风格（中文 godoc，段落式说明）
- 使用 `xError.NewError()` 构造错误
- Entity 嵌入 `xModels.BaseEntity`，实现 `GetGene()` 方法
- Repository 方法返回 `(result, found, error)` 三元组
- 使用 `pickDB(ctx, tx)` 支持事务

### Yggdrasil 特有规则
- Yggdrasil 错误响应使用 `api/yggdrasil/error.go` 中的辅助函数
- 204 No Content 响应使用 `c.Status(http.StatusNoContent)`
- 非标准 JSON 响应（metadata、profile）直接使用 `c.JSON()`
- UUID 输出使用无符号格式（去除连字符）

---

## 多代理协作规则

### 基本流程
1. 每个 Agent 开始前先读取 `dev/tracking/progress.md` 获取当前状态
2. 认领任务后更新 progress.md 状态为「🔄 进行中」
3. 完成任务后更新 progress.md 状态为「✅ 完成」
4. 发现阻塞项时更新「阻塞项」表并通知协调者

### 阶段规则
1. **严格遵循 Phase 顺序**，不跨阶段工作
2. Phase 0 内部任务可并行（无依赖）
3. Phase 1-3 之间有严格依赖关系：
   - Phase 1 依赖 Phase 0 全部完成
   - Phase 2 依赖 Phase 0 全部完成
   - Phase 3 依赖 Phase 0 全部完成
4. 每个 Agent 只修改自己负责的文件

### 共享文件协调
以下文件可能被多个 Agent 修改，需特别注意：
- `internal/app/route/route.go` — 仅修改一次（添加 yggdrasilRouter 调用）
- `internal/app/startup/startup_database.go` — 仅修改一次（添加 Token 迁移）
- `internal/app/startup/prepare/prepare.go` — 仅修改一次（添加 RSA 初始化调用）
- `internal/constant/*.go` — 各 Agent 添加各自的常量

### 冲突解决
- 若文件因冲突/重试无法修改，暂时跳过并记录到阻塞项
- 若 Prompt 持续失败，报告为「无法处理」留待后续维护
- 编译错误必须立即修复，不得累积

---

## Phase 间交接检查清单

### Phase 0 → Phase 1 交接条件
- [ ] Token 实体已创建并可编译
- [ ] 所有常量已配置
- [ ] TokenRepo 和 GameProfileYggRepo 已创建
- [ ] SessionCache 已创建
- [ ] YggdrasilLogic 基类已创建
- [ ] 签名工具（signing.go）已创建
- [ ] RSA 密钥初始化已配置
- [ ] API DTO 和错误辅助函数已创建
- [ ] Handler 基类已创建
- [ ] 认证中间件已创建
- [ ] 路由骨架已注册
- [ ] `go build ./...` 编译通过

### Phase 1 → Phase 2 交接条件
- [ ] ServerHandler 及所有服务端 Handler 已实现
- [ ] auth.go logic 方法签名已定义
- [ ] session.go logic 方法签名已定义
- [ ] `go build ./...` 编译通过

### Phase 2 → Phase 3 交接条件
- [ ] ClientHandler 及所有客户端 Handler 已实现
- [ ] 所有认证服务端点可用
- [ ] `go build ./...` 编译通过

### Phase 3 完成条件
- [ ] ShareHandler 及所有公共 Handler 已实现
- [ ] ALI 响应头已配置
- [ ] 完整登录流程可测试
- [ ] `go build ./...` 编译通过

---

## 接口与消费者对应关系

| 接口编号 | 路径 | 消费者 | Phase |
|---------|------|--------|-------|
| #1 | `GET /` | authlib-injector / 启动器 | Phase 3 (share) |
| #2 | `POST /authserver/authenticate` | 启动器 (客户端) | Phase 2 (client) |
| #3 | `POST /authserver/refresh` | 启动器 (客户端) | Phase 2 (client) |
| #4 | `POST /authserver/validate` | 启动器 (客户端) | Phase 2 (client) |
| #5 | `POST /authserver/invalidate` | 启动器 (客户端) | Phase 2 (client) |
| #6 | `POST /authserver/signout` | 启动器 (客户端) | Phase 2 (client) |
| #7 | `POST .../join` | Minecraft 客户端 | Phase 2 (client) |
| #8 | `GET .../hasJoined` | Minecraft 服务端 | Phase 1 (server) |
| #9 | `GET .../profile/{uuid}` | Minecraft 服务端/客户端 | Phase 1 (server) |
| #10 | `POST /api/profiles/minecraft` | Minecraft 服务端 | Phase 1 (server) |
| #11 | `PUT .../textureType` | 启动器 (客户端) | Phase 3 (share) |
| #12 | `DELETE .../textureType` | 启动器 (客户端) | Phase 3 (share) |
