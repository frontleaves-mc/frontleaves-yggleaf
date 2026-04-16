// Package server 提供 Yggdrasil 协议中由 Minecraft 服务端消费的 HTTP 请求处理器。
//
// 该包中的处理器负责处理以下接口：
//   - #8: GET /sessionserver/session/minecraft/hasJoined — 服务端验证客户端
//   - #9: GET /sessionserver/session/minecraft/profile/{uuid} — 查询角色属性
//   - #10: POST /api/profiles/minecraft — 按名称批量查询角色
package server

import (
	"fmt"
	"net/http"
	"strconv"

	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic/yggdrasil"
	ygghandler "github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil"
	"github.com/gin-gonic/gin"
)

// ServerHandler 服务端 Handler，处理 Minecraft 服务端消费的 Yggdrasil 接口。
//
// 嵌入 YggdrasilBase 以复用日志记录和 Yggdrasil 业务逻辑调用能力。
type ServerHandler struct {
	*ygghandler.YggdrasilBase
}

// NewServerHandler 创建服务端 Handler 实例。
func NewServerHandler(base *ygghandler.YggdrasilBase) *ServerHandler {
	return &ServerHandler{YggdrasilBase: base}
}

// HasJoined 服务端验证客户端是否成功加入服务器
//
// @Summary     [服务端] 验证客户端会话
// @Description 由 Minecraft 服务端调用，通过 username 和 serverId 验证客户端是否已成功加入服务器。验证通过后返回角色完整信息（含 textures 属性和数字签名）。
// @Tags        Yggdrasil-会话接口
// @Accept      json
// @Produce     json
// @Param       username query string true  "角色名称"
// @Param       serverId query string true  "服务端生成的随机标识"
// @Param       ip        query string false "客户端 IP 地址（可选，用于防止代理连接）"
// @Success     200   {object}  apiYgg.ProfileResponse      "验证成功，返回角色完整信息"
// @Failure     204   {object}  nil                        "未找到匹配的会话"
// @Failure     400   {object}  apiYgg.YggdrasilError    "缺少必要参数或参数过长"
// @Failure     500   {object}  apiYgg.YggdrasilError    "服务器内部错误"
// @Router      /sessionserver/session/minecraft/hasJoined [get]
func (h *ServerHandler) HasJoined(ctx *gin.Context) {
	h.Log.Info(ctx, "HasJoined - 服务端验证客户端")

	username := ctx.Query("username")
	serverId := ctx.Query("serverId")
	ip := ctx.Query("ip")

	if username == "" || serverId == "" {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "缺少必要参数 username 或 serverId")
		return
	}

	// 与 JoinServerRequest.ServerID (max=256) 保持一致的长度校验
	if len(serverId) > 256 {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "serverId 参数过长")
		return
	}

	// 调用 Logic 层验证会话
	profileResp, found, xErr := h.Service.Logic().HasJoined(ctx.Request.Context(), username, serverId, ip)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "会话验证失败")
		return
	}
	if !found {
		apiYgg.YggNoContent(ctx)
		return
	}

	ctx.JSON(http.StatusOK, profileResp)
}

// ProfileQuery 查询指定角色的属性信息
//
// @Summary     [服务端] 查询角色属性
// @Description 由 Minecraft 服务端或客户端调用，查询指定 UUID 的角色信息。根据 unsigned 参数决定是否包含 textures 属性的数字签名。
// @Tags        Yggdrasil-会话接口
// @Accept      json
// @Produce     json
// @Param       uuid     path  string true  "角色的无符号 UUID（32 位十六进制字符串）"
// @Param       unsigned query bool   false "是否不含签名，默认 true"
// @Success     200   {object}  apiYgg.ProfileResponse   "查询成功，返回角色信息（含属性）"
// @Failure     204   {object}  nil                         "角色不存在"
// @Failure     500   {object}  apiYgg.YggdrasilError     "服务器内部错误"
// @Router      /sessionserver/session/minecraft/profile/{uuid} [get]
func (h *ServerHandler) ProfileQuery(ctx *gin.Context) {
	h.Log.Info(ctx, "ProfileQuery - 查询角色属性")

	uuid := ctx.Param("uuid")
	if uuid == "" {
		apiYgg.YggNoContent(ctx)
		return
	}

	// 预校验 UUID 格式（与 JoinServer/RefreshToken 保持一致）
	// 无符号 UUID 为 32 个十六进制字符（0-9, a-f, A-F）
	if !yggdrasil.IsValidUnsignedUUID(uuid) {
		apiYgg.YggNoContent(ctx)
		return
	}

	// 解析 unsigned 参数，默认为 true
	unsigned := true
	if unsignedStr := ctx.Query("unsigned"); unsignedStr != "" {
		if val, err := strconv.ParseBool(unsignedStr); err == nil {
			unsigned = val
		}
	}

	// 调用 Logic 层查询角色
	profileResp, found, xErr := h.Service.Logic().QueryProfile(ctx.Request.Context(), uuid, unsigned)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "查询角色失败")
		return
	}
	if !found {
		apiYgg.YggNoContent(ctx)
		return
	}

	ctx.JSON(http.StatusOK, profileResp)
}

// ProfilesBatchLookup 按名称批量查询角色
//
// @Summary     [服务端] 批量查询角色
// @Description 由 Minecraft 服务端调用，根据角色名称列表批量查询角色信息。仅返回无符号 UUID 和名称，不包含角色属性。不存在的角色不包含在响应中，单次最多查询 10 个。
// @Tags        Yggdrasil-会话接口
// @Accept      json
// @Produce     json
// @Param       names body []string true "角色名称列表（JSON 数组，最多 10 个）"
// @Success     200   {object}  []apiYgg.BatchProfileItem    "查询成功"
// @Failure     400   {object}  apiYgg.YggdrasilError      "请求体格式错误"
// @Failure     500   {object}  apiYgg.YggdrasilError      "服务器内部错误"
// @Router      /api/profiles/minecraft [post]
func (h *ServerHandler) ProfilesBatchLookup(ctx *gin.Context) {
	h.Log.Info(ctx, "ProfilesBatchLookup - 批量查询角色")

	var names []string
	if err := ctx.ShouldBindJSON(&names); err != nil {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "请求体格式错误，期望 JSON 字符串数组")
		return
	}

	// 限制单次查询数量
	if len(names) == 0 {
		ctx.JSON(http.StatusOK, []apiYgg.BatchProfileItem{})
		return
	}
	if len(names) > bConst.YggdrasilBatchLookupMaxNames {
		h.Log.Warn(ctx, fmt.Sprintf("批量查询名称数量超限: 请求 %d 个，截断至 %d 个", len(names), bConst.YggdrasilBatchLookupMaxNames))
		names = names[:bConst.YggdrasilBatchLookupMaxNames]
	}

	// 调用 Logic 层批量查询
	items, xErr := h.Service.Logic().BatchLookupProfiles(ctx.Request.Context(), names)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "批量查询角色失败")
		return
	}

	ctx.JSON(http.StatusOK, items)
}
