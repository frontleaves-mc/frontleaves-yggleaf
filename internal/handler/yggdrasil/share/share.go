// Package share 提供 Yggdrasil 协议中公共的 HTTP 请求处理器。
//
// 该包中的处理器负责处理以下接口：
//   - #1: GET / — API 元数据获取
//   - #11: PUT /api/user/profile/{uuid}/{textureType} — 上传材质
//   - #12: DELETE /api/user/profile/{uuid}/{textureType} — 清除材质
package share

import (
	"net/http"
	"strings"

	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic/yggdrasil"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	ygghandler "github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil"
	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	"github.com/gin-gonic/gin"
)

// ShareHandler 公共 Handler，处理 authlib-injector 和启动器共用的 Yggdrasil 接口。
//
// 嵌入 YggdrasilBase 以复用日志记录和 Yggdrasil 业务逻辑调用能力。
type ShareHandler struct {
	*ygghandler.YggdrasilBase
}

// NewShareHandler 创建公共 Handler 实例。
func NewShareHandler(base *ygghandler.YggdrasilBase) *ShareHandler {
	return &ShareHandler{YggdrasilBase: base}
}

// APIMetadata API 元数据获取
//
// @Summary     [公共] API 元数据
// @Description 由 authlib-injector 自动发现和启动器调用，返回 API 元数据（服务名称、签名公钥、皮肤域名等），authlib-injector 据此自动配置 API 位置。
// @Tags        Yggdrasil-公共接口
// @Accept      json
// @Produce     json
// @Success     200   {object}  apiYgg.MetadataResponse  "获取成功"
// @Router      / [get]
func (h *ShareHandler) APIMetadata(ctx *gin.Context) {
	h.Log.Info(ctx, "APIMetadata - 获取 API 元数据")

	resp := apiYgg.MetadataResponse{
		Meta: apiYgg.MetadataMeta{
			ServerName:            bConst.YggdrasilServerName,
			ImplementationName:    bConst.YggdrasilImplementationName,
			ImplementationVersion: bConst.YggdrasilImplementationVer,
			Links: apiYgg.MetadataLinks{
				Homepage: bConst.YggdrasilHomepageURL,
				Register: bConst.YggdrasilRegisterURL,
			},
			FeatureNonEmailLogin: true,
		},
		SkinDomains: buildSkinDomains(),
		SignaturePublickey: h.Service.Logic().GetPubKeyPEM(),
	}

	ctx.JSON(http.StatusOK, resp)
}

// UploadTexture 上传材质
//
// @Summary     [玩家] 上传材质
// @Description 由启动器调用，上传皮肤或披风材质。需通过 Bearer Token 认证，验证令牌有效且角色属于该用户。当前返回 501 功能尚未实现。
// @Tags        Yggdrasil-公共接口
// @Accept      multipart/form-data
// @Produce     json
// @Param       uuid        path   string true  "角色的无符号 UUID"
// @Param       textureType path   string true  "材质类型：skin 或 cape"
// @Param       model       formData string false "皮肤模型（slim 或空，仅 skin 类型）"
// @Param       file        formData file   true  "PNG 图片文件"
// @Param       Authorization header string true "Bearer Access Token"
// @Success     200   {object}  nil                       "上传成功"
// @Failure     400   {object}  apiYgg.YggdrasilError  "UUID 格式错误或材质类型无效"
// @Failure     401   {object}  apiYgg.YggdrasilError  "未授权或令牌无效"
// @Failure     403   {object}  apiYgg.YggdrasilError  "角色不属于该用户"
// @Failure     501   {object}  apiYgg.YggdrasilError  "功能尚未实现"
// @Router      /api/user/profile/{uuid}/{textureType} [put]
func (h *ShareHandler) UploadTexture(ctx *gin.Context) {
	h.Log.Info(ctx, "UploadTexture - 上传材质")

	// 从中间件注入的上下文中获取已验证的游戏令牌
	gameToken, ok := ctx.Value(bConst.CtxYggdrasilGameToken).(*entity.GameToken)
	if !ok || gameToken == nil {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusUnauthorized, apiYgg.ErrUnauthorized)
		return
	}

	// 获取路径参数
	uuid := ctx.Param("uuid")
	textureType := ctx.Param("textureType")

	// 预校验 UUID 格式（与 ProfileQuery/JoinServer 保持一致）
	if !yggdrasil.IsValidUnsignedUUID(uuid) {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusBadRequest, apiYgg.ErrForbidden)
		return
	}

	// 验证材质类型
	if textureType != "skin" && textureType != "cape" {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "无效的材质类型，仅支持 skin 或 cape")
		return
	}

	// 验证角色归属
	if _, ok := h.verifyOwnership(ctx, gameToken, uuid); !ok {
		return
	}

	// [SECURITY-MUST] Phase 3 实现时必须满足以下安全要求：
	//   1. 必须使用 magic number (89 50 4E 47 0D 0A 1A 0A) 或 http.DetectContentType 检测实际文件类型
	//      （不可仅信任客户端声明的 Content-Type header）
	//   2. 必须解析 PNG 并去除非位图数据（嵌入的 tEXt/iTXt/zTXt chunk 可被滥用）
	//   3. 必须验证像素尺寸并限制解压后内存（防 PNG Bomb DoS）
	//   4. 文件大小建议收紧至 256KB 以内
	//
	// 当前提前返回 501 以避免功能未实现时执行无谓的文件 I/O
	apiYgg.AbortYggError(ctx, http.StatusNotImplemented, "NotImplemented", "材质上传功能尚未实现")
}

// DeleteTexture 清除材质
//
// @Summary     [玩家] 清除材质
// @Description 由启动器调用，清除指定角色的皮肤或披风材质。需通过 Bearer Token 认证，验证令牌有效且角色属于该用户。当前返回 501 功能尚未实现。
// @Tags        Yggdrasil-公共接口
// @Accept      json
// @Produce     json
// @Param       uuid        path    string true  "角色的无符号 UUID"
// @Param       textureType path    string true  "材质类型：skin 或 cape"
// @Param       Authorization header string true "Bearer Access Token"
// @Success     204   {object}  nil                       "清除成功"
// @Failure     400   {object}  apiYgg.YggdrasilError  "UUID 格式错误或材质类型无效"
// @Failure     401   {object}  apiYgg.YggdrasilError  "未授权或令牌无效"
// @Failure     403   {object}  apiYgg.YggdrasilError  "角色不属于该用户"
// @Failure     501   {object}  apiYgg.YggdrasilError  "功能尚未实现"
// @Router      /api/user/profile/{uuid}/{textureType} [delete]
func (h *ShareHandler) DeleteTexture(ctx *gin.Context) {
	h.Log.Info(ctx, "DeleteTexture - 清除材质")

	// 从中间件注入的上下文中获取已验证的游戏令牌
	gameToken, ok := ctx.Value(bConst.CtxYggdrasilGameToken).(*entity.GameToken)
	if !ok || gameToken == nil {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusUnauthorized, apiYgg.ErrUnauthorized)
		return
	}

	// 获取路径参数
	uuid := ctx.Param("uuid")
	textureType := ctx.Param("textureType")

	// 预校验 UUID 格式（与 ProfileQuery/JoinServer 保持一致）
	if !yggdrasil.IsValidUnsignedUUID(uuid) {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusBadRequest, apiYgg.ErrForbidden)
		return
	}

	// 验证材质类型
	if textureType != "skin" && textureType != "cape" {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "无效的材质类型，仅支持 skin 或 cape")
		return
	}

	// 验证角色归属
	_, ok = h.verifyOwnership(ctx, gameToken, uuid)
	if !ok {
		return
	}

	// TODO: Phase 3 后续实现完整的材质清除流程
	// 1. 将 GameProfile 的 SkinLibraryID 或 CapeLibraryID 置为 NULL
	// 2. 通过 GameProfileRepo 更新关联

	apiYgg.AbortYggError(ctx, http.StatusNotImplemented, "NotImplemented", "材质清除功能尚未实现")
}

// verifyOwnership 验证角色是否属于令牌关联的用户。
//
// 先查询角色实体，再验证角色的 UserID 与令牌的 UserID 匹配。
// 验证失败时自动写入错误响应并中止请求。
func (h *ShareHandler) verifyOwnership(ctx *gin.Context, gameToken *entity.GameToken, unsignedUUID string) (*entity.GameProfile, bool) {
	profile, found, xErr := h.Service.Logic().VerifyProfileOwnership(ctx.Request.Context(), gameToken.UserID.Int64(), unsignedUUID)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "验证角色归属失败")
		return nil, false
	}
	if !found {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return nil, false
	}

	return profile, true
}

// buildSkinDomains 构建 skinDomains 白名单列表。
//
// 基础域名来自常量配置（主域名 + 后缀通配），额外域名通过环境变量
// YGGDRASIL_SKIN_DOMAINS_EXTRA（逗号分隔）追加。
// 这允许 beacon-bucket 等 CDN 返回的纹理 URL 域名动态加入白名单，
// 解决 Minecraft 游戏客户端严格校验 skinDomains 导致皮肤不显示的问题。
func buildSkinDomains() []string {
	domains := []string{
		bConst.YggdrasilSkinDomainMain,
		bConst.YggdrasilSkinDomainSuffix,
	}
	if extra := xEnv.GetEnvString(bConst.EnvYggdrasilSkinDomainsExtra, ""); extra != "" {
		for _, d := range strings.Split(extra, ",") {
			if trimmed := strings.TrimSpace(d); trimmed != "" {
				domains = append(domains, trimmed)
			}
		}
	}
	return domains
}
