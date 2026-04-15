### Code review

Found 48 issues across 4 files. All issues stem from the same root cause: **Logic layer directly imports and returns `api/*Response` types**, violating the project's layered architecture (Handler -> Logic -> Repository). Logic should return entity or internal model/DTO, not HTTP Response DTOs.

---

#### A. Logic 层结构性违规 — 导入 api 包

1. `internal/logic/library.go` 导入 `api/library` 包，Logic 层不应感知 HTTP DTO 的存在

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L15-L15

2. `internal/logic/game_profile.go` 导入 `api/user` 包，同上

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L12-L12

---

#### B. Logic 层 convert 方法直接构造 Response DTO（CRITICAL）

3. `convertSkinEntity` 返回 `*apiLibrary.SkinResponse` — 应返回 model 层 VO 或 entity

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L148-L148

4. `convertCapeEntity` 返回 `*apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L166-L166

5. `convertSkinEntities` 返回 `[]apiLibrary.SkinResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L185-L185

6. `convertCapeEntities` 返回 `[]apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L198-L198

7. `convertUserSkinAssociations` 返回 `[]apiLibrary.SkinResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L214-L214

8. `convertUserCapeAssociations` 返回 `[]apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L240-L240

9. `convertProfileEntity` 返回 `*apiUser.GameProfileResponse`，且嵌套调用 `libraryLogic.convertSkinEntity`/`convertCapeEntity` 形成三层嵌套违规（`api/user` -> `api/library` 跨包耦合）

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L456-L484

---

#### C. Logic 业务方法返回 Response DTO（CRITICAL）

##### `internal/logic/library.go` — 14 处

10. `CreateSkin` 返回 `(*apiLibrary.SkinResponse, *xError.Error)`，且参数接收 `*apiLibrary.CreateSkinRequest`（Request DTO 穿透到 Logic）

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L276-L276

11. `UpdateSkin` 返回 `(*apiLibrary.SkinResponse, *xError.Error)`，参数接收 `*apiLibrary.UpdateSkinRequest`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L344-L344

12. `ListSkins` 返回 `([]apiLibrary.SkinResponse, int64, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L437-L437

13. `ListMySkins` 返回 `([]apiLibrary.SkinResponse, int64, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L457-L457

14. `CreateCape` 返回 `(*apiLibrary.CapeResponse, *xError.Error)`，参数接收 `*apiLibrary.CreateCapeRequest`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L484-L484

15. `UpdateCape` 返回 `(*apiLibrary.CapeResponse, *xError.Error)`，参数接收 `*apiLibrary.UpdateCapeRequest`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L546-L546

16. `ListCapes` 返回 `([]apiLibrary.CapeResponse, int64, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L639-L639

17. `ListMyCapes` 返回 `([]apiLibrary.CapeResponse, int64, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L659-L659

18. `GiftSkin` 返回 `(*apiLibrary.SkinResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L697-L697

19. `GiftCape` 返回 `(*apiLibrary.CapeResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L740-L740

20. `ListUserSkins` 返回 `([]apiLibrary.SkinResponse, int64, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L791-L791

21. `ListUserCapes` 返回 `([]apiLibrary.CapeResponse, int64, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/library.go#L808-L808

##### `internal/logic/game_profile.go` — 8 处

22. `AddGameProfile` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`，含手动字段逐一赋值构建 Response

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L110-L148

23. `ChangeUsername` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`，短路路径和正常路径均手动构造 Response

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L171-L219

24. `GetGameProfileDetail` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L232-L232

25. `ListGameProfiles` 返回 `([]apiUser.GameProfileResponse, *xError.Error)`，循环中逐个构造 Response

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L263-L288

26. `EquipSkin` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L322-L322

27. `EquipCape` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L363-L363

28. `UnequipSkin` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L402-L402

29. `UnequipCape` 返回 `(*apiUser.GameProfileResponse, *xError.Error)`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/logic/game_profile.go#L428-L428

---

#### D. Handler 层职责缺失 — 透传 Logic 返回的 Response DTO（HIGH）

##### `internal/handler/library.go` — 12 处

30. `CreateSkin` 直接将 Logic 返回的 `*apiLibrary.SkinResponse` 透传给 `xResult.SuccessHasData`，未做任何转换

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L45-L51

31. `ListSkins` (market) 将 Logic 返回的 `[]apiLibrary.SkinResponse` 直接塞入 `SkinListResponse.Items`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L78-L88

32. `ListSkins` (mine) 同上

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L90-L100

33. `UpdateSkin` 透传 `*apiLibrary.SkinResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L130-L136

34. `CreateCape` 透传 `*apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L193-L199

35. `ListCapes` (market) 透传 `[]apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L226-L236

36. `ListCapes` (mine) 同上

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L238-L248

37. `UpdateCape` 透传 `*apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L278-L284

38. `GiftSkin` 透传 `*apiLibrary.SkinResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L381-L387

39. `GiftCape` 透传 `*apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L449-L455

40. `ListUserSkins` 透传 `[]apiLibrary.SkinResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L515-L525

41. `ListUserCapes` 透传 `[]apiLibrary.CapeResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/library.go#L540-L550

##### `internal/handler/game_profile.go` — 8 处

42. `AddGameProfile` 将 Logic 返回的 `*apiUser.GameProfileResponse` 直接传给 `xResult.SuccessHasData`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L48-L54

43. `ChangeUsername` 透传 `*apiUser.GameProfileResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L98-L104

44. `GetGameProfileDetail` 透传 `*apiUser.GameProfileResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L141-L147

45. `ListGameProfiles` 将 Logic 返回的 `[]apiUser.GameProfileResponse` 直接塞入 `GameProfileListResponse.Items`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L176-L186

46. `EquipSkin` 透传 `*apiUser.GameProfileResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L254-L260

47. `UnequipSkin` 透传 `*apiUser.GameProfileResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L285-L291

48. `EquipCape` 透传 `*apiUser.GameProfileResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L322-L328

49. `UnequipCape` 透传 `*apiUser.GameProfileResponse`

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/internal/handler/game_profile.go#L353-L359

---

#### E. 设计瑕疵 — api 包间耦合（MEDIUM）

50. `api/user/game_profile.go` 中 `GameProfileResponse.Skin` 和 `.Cape` 字段直接嵌入 `*apiLibrary.SkinResponse` / `*apiLibrary.CapeResponse` 类型，导致 `api/user` 包依赖 `api/library` 包，造成 DTO 层跨包耦合

https://github.com/frontleaves-mc/frontleaves-yggleaf/blob/7e90364a2bcfb7cb2f3946a983bb6cb003204b2d/api/user/game_profile.go#L7-L7

---

#### 正面参照（合规实现）

以下方法遵循了正确的分层规范，可作为修复参照：

- `DeleteSkin` / `DeleteCape`（Logic 层）: 返回 `*xError.Error`，不返回 Response DTO
- `RevokeSkin` / `RevokeCape` / `RecalculateQuota`（Logic 层）: 同上
- `GetQuota`（两个 Logic 层）: 返回 `*entity.GameProfileQuota`
- `GetQuota`（两个 Handler 层）: 直接使用 entity 传给 `xResult.SuccessHasData`

---

#### 修复方案摘要

引入 `internal/model` 层定义 VO（View Object）类型，承载包含 TextureURL 的业务数据结构：

| 步骤 | 操作 | 影响文件 |
|------|------|---------|
| P0 | Logic 层删除 `api/` 包导入 | `library.go`, `game_profile.go` |
| P0 | 所有 `convert*` 方法改为返回 `model.*VO` | 8 个 convert 方法 |
| P1 | 业务方法签名改为返回 `model.*VO` 或 entity | 18 个业务方法 |
| P1 | Request DTO 不再穿透到 Logic，Handler 拆解为基本类型 | 6 个 Handler 方法 |
| P2 | Handler 补全 VO -> Response 转换 | 20 个 Handler 方法 |
| P3 | `GameProfileResponse` 中 Skin/Cape 字段内联定义，消除 api 包间依赖 | `api/user/game_profile.go` |

---

Generated with [Claude Code](https://claude.ai/code)
