# 管理员调整 UserGameProfile 配额

## Context

当前 UserGameProfile 配额系统硬编码默认 Total=1，管理员无法为特定用户调整配额。随着业务发展，需要管理员能够灵活调整用户的游戏档案配额（如活动奖励、特殊用户需求等）。

## 接口设计

**路由：** `POST /admin/game-profile/users/:user_id/quota`

**中间件链：** Auth → User → Admin

**请求体：**
```json
{ "delta": 3, "remark": "活动奖励" }
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| delta | int32 | 是 | 相对变化量（正数增加，负数减少，不可为 0） |
| remark | string | 否 | 备注说明，最长 255 字符 |

**响应：** 更新后的 `GameProfileQuota` 对象

## 事务流程

`GameProfileTxnRepo.AdjustQuotaAdmin` 原子事务：

1. 行锁查询目标用户配额（`SELECT ... FOR UPDATE`）
2. 计算 `newTotal = Total + delta`
3. 校验：`newTotal < Used` → 拒绝（"调整后总额度不能小于已使用额度"）
4. 校验：`newTotal < 0` → 拒绝（"调整后总额度不能小于 0"）
5. 更新 `Total` 字段
6. 直接构造并写入 `GameProfileQuotaLog`（不复用 `quotaLogRepo.Create`，因为其硬编码 `AfterTotal = beforeTotal`）

## 审计日志

复用 `GameProfileQuotaLog` 表，新增操作类型 `ObTypeAdminAdjustQuota`。

日志字段映射：
- `OpType` = `ADMIN_ADJUST_QUOTA`
- `Delta` = `|delta|`（绝对值）
- `BeforeUsed` / `AfterUsed` = 当前 Used 值（不变）
- `BeforeTotal` = 原 Total
- `AfterTotal` = 新 Total
- `Remark` = 操作者信息 + 可选备注

## 错误处理

| 场景 | 错误码 | 消息 |
|---|---|---|
| delta 为 0 | ParameterError | 无效变化量：delta 不能为 0 |
| 用户配额不存在 | ResourceNotFound | 用户游戏档案配额不存在 |
| newTotal < Used | ParameterError | 调整后总额度不能小于已使用额度 |
| newTotal < 0 | ParameterError | 调整后总额度不能小于 0 |
| 事务失败 | DatabaseError | 调整游戏档案配额失败 |

## 修改文件清单

| 文件 | 变更 |
|---|---|
| `internal/entity/type/game_profile_quota_log.go` | 新增 `ObTypeAdminAdjustQuota` 常量并注册到验证 map |
| `internal/repository/game_profile_quota.go` | 新增 `UpdateTotal` 方法（模式同 `UpdateUsed`） |
| `internal/repository/txn/game_profile.go` | 新增 `AdjustQuotaAdmin` 事务方法 |
| `internal/logic/game_profile.go` | 新增 `AdjustQuotaAdmin` 方法 |
| `internal/handler/game_profile.go` | 新增 `AdjustQuotaAdmin` handler |
| `api/user/game_profile.go` | 新增 `AdminAdjustQuotaRequest` DTO |
| `internal/app/route/route_game_profile.go` | 新增管理员路由组 |

无需新建文件、无需数据库迁移。

## 验证方式

1. 编译检查：`go build ./...`
2. Swagger 生成：`swag init`
3. 手动测试：使用管理员 Token 调用 `POST /admin/game-profile/users/:user_id/quota`，验证配额变更和日志记录
