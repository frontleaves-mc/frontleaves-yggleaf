# API 权限粒度拆分变更通知

> **版本**: v1.x → v1.x (权限中间件重构)
> **日期**: 2026-04-21
> **影响范围**: 管理端 API 接口权限校验
> **前端对接**: 需根据本文档调整管理后台的权限控制逻辑与错误提示

---

## 一、变更概述

后端将原有的单一 `Admin` 权限中间件拆分为两个级别：

| 中间件 | 允许角色 | 说明 |
|--------|---------|------|
| `Admin` | `SUPER_ADMIN` + `ADMIN` | 通用管理员权限 |
| `SuperAdmin` | 仅 `SUPER_ADMIN` | 超级管理员专属权限 |

**核心变化**: 原 13 个使用 `Admin` 中间件的敏感接口已升级为 `SuperAdmin`，`ADMIN` 角色将无法再访问这些接口（返回 403）。

---

## 二、受影响接口清单

### 2.1 升级为 SuperAdmin 的接口（ADMIN 角色将收到 403）

#### 用户管理模块 (`/api/v1/admin/users`)

| 方法 | 路径 | 接口名称 | 变更前 | 变更后 |
|------|------|---------|--------|--------|
| GET | `/api/v1/admin/users` | 用户列表 | Admin ✅ | **SuperAdmin** |
| GET | `/api/v1/admin/users/:user_id` | 用户详情 | Admin ✅ | **SuperAdmin** |

#### 游戏档案管理模块 (`/api/v1/admin/game-profile`)

| 方法 | 路径 | 接口名称 | 变更前 | 变更后 |
|------|------|---------|--------|--------|
| POST | `/api/v1/admin/game-profile/users/:user_id/quota` | 调整配额 | Admin ✅ | **SuperAdmin** |

#### 资源库管理模块 (`/api/v1/library/admin`)

| 方法 | 路径 | 接口名称 | 变更前 | 变更后 |
|------|------|---------|--------|--------|
| POST | `/api/v1/library/admin/users/:user_id/skins/gift` | 赠送皮肤 | Admin ✅ | **SuperAdmin** |
| DELETE | `/api/v1/library/admin/users/:user_id/skins/:skin_library_id` | 撤销皮肤 | Admin ✅ | **SuperAdmin** |
| POST | `/api/v1/library/admin/users/:user_id/capes/gift` | 赠送披风 | Admin ✅ | **SuperAdmin** |
| DELETE | `/api/v1/library/admin/users/:user_id/capes/:cape_library_id` | 撤销披风 | Admin ✅ | **SuperAdmin** |
| POST | `/api/v1/library/admin/users/:user_id/quota/sync` | 同步配额 | Admin ✅ | **SuperAdmin** |
| GET | `/api/v1/library/admin/users/:user_id/skins` | 查询用户皮肤 | Admin ✅ | **SuperAdmin** |
| GET | `/api/v1/library/admin/users/:user_id/capes` | 查询用户披风 | Admin ✅ | **SuperAdmin** |

#### 问题类型管理模块 (`/api/v1/admin/issue-type`)

| 方法 | 路径 | 接口名称 | 变更前 | 变更后 |
|------|------|---------|--------|--------|
| POST | `/api/v1/admin/issue-type` | 创建类型 | Admin ✅ | **SuperAdmin** |
| PUT | `/api/v1/admin/issue-type/:id` | 编辑类型 | Admin ✅ | **SuperAdmin** |
| DELETE | `/api/v1/admin/issue-type/:id` | 删除类型 | Admin ✅ | **SuperAdmin** |

---

### 2.2 保持 Admin 不变的接口（ADMIN 角色仍可正常访问）

#### 工单处理模块 (`/api/v1/admin/issue`)

| 方法 | 路径 | 接口名称 | 权限 |
|------|------|---------|------|
| GET | `/api/v1/admin/issue/list` | 全量问题列表 | Admin |
| PUT | `/api/v1/admin/issue/:id/status` | 修改状态 | Admin |
| PUT | `/api/v1/admin/issue/:id/priority` | 修改优先级 | Admin |
| PUT | `/api/v1/admin/issue/:id/note` | 更新备注 | Admin |

---

### 2.3 无变化的接口（不受本次变更影响）

以下接口权限未发生任何变化：

- 所有玩家端接口（`/api/v1/user/*`, `/api/v1/game-profile/*`, `/api/v1/library/*`, `/api/v1/issue/*`）
- Yggdrasil 协议栈接口
- 公开接口（如 `GET /api/v1/issue-type/list`）

---

## 三、前端对接指南

### 3.1 权限判断逻辑调整

前端管理后台需根据当前登录用户的 `roleName` 字段控制功能可见性：

```typescript
// 角色枚举
enum Role {
  SUPER_ADMIN = 'SUPER_ADMIN',
  ADMIN = 'ADMIN',
  PLAYER = 'PLAYER'
}

// 判断是否为超管
const isSuperAdmin = userInfo.roleName === Role.SUPER_ADMIN

// 判断是否为管理员（含超管）
const isAdmin = userInfo.roleName === Role.SUPER_ADMIN || userInfo.roleName === Role.ADMIN
```

### 3.2 需要隐藏/禁用的功能（仅 SUPER_ADMIN 可见）

| 页面/模块 | 功能点 | 对应接口 |
|-----------|--------|---------|
| 用户管理 | 用户列表页、用户详情页 | `GET /admin/users*` |
| 游戏档案 | 配额调整按钮 | `POST /admin/game-profile/users/*/quota` |
| 资源库-管理 | 赠送/撤销皮肤、赠送/撤销披风 | `/library/admin/users/*/skins/*`, `/library/admin/users/*/capes/*` |
| 资源库-管理 | 配额同步按钮 | `POST /library/admin/users/*/quota/sync` |
| 资源库-管理 | 查看用户资源明细 | `GET /library/admin/users/*/skins`, `GET /library/admin/users/*/capes` |
| 工单设置 | 问题类型的增删改 | `/admin/issue-type/*` |

### 3.3 错误码处理

当 `ADMIN` 角色访问升级后的接口时，后端返回：

```json
{
  "errorCode": 403,
  "errorMessage": "需要超级管理员权限"
}
```

**建议**: 前端针对 403 错误统一处理：
- `ADMIN` 角色收到 403 时，提示"该操作需要超级管理员权限"
- 隐藏或禁用对应的功能入口（而非在操作时报错）

### 3.4 Swagger 文档更新

Swagger 文档已同步更新：
- 新增全局 Security Definition（BearerAuth）
- 超管接口 Summary 标记为 `[超管]`
- 管理员接口 Summary 标记为 `[管理]`
- 403 错误描述区分"需要超级管理员权限" / "需要管理员权限"
- 管理端接口 Tag 独立分组（`管理员-用户接口`、`管理员-游戏档案接口`、`管理员-资源库接口`、`管理员问题接口`）

---

## 四、接口速查表

| 接口路径 | 方法 | 权限级别 | 前端角色要求 |
|----------|------|---------|-------------|
| `/admin/users` | GET | SuperAdmin | SUPER_ADMIN |
| `/admin/users/:id` | GET | SuperAdmin | SUPER_ADMIN |
| `/admin/game-profile/users/:id/quota` | POST | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/skins/gift` | POST | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/skins/:id` | DELETE | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/capes/gift` | POST | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/capes/:id` | DELETE | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/quota/sync` | POST | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/skins` | GET | SuperAdmin | SUPER_ADMIN |
| `/library/admin/users/*/capes` | GET | SuperAdmin | SUPER_ADMIN |
| `/admin/issue-type` | POST | SuperAdmin | SUPER_ADMIN |
| `/admin/issue-type/:id` | PUT | SuperAdmin | SUPER_ADMIN |
| `/admin/issue-type/:id` | DELETE | SuperAdmin | SUPER_ADMIN |
| `/admin/issue/list` | GET | Admin | SUPER_ADMIN / ADMIN |
| `/admin/issue/:id/status` | PUT | Admin | SUPER_ADMIN / ADMIN |
| `/admin/issue/:id/priority` | PUT | Admin | SUPER_ADMIN / ADMIN |
| `/admin/issue/:id/note` | PUT | Admin | SUPER_ADMIN / ADMIN |

---

## 五、注意事项

1. **本次变更是破坏性变更**: 已有 `ADMIN` 角色的 token 访问上述 13 个接口时会收到 403，请确保前端同步发布
2. **建议前端先行发布**: 或前后端同时发布，避免出现 ADMIN 用户看到功能但无法操作的间隙
3. **后续新增超管接口**: 后续如有新的超管级接口会统一使用 `[超管]` 前缀标记，便于识别
