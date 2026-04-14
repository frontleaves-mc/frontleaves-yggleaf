# Library 系统重构设计方案

> **文档版本**: v1.0 | **创建日期**: 2026-04-15 | **作者**: 筱锋

---

## 一、背景与动机

### 1.1 当前架构问题

当前 Library 系统中，用户上传皮肤/披风后直接通过 `SkinLibrary.UserID` / `CapeLibrary.UserID` 标记归属，上传即拥有、即可使用。这种设计存在以下问题：

1. **缺少库与用户的关联层** — 用户与资源之间没有显式的"拥有"关系表，无法准确区分"谁上传的"与"谁拥有的"
2. **Quota 计算不准确** — 由于缺少关联层，`LibraryQuota` 的 Used 字段无法精确统计用户实际占用的资源数量
3. **无法支持管理员赠送** — 无法让管理员上传皮肤/披风并指定给特定用户使用，赠送的资源也不应计入用户配额

### 1.2 设计目标

- 引入 **用户资源库关联表**，明确资源归属关系
- 通过 `AssignmentType` 区分资源来源，精确控制配额计算
- 保持 `GameProfile` 直接引用 `SkinLibrary`/`CapeLibrary` 的设计不变
- 最小化对现有 API 的破坏性变更

---

## 二、核心概念

### 2.1 AssignmentType（关联类型）

```go
type AssignmentType uint8

const (
    AssignmentTypeNormal AssignmentType = 1 // 用户自主上传，计入配额
    AssignmentTypeGift   AssignmentType = 2 // 管理员赠送，不计入配额
    AssignmentTypeAdmin  AssignmentType = 3 // 系统预置/管理员分配，不计入配额
)
```

| 类型 | 值 | 来源 | 配额影响 | 说明 |
|------|---|------|---------|------|
| `Normal` | 1 | 用户自行上传 | **计入** quota | 占用用户的公开/私有配额 |
| `Gift` | 2 | 管理员赠送 | **不计入** quota | 管理员指定用户获得 |
| `Admin` | 3 | 系统预置 | **不计入** quota | 管理员上传并分配给用户 |

### 2.2 核心语义变更

| 概念 | 变更前 | 变更后 |
|------|--------|--------|
| `SkinLibrary.UserID` | 资源拥有者 | 资源**创建者/上传者**（nil = 系统内置或管理员上传） |
| 资源归属 | 由 `SkinLibrary.UserID` 直接决定 | 由 `UserSkinLibrary` 关联表决定 |
| Quota 计算依据 | `SkinLibrary` 表的记录数 | `UserSkinLibrary(AssignmentType=Normal)` 的记录数 |

---

## 三、实体设计

### 3.1 新增实体

#### 3.1.1 AssignmentType 枚举

**文件**: `internal/entity/type/assignment_type.go`

```go
package entity

// AssignmentType 资源关联类型，标识用户与资源的关联方式及配额影响规则。
type AssignmentType uint8

const (
    // AssignmentTypeNormal 用户自主上传的资源，计入配额消耗。
    AssignmentTypeNormal AssignmentType = 1

    // AssignmentTypeGift 管理员赠送的资源，不计入配额消耗。
    AssignmentTypeGift AssignmentType = 2

    // AssignmentTypeAdmin 系统预置/管理员分配的资源，不计入配额消耗。
    AssignmentTypeAdmin AssignmentType = 3
)

// IsValid 校验关联类型是否为合法值。
func (t AssignmentType) IsValid() bool {
    return t == AssignmentTypeNormal || t == AssignmentTypeGift || t == AssignmentTypeAdmin
}

// CountsTowardQuota 判断该类型是否计入用户配额。
// 只有 AssignmentTypeNormal 类型的关联才计入配额消耗。
func (t AssignmentType) CountsTowardQuota() bool {
    return t == AssignmentTypeNormal
}
```

#### 3.1.2 UserSkinLibrary（用户皮肤关联表）

**文件**: `internal/entity/user_skin_library.go`

```go
package entity

// UserSkinLibrary 用户皮肤关联实体，记录用户对皮肤资源的拥有关系。
//
// 该实体是 SkinLibrary（资源定义）与 User（所有者）之间的关联表，
// 通过 AssignmentType 区分资源来源并决定配额计算行为：
//   - normal：用户自行上传，计入配额
//   - gift：管理员赠送，不计入配额
//   - admin：系统预置，不计入配额
type UserSkinLibrary struct {
    xModels.BaseEntity                                              // 嵌入基础实体字段
    UserID         xSnowflake.SnowflakeID  `gorm:"not null;index:idx_user_skin_library_user_id;comment:关联用户ID" json:"user_id"`                                      // 关联用户ID
    SkinLibraryID  xSnowflake.SnowflakeID  `gorm:"not null;uniqueIndex:uk_user_skin_library_user_skin;comment:关联皮肤库ID" json:"skin_library_id"`                       // 关联皮肤库ID
    AssignmentType AssignmentType            `gorm:"not null;type:smallint;default:1;comment:关联类型(1=normal,2=gift,3=admin)" json:"assignment_type"` // 关联类型

    // ----------
    //  外键约束
    // ----------
    User        *User         `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`                                               // 关联用户
    SkinLibrary *SkinLibrary  `gorm:"foreignKey:SkinLibraryID;references:ID;constraint:OnDelete:CASCADE;comment:关联皮肤库" json:"skin_library,omitempty"` // 关联皮肤库
}
```

**唯一约束**: `uk_user_skin_library_user_skin(user_id, skin_library_id)` — 防止同一用户重复关联同一皮肤。

#### 3.1.3 UserCapeLibrary（用户披风关联表）

**文件**: `internal/entity/user_cape_library.go`

与 `UserSkinLibrary` 同构，`SkinLibraryID` 替换为 `CapeLibraryID`，唯一约束为 `uk_user_cape_library_user_cape(user_id, cape_library_id)`。

### 3.2 变更的实体

#### 3.2.1 User 实体

**文件**: `internal/entity/user.go`

新增两个关联字段：

```go
// 新增（在 LibraryQuotas 字段之后）
UserSkinLibraries []UserSkinLibrary `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;comment:用户皮肤关联" json:"user_skin_libraries,omitempty"` // 用户皮肤关联
UserCapeLibraries []UserCapeLibrary `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;comment:用户披风关联" json:"user_cape_libraries,omitempty"` // 用户披风关联
```

现有的 `SkinLibraries []SkinLibrary` 和 `CapeLibraries []CapeLibrary` 字段**保留**，注释标记为 deprecated，过渡期共存。

#### 3.2.2 SkinLibrary / CapeLibrary

**仅注释变更**: `UserID` 字段的语义从"拥有者"变为"创建者/上传者"。

```
// Before: comment:关联用户ID(为空代表系统内置皮肤)
// After:  comment:创建者/上传者用户ID(为空代表系统内置或管理员上传)
```

#### 3.2.3 LibraryQuota

**结构不变**。配额字段 (`SkinsPublicUsed` 等) 仍然由 Txn 层的 increment/decrement 操作维护，只是数据来源从 `SkinLibrary` 表切换到 `UserSkinLibrary(AssignmentType=Normal)` 关联。

#### 3.2.4 GameProfile

**结构不变**。`SkinLibraryID` 和 `CapeLibraryID` 仍然直接引用 `SkinLibrary`/`CapeLibrary`。

新增 **装备校验逻辑**（Logic 层）：装备时需验证当前用户在 `UserSkinLibrary`/`UserCapeLibrary` 中存在对应关联记录。

### 3.3 Gene 编号分配

**文件**: `internal/constant/gene_number.go`

```go
const (
    GeneForGameProfile         xSnowflake.Gene = 32  // 游戏档案
    GeneForGameProfileQuota    xSnowflake.Gene = 33  // 游戏档案配额
    GeneForGameProfileQuotaLog xSnowflake.Gene = 34  // 游戏档案配额日志
    GeneForSkinLibrary         xSnowflake.Gene = 35  // 皮肤库
    GeneForCapeLibrary         xSnowflake.Gene = 36  // 披风库
    GeneForLibraryQuota        xSnowflake.Gene = 37  // 资源库配额
    GeneForUserSkinLibrary     xSnowflake.Gene = 38  // 用户皮肤关联    ← 新增
    GeneForUserCapeLibrary     xSnowflake.Gene = 39  // 用户披风关联    ← 新增
    GeneForToken               xSnowflake.Gene = 40  // 令牌 (原计划 38，后移)
)
```

### 3.4 AutoMigrate 注册

**文件**: `internal/app/startup/startup_database.go`

新增 2 个实体到 `migrateTables`：

```go
var migrateTables = []interface{}{
    // ...existing...
    &entity.UserSkinLibrary{},  // 新增
    &entity.UserCapeLibrary{},  // 新增
}
```

---

## 四、实体关系图

```
                          +------------------+
                          |      User        |
                          |------------------|
                          | ID (PK)          |
                          | Username         |
                          +--------+---------+
                                   |
           +-----------------------+-----------------------+-------------------+
           |                       |                       |                   |
           v                       v                       v                   v
+----------+-----------+ +--------+---------+ +------------+---------+ +-------+--------+
|   SkinLibrary        | |   CapeLibrary     | |   LibraryQuota       | |  UserSkin      |
|----------------------| |-------------------| |-----------------------| |  Library       |
| ID (PK)              | | ID (PK)           | | ID (PK)               | |----------------|
| UserID (FK->User)*   | | UserID (FK->User)*| | UserID (UK, FK->User) | | ID (PK)        |
| Name                 | | Name              | | SkinsPublicTotal      | | UserID (FK)    |
| Texture              | | Texture           | | SkinsPrivateTotal     | | SkinLibID (FK) |
| TextureHash (UQ)     | | TextureHash (UQ)  | | SkinsPublicUsed  ←----+-| AssignmentType |
| Model                | | IsPublic          | | SkinsPrivateUsed      |
| IsPublic             | +--------+----------+ | CapesPublicTotal      |
+----------+-----------+          |            | CapesPrivateTotal     |
           |                      |            | CapesPublicUsed       |
           | 1                    | 1          | CapesPrivateUsed      |
           v                      v            +-----------------------+
+----------+-----------+ +--------+---------+
|  UserSkinLibrary     | |  UserCapeLibrary |  Quota 只统计
|----------------------| |-------------------|  AssignmentType
| ID (PK)              | | ID (PK)           |  == Normal (1)
| UserID (FK->User)    | | UserID (FK->User)  |  的关联记录
| SkinLibraryID (FK)   | | CapeLibraryID(FK)  |
| (UK: UserID+SkinID)  | | (UK: UserID+CapeID)|
| AssignmentType       | | AssignmentType     |
+----------+-----------+ +--------+---------+

**GameProfile 关系（不变）**:

```
GameProfile.SkinLibraryID  ──SET NULL──>  SkinLibrary
GameProfile.CapeLibraryID  ──SET NULL──>  CapeLibrary
```

装备时新增校验：`UserSkinLibrary(UserID=current, SkinLibraryID=target)` 必须存在。

---

## 五、配额计算逻辑

### 5.1 核心规则

**只有 `AssignmentType == Normal (1)` 的关联记录计入配额。** Quota 的 Used 字段与 `SkinLibrary.IsPublic` 配合确定使用公开还是私有配额桶。

计算公式：

```sql
-- 公开皮肤已用 = 用户关联的 normal 类型 + 皮肤 IsPublic=true 的数量
SELECT COUNT(*) FROM user_skin_library usl
JOIN skin_library sl ON usl.skin_library_id = sl.id
WHERE usl.user_id = ? AND usl.assignment_type = 1 AND sl.is_public = true;

-- 私有皮肤已用 = 用户关联的 normal 类型 + 皮肤 IsPublic=false 的数量
SELECT COUNT(*) FROM user_skin_library usl
JOIN skin_library sl ON usl.skin_library_id = sl.id
WHERE usl.user_id = ? AND usl.assignment_type = 1 AND sl.is_public = false;
```

> **注意**: 实际代码中，Quota 的 Used 字段仍由 Txn 层通过 increment/decrement 维护（非实时 COUNT），上述 SQL 仅用于重算/校准。

### 5.2 用户上传皮肤流程（变更后）

```
Handler.CreateSkin
  └─> Logic.CreateSkin
        ├─ 校验参数（名称、模型、纹理）
        ├─ 解码 Base64 + 计算 SHA256 哈希
        ├─ 上传纹理到 Bucket（事务外）
        ├─ 构建 SkinLibrary（UserID = 创建者）
        └─> TxnRepo.CreateSkinWithQuota(skin, userID)
              │
              TRANSACTION BEGIN
              ├─ 1. 纹理哈希去重检查（SkinLibrary 表，不变）
              ├─ 2. 创建 SkinLibrary 记录（资源定义）
              ├─ 3. 获取配额行锁 FOR UPDATE
              ├─ 4. 校验配额是否充足（IsPublic → Public 桶 / !IsPublic → Private 桶）
              ├─ 5. 创建 UserSkinLibrary(AssignmentType=Normal) 记录
              ├─ 6. 递增对应配额 Used 计数
              TRANSACTION COMMIT
```

### 5.3 用户删除皮肤流程（变更后）

```
TxnRepo.DeleteSkinWithQuota(userID, skinID)
  │
  TRANSACTION BEGIN
  ├─ 1. 查找 UserSkinLibrary(UserID, SkinLibraryID)
  ├─ 2. IF AssignmentType == Normal:
  │     ├─ 获取配额行锁 FOR UPDATE
  │     └─ 递减对应配额 Used 计数（依据 SkinLibrary.IsPublic）
  ├─ 3. 删除 UserSkinLibrary 记录
  ├─ 4. 检查 SkinLibrary 是否还有其他 UserSkinLibrary 引用
  │     ├─ 无引用 AND SkinLibrary.UserID == 当前用户 → 删除 SkinLibrary（清理孤立资源）
  │     └─ 有引用 → 保留 SkinLibrary（其他用户可能仍在使用）
  TRANSACTION COMMIT
```

### 5.4 管理员赠送皮肤流程（新增）

```
Handler.GiftSkin ──> Logic.GiftSkin ──> TxnRepo.GiftSkinToUser(operatorID, targetUserID, skinLibraryID, ...)
  │
  TRANSACTION BEGIN
  ├─ 1. 校验 SkinLibrary 存在
  ├─ 2. 校验 UserSkinLibrary(user_id, skin_library_id) 不存在（防重复赠送）
  ├─ 3. 创建 UserSkinLibrary(AssignmentType=Gift)   ← 不检查配额，不递增 Used
  TRANSACTION COMMIT
```

### 5.5 管理员撤销赠送流程（新增）

```
TxnRepo.RevokeSkinFromUser(targetUserID, skinLibraryID)
  │
  TRANSACTION BEGIN
  ├─ 1. 查找 UserSkinLibrary(UserID, SkinLibraryID)
  ├─ 2. 校验 AssignmentType != Normal（不能撤销用户自主上传的资源）
  ├─ 3. 删除 UserSkinLibrary 记录   ← 不操作配额（gift/admin 不计入）
  TRANSACTION COMMIT
```

### 5.6 Quota 校准方法（新增）

```go
// RecalculateQuota 从关联表重新计算并同步配额已用值（管理员操作）
func (t *LibraryTxnRepo) RecalculateQuota(ctx context.Context, userID xSnowflake.SnowflakeID) (*entity.LibraryQuota, *xError.Error)
```

基于 `UserSkinLibrary(AssignmentType=Normal) JOIN SkinLibrary` 重新计算四个 Used 字段，用于数据修复。

---

## 六、索引设计

### 6.1 UserSkinLibrary

| 索引名 | 类型 | 列 | 用途 |
|--------|------|----|------|
| `PRIMARY` | PK | `id` | Snowflake 主键 |
| `uk_user_skin_library_user_skin` | UNIQUE | `(user_id, skin_library_id)` | 防止重复关联 |
| `idx_user_skin_library_user_id` | INDEX | `user_id` | 按用户查询关联列表 |

### 6.2 UserCapeLibrary

同构于 UserSkinLibrary，列名对应替换。

---

## 七、API 变更

### 7.1 现有端点（行为变更，签名不变）

| 方法 | 路径 | 行为变更 |
|------|------|---------|
| `POST` | `/library/skins` | 创建 SkinLibrary 后，额外创建 `UserSkinLibrary(AssignmentType=Normal)` |
| `GET` | `/library/skins?mode=mine` | 查询 `UserSkinLibrary JOIN SkinLibrary` 替代 `SkinLibrary WHERE user_id` |
| `PATCH` | `/library/skins/:skin_id` | 校验通过 `UserSkinLibrary` 进行归属权验证 |
| `DELETE` | `/library/skins/:skin_id` | 删除 `UserSkinLibrary`，按条件清理孤立 `SkinLibrary` |
| `POST` | `/library/capes` | 同皮肤创建逻辑 |
| `GET` | `/library/capes?mode=mine` | 同皮肤列表逻辑 |
| `PATCH` | `/library/capes/:cape_id` | 同皮肤更新逻辑 |
| `DELETE` | `/library/capes/:cape_id` | 同皮肤删除逻辑 |
| `GET` | `/library/quota` | **无变更**（配额结构不变） |

### 7.2 新增端点（管理员操作）

| 方法 | 路径 | 功能 | 说明 |
|------|------|------|------|
| `POST` | `/library/admin/users/:user_id/skins/gift` | 赠送皮肤给用户 | 需管理员权限，skin_library_id 在请求体中 |
| `POST` | `/library/admin/users/:user_id/capes/gift` | 赠送披风给用户 | 需管理员权限，cape_library_id 在请求体中 |
| `DELETE` | `/library/admin/users/:user_id/skins/:skin_library_id` | 撤销用户皮肤关联 | 仅可撤销 gift/admin 类型 |
| `DELETE` | `/library/admin/users/:user_id/capes/:cape_library_id` | 撤销用户披风关联 | 仅可撤销 gift/admin 类型 |
| `POST` | `/library/admin/users/:user_id/quota/sync` | 配额重算同步 | 数据修复用 |
| `GET` | `/library/admin/users/:user_id/skins` | 查询用户皮肤关联（含所有类型） | 管理视角 |
| `GET` | `/library/admin/users/:user_id/capes` | 查询用户披风关联（含所有类型） | 管理视角 |

---

## 八、各层影响分析

### 8.1 Entity 层

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `entity/type/assignment_type.go` | **新增** | AssignmentType 枚举定义 |
| `entity/user_skin_library.go` | **新增** | UserSkinLibrary 实体 |
| `entity/user_cape_library.go` | **新增** | UserCapeLibrary 实体 |
| `entity/user.go` | **修改** | 新增 `UserSkinLibraries` + `UserCapeLibraries` 关联字段 |
| `entity/skin_library.go` | **轻微** | `UserID` 注释语义变更 |
| `entity/cape_library.go` | **轻微** | `UserID` 注释语义变更 |
| `constant/gene_number.go` | **修改** | 新增 Gene 38-39，Token 后移至 40 |
| `app/startup/startup_database.go` | **修改** | 新增 2 个实体到 migrateTables |

### 8.2 Repository 层

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `repository/user_skin_library.go` | **新增** | CRUD + `ExistsByUserAndSkin` + `CountNormalByUserAndVisibility` + `ListByUserID` + `CountReferences` |
| `repository/user_cape_library.go` | **新增** | 同构于 user_skin_library |
| `repository/library_quota.go` | **修改** | 新增 `UpdateAllUsed` 方法 |

### 8.3 Txn 层

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `repository/txn/library.go` | **重构** | 新增 2 个 repo 字段；修改 `Create/Update/Delete` 方法；新增 `GiftSkinToUser` / `RevokeSkinFromUser` / `RecalculateQuota` 等方法 |

### 8.4 Logic 层

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `logic/library.go` | **重构** | 新增 repo 字段；修改 `Create/Update/Delete/List` 方法路由到关联表；新增 `GiftSkin` / `RevokeSkin` / `SyncQuota` 等方法 |
| `logic/game_profile.go` | **修改** | 装备时新增关联校验（调用 `userSkinRepo.ExistsByUserAndSkin`） |

### 8.5 Handler 层

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `handler/library.go` | **修改** | 新增 ~8 个管理员 Handler 方法 |
| `api/library/gift.go` | **新增** | 赠送 API 请求结构体 |
| `api/library/skin.go` | **修改** | 响应中增加 `assignment_type` 字段 |
| `app/route/route_library.go` | **修改** | 新增管理员路由组 |

---

## 九、数据迁移策略

### 9.1 迁移步骤

```
Phase 0: Schema 变更（AutoMigrate 创建 2 张新空表）
Phase 1: 数据回填（一次性 SQL 脚本）
Phase 2: 双写期（新旧代码路径并存）
Phase 3: 切换（全面切换到新代码路径）
Phase 4: 清理（移除 deprecated 字段/方法）
```

### 9.2 回填 SQL 脚本

```sql
-- ============================================================
--  回填 UserSkinLibrary：将现有 SkinLibrary 的直接归属转为关联记录
-- ============================================================
INSERT INTO fyl_user_skin_library (id, user_id, skin_library_id, assignment_type, created_at, updated_at)
SELECT
    sl.id AS id,
    sl.user_id,
    sl.id AS skin_library_id,
    1 AS assignment_type,    -- AssignmentTypeNormal
    sl.created_at,
    sl.updated_at
FROM fyl_skin_library sl
WHERE sl.user_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM fyl_user_skin_library usl
    WHERE usl.skin_library_id = sl.id AND usl.user_id = sl.user_id
  );

-- ============================================================
--  回填 UserCapeLibrary（同构）
-- ============================================================
INSERT INTO fyl_user_cape_library (id, user_id, cape_library_id, assignment_type, created_at, updated_at)
SELECT
    cl.id AS id,
    cl.user_id,
    cl.id AS cape_library_id,
    1 AS assignment_type,
    cl.created_at,
    cl.updated_at
FROM fyl_cape_library cl
WHERE cl.user_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM fyl_user_cape_library ucl
    WHERE ucl.cape_library_id = cl.id AND ucl.user_id = cl.user_id
  );

-- ============================================================
--  校验脚本：检查旧计数与新关联计数一致性
-- ============================================================
SELECT
    q.user_id,
    q.skins_public_used AS old_public,
    COALESCE(new_cnt.public_count, 0) AS new_public,
    q.skins_private_used AS old_private,
    COALESCE(new_cnt.private_count, 0) AS new_private
FROM fyl_library_quota q
LEFT JOIN (
    SELECT
        usl.user_id,
        SUM(CASE WHEN sl.is_public = true THEN 1 ELSE 0 END) AS public_count,
        SUM(CASE WHEN sl.is_public = false THEN 1 ELSE 0 END) AS private_count
    FROM fyl_user_skin_library usl
    JOIN fyl_skin_library sl ON usl.skin_library_id = sl.id
    WHERE usl.assignment_type = 1
    GROUP BY usl.user_id
) new_cnt ON q.user_id = new_cnt.user_id
WHERE q.skins_public_used != COALESCE(new_cnt.public_count, 0)
   OR q.skins_private_used != COALESCE(new_cnt.private_count, 0);
```

> **ID 复用说明**: 回填时复用 `SkinLibrary`/`CapeLibrary` 的 Snowflake ID 作为关联表 ID，这在跨表场景下是安全的（不同表有独立的 ID 命名空间），且便于追溯。

---

## 十、关键业务规则

### 10.1 删除资源时的清理策略

当用户删除自己上传的皮肤时：
1. 删除 `UserSkinLibrary` 记录
2. 释放配额（如果是 Normal 类型）
3. 检查该 `SkinLibrary` 是否还有其他用户的关联记录
   - **无其他引用** → 可以安全删除 `SkinLibrary` 记录（孤立资源清理）
   - **有其他引用** → 保留 `SkinLibrary`（其他用户仍通过 gift/admin 关联使用）

### 10.2 管理员上传与赠送的关系

```
管理员上传皮肤 → SkinLibrary(UserID=nil, IsPublic=false)  // 不公开在市场
管理员赠送皮肤 → UserSkinLibrary(TargetUserID, SkinLibraryID, AssignmentType=Gift)
```

赠送的皮肤不会出现在公开市场（`SkinLibrary.IsPublic=false`），只有被赠送的用户可以在自己的库中看到并使用。

### 10.3 错误处理

| 场景 | 错误码 | 信息 |
|------|--------|------|
| 重复赠送同一皮肤 | 409 DataConflict | "该用户已拥有此皮肤" |
| 撤销用户自主上传的资源 | 403 PermissionDenied | "无法撤销用户自主上传的资源" |
| 装备未拥有的皮肤 | 403 PermissionDenied | "您未拥有该皮肤，无法装备" |
| 无效的 AssignmentType | 400 ParameterError | "无效的关联类型" |
| 赠送给自己 | 400 ParameterError | "不能向自己赠送资源" |
| 赠送的皮肤不存在 | 404 ResourceNotFound | "皮肤资源不存在" |

### 10.4 并发安全

- 所有配额修改操作继续使用 `SELECT ... FOR UPDATE` 悲观锁
- `uk_user_skin_library_user_skin` 唯一约束防止竞态条件下的重复关联
- 赠送操作先检查存在性再插入，唯一约束作为最终保障

---

## 十一、文件变更清单

### 新增文件（5 个）

| 路径 | 用途 |
|------|------|
| `internal/entity/type/assignment_type.go` | AssignmentType 枚举 |
| `internal/entity/user_skin_library.go` | UserSkinLibrary 实体 |
| `internal/entity/user_cape_library.go` | UserCapeLibrary 实体 |
| `internal/repository/user_skin_library.go` | UserSkinLibrary 仓储 |
| `internal/repository/user_cape_library.go` | UserCapeLibrary 仓储 |

### 修改文件（~10 个）

| 路径 | 变更说明 |
|------|---------|
| `internal/constant/gene_number.go` | 新增 Gene 38-39，Token 后移至 40 |
| `internal/entity/user.go` | 新增 2 个关联字段 |
| `internal/entity/skin_library.go` | UserID 注释语义变更 |
| `internal/entity/cape_library.go` | UserID 注释语义变更 |
| `internal/app/startup/startup_database.go` | 新增 2 个实体到 migrateTables |
| `internal/repository/library_quota.go` | 新增 UpdateAllUsed 方法 |
| `internal/repository/txn/library.go` | 重构：新增 repo 字段、修改 6 个方法、新增 5 个方法 |
| `internal/logic/library.go` | 重构：新增 repo 字段、修改 8 个方法、新增 5 个方法 |
| `internal/logic/game_profile.go` | 新增装备时关联校验 |
| `internal/handler/library.go` | 新增管理员 Handler 方法 |
| `api/library/gift.go` | 管理员赠送 API 请求结构体 |
| `api/library/cape.go` | 响应增加 AssignmentType 字段 |
| `internal/app/route/route_library.go` | 新增管理员路由组 |

---

## 十二、实施阶段建议

| 阶段 | 内容 | 风险 |
|------|------|------|
| **Phase 1** | Entity + Gene + AutoMigrate | 低 — 仅新增表，不影响现有功能 |
| **Phase 2** | Repository 层 | 低 — 独立的新文件 |
| **Phase 3** | Txn 层重构 | 中 — 修改核心事务逻辑 |
| **Phase 4** | Logic 层重构 | 中 — 修改业务流程 |
| **Phase 5** | API + Handler + Route | 低 — 新增管理员端点 |
| **Phase 6** | 数据迁移 | 高 — 需在测试环境充分验证 |
| **Phase 7** | 测试与校验 | — — 覆盖所有流程 |
