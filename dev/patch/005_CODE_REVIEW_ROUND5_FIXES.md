# Patch 005: 代码审查第五轮修复 — JoinServer 校验、Refresh 原子绑定、中间件激活、安全加固

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：Patch 001 ~ Patch 004
> 基于审查：Yggdrasil 协议实现完整代码审查报告（6 Agent 并行审查）

---

## 一、缺陷背景

基于对 Yggdrasil 协议实现（Patch 001-004 已执行完毕）的第四轮深度审查后，
启动 **Patch 005** 修复全部 12 项缺陷（3 Critical + 6 Important + 3 Low）。

---

## 二、Critical 级别修复（3 项）

### 缺陷 C-1 [置信度: 95]：`JoinServer` 未执行令牌验证与绑定一致性检查

**文件**：`internal/logic/yggdrasil/session.go:92-118`

**改动**：在 `JoinServer` 方法入口增加完整的令牌验证链路：
1. `ValidateGameToken(ctx, accessToken)` — 验证令牌有效且未过期
2. 检查 `token.BoundProfileID != nil` — 确认令牌已绑定角色
3. `GetByID` + `EncodeUnsignedUUID` 比较 — 验证 selectedProfile 与绑定角色一致

**行为变更**：Handler 层 (`client.go:274`) 的内联验证保留作为防御纵深，Logic 层成为权威校验点。

---

### 缺陷 C-2 [置信度: 92]：`RefreshToken` 角色绑定操作在事务外执行

**文件 A**：`internal/repository/txn/game_token.go`

**改动**：
- `RevokeAndCreate` 方法签名新增 `boundProfileID *xSnowflake.SnowflakeID` 参数
- 事务内新增 Step 3：当 `boundProfileID != nil` 时，原子执行 `UpdateBoundProfile`
- 向后兼容：传入 `nil` 时行为与原方法完全一致

**文件 B**：`internal/logic/yggdrasil/auth.go:198-220`

**改动**：
- 重构角色选择逻辑：先确定 `bindProfileID`，再一次性传入 `RevokeAndCreate`
- "选择新角色"分支：`bindProfileID = &profile.ID`
- "继承原绑定"分支：`bindProfileID = oldToken.BoundProfileID`
- 绑定失败时整个事务回滚（旧令牌恢复 Valid），用户不会丢失令牌

---

### 缺陷 C-3 [置信度: 95]：Bearer Auth 中间件已实现但从未被路由使用

**文件 A**：`internal/app/route/route_yggdrasil.go`

**改动**：
1. 新增 `yggmiddleware` 导入
2. 新增 `authRequired` 路由组，挂载 `YggdrasilBearerAuth` 中间件
3. #11 (UploadTexture) 和 #12 (DeleteTexture) 移入认证路由组
4. #7 (JoinServer) 保持 body-token 方式，注释更新消除歧义
5. 补回误删的 #9 (ProfileQuery) 和 #10 (ProfilesBatchLookup) 路由

**文件 B**：`internal/handler/yggdrasil/share/share.go`

**改动**：
1. `UploadTexture` / `DeleteTexture` 改为从 `context.Value(CtxYggdrasilGameToken)` 读取已验证令牌
2. 删除死代码 `validateBearerAuth()` 函数及其 `strings` 包导入
3. `verifyOwnership()` 保留供未来使用

---

## 三、Important 级别修复（6 项）

### 缺陷 I-1 [置信度: 88]：`GetByEmail/GetByPhone` 缺少 `tx *gorm.DB` 参数

**文件**：`internal/repository/user.go`

**改动**：
- 两方法签名新增 `tx *gorm.DB` 参数
- 方法体内改用 `r.pickDB(ctx, tx)` 替代 `r.db.WithContext(ctx)`
- 新增 `pickDB(ctx, tx)` 辅助方法（与 GameTokenRepo 模式一致）

**联动修改**：`internal/logic/yggdrasil/auth.go` 中两处调用点补充 `nil` 参数

---

### 缺陷 I-2 [置信度: 85]：认证接口时序侧信道攻击风险

**文件**：`internal/logic/yggdrasil/auth.go:85,301`

**改动**：`AuthenticateUser` 和 `SignoutUser` 中用户不存在分支增加 dummy bcrypt 比较：
```go
_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashvaluefortimingattackprevention"), []byte(password))
```
恒定化响应时间，防止通过响应时间差枚举账号存在性。

---

### 缺陷 I-3 [置信度: 83]：`BuildProfileResponse` 中 `json.Marshal` 错误被静默忽略

**文件**：`internal/logic/yggdrasil/session.go:137-143`

**改动**：
- `value` 变量提前声明为 `var value string`
- `json.Marshal` 错误增加 `l.log.Error(...)` 日志记录
- 序列化失败时 `value = ""`（空 Base64 而非 panic）

---

### 缺陷 I-4 [置信度: 82]：`InvalidateToken` DB 故障时日志级别应为 Error

**文件**：`internal/handler/yggdrasil/client/client.go:222`

**改动**：`h.Log.Warn` → `h.Log.Error`，确保数据库故障触发告警通道。

---

### 缺陷 I-5 [置信度: 85]：RSA 密钥对为 nil 时缺少防御性 Panic 检查

**文件**：`internal/logic/yggdrasil/logic.go:95-97`

**改动**：提取密钥对后增加 nil/空检查，不满足时调用 `xLog.Panic(...)` 使启动过程明确失败而非运行时 panic。

---

### 缺陷 I-6 [置信度: 82]：`BatchLookupProfiles` 未实施名称数量上限校验

**文件**：`internal/logic/yggdrasil/profile.go:53-59`

**改动**：
- 新增 `bConst` 导入
- 方法入口增加 `len(names) > bConst.YggdrasilTokenMaxPerUser` 截断逻辑
- 与 Handler 层限制一致（防御性编程，双层保护）

---

## 四、Low 级别修复（3 项）

### 缺陷 L-1 [置信度: 82]：`os.Stat` 非 IsNotExist 错误被静默忽略

**文件**：`internal/app/startup/prepare/prepare_rsa.go:33-41`

**改动**：增加 `else` 分支返回 `fmt.Errorf("检查密钥文件状态失败: %w", err)`。

### 缺陷 L-2 [置信度: 80]：`pem.Decode` 剩余数据被丢弃

**文件**：`internal/app/startup/prepare/prepare_rsa.go:114-119`

**改动**：检查 `len(rest) > 0` 时返回"私钥文件包含意外额外数据"错误。

### 缺陷 L-3 [置信度: 60]：规范文档 Gene 编号过时

**文件**：`dev/markdown/YGGDRASIL_SPECIFICATION.md:661`

**改动**：Gene 编号建议从 `38` 更新为 `40`。

---

## 五、验证标准

- [x] `go build ./...` 编译通过（零错误零警告）
- [x] `JoinServer` 包含 ValidateGameToken + BoundProfileID 一致性检查
- [x] `RevokeAndCreate` 签名含 `boundProfileID` 参数，事务内包含绑定步骤
- [x] 路由层 `authRequired` 组挂载了 `YggdrasilBearerAuth` 中间件
- [x] `share.go` UploadTexture/DeleteTexture 从 context 读取令牌
- [x] `validateBearerAuth()` 死代码已删除
- [x] `GetByEmail/GetByPhone` 签名含 `tx *gorm.DB` + 使用 `pickDB`
- [x] 认证接口用户不存在时执行 dummy bcrypt 比较
- [x] `BuildProfileResponse` Marshal 失败有 Error 日志
- [x] Invalidate 日志级别为 Error
- [x] `NewYggdrasilLogic` 有 RSA 密钥 nil Panic 防御
- [x] `BatchLookupProfiles` 有名称数量上限校验
- [x] `os.Stat` 非 IsNotExist 错误显式处理
- [x] `pem.Decode` 检查剩余数据
- [x] 规范文档 Gene 编号为 40
