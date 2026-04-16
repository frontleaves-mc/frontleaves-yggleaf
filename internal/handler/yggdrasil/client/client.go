// Package client 提供 Yggdrasil 协议中由启动器/Minecraft 客户端消费的 HTTP 请求处理器。
//
// 该包中的处理器负责处理以下接口：
//   - #2: POST /authserver/authenticate — 密码登录认证
//   - #3: POST /authserver/refresh — 刷新令牌
//   - #4: POST /authserver/validate — 验证令牌有效性
//   - #5: POST /authserver/invalidate — 吊销指定令牌
//   - #6: POST /authserver/signout — 吊销用户所有令牌
//   - #7: POST /sessionserver/session/minecraft/join — 客户端加入服务器
package client

import (
	"fmt"
	"net/http"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	ygghandler "github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil"
	yggLogic "github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic/yggdrasil"
	"github.com/gin-gonic/gin"
)

// ClientHandler 客户端 Handler，处理启动器和 Minecraft 客户端消费的 Yggdrasil 接口。
//
// 嵌入 YggdrasilBase 以复用日志记录和 Yggdrasil 业务逻辑调用能力。
type ClientHandler struct {
	*ygghandler.YggdrasilBase
}

// NewClientHandler 创建客户端 Handler 实例。
func NewClientHandler(base *ygghandler.YggdrasilBase) *ClientHandler {
	return &ClientHandler{YggdrasilBase: base}
}

// Authenticate 密码登录认证
//
// @Summary     [客户端] 密码登录认证
// @Description 由启动器调用，使用邮箱或手机号+密码进行登录认证。单角色时自动绑定到令牌并返回 selectedProfile，多角色时通过 refresh 接口选择角色。
// @Tags        Yggdrasil-认证接口
// @Accept      json
// @Produce     json
// @Param       request body apiYgg.AuthenticateRequest true "登录认证请求"
// @Success     200   {object}  apiYgg.AuthenticateResponse  "认证成功"
// @Failure     400   {object}  apiYgg.YggdrasilError      "请求参数错误"
// @Failure     403   {object}  apiYgg.YggdrasilError      "认证失败（密码错误/账户异常）"
// @Failure     500   {object}  apiYgg.YggdrasilError      "服务器内部错误"
// @Router      /authserver/authenticate [post]
func (h *ClientHandler) Authenticate(ctx *gin.Context) {
	h.Log.Info(ctx, "Authenticate - 密码登录认证")

	req := &apiYgg.AuthenticateRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "请求体格式错误")
		return
	}

	// 调用 Logic 层认证
	accessToken, clientToken, profiles, selectedProfile, user, xErr := h.Service.Logic().AuthenticateUser(
		ctx.Request.Context(), req.Username, req.Password, req.ClientToken, req.RequestUser,
	)
	if xErr != nil {
		errMsg := string(xErr.ErrorMessage)
		// 根据错误码区分：5xxxx 为服务器内部错误，其余为认证错误
		if xErr.GetErrorCode() != nil && xErr.GetErrorCode().GetCode() >= 50000 {
			apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", errMsg)
		} else {
			apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", errMsg)
		}
		return
	}

	// 构建可用角色列表
	availableProfiles := make([]apiYgg.ProfileResponse, 0, len(profiles))
	for _, p := range profiles {
		availableProfiles = append(availableProfiles, apiYgg.ProfileResponse{
			ID:   yggLogic.EncodeUnsignedUUID(p.UUID),
			Name: p.Name,
		})
	}

	// 构建选中角色（含 textures 属性和数字签名）
	var selectedResp *apiYgg.ProfileResponse
	if selectedProfile != nil {
		selectedResp = h.Service.Logic().BuildProfileResponse(ctx.Request.Context(), selectedProfile, false)
	}

	// 构建响应
	resp := apiYgg.AuthenticateResponse{
		AccessToken:       accessToken,
		ClientToken:       clientToken,
		AvailableProfiles: availableProfiles,
		SelectedProfile:   selectedResp,
	}

	// 附加用户信息
	if user != nil {
		resp.User = &apiYgg.UserResponse{
			ID:         yggLogic.EncodeUnsignedUUID(yggLogic.DeriveUserUUID(user.ID.Int64())),
			Properties: []apiYgg.UserPropertyResponse{},
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// Refresh 刷新令牌
//
// @Summary     [客户端] 刷新令牌
// @Description 由启动器调用，使用 accessToken 刷新令牌。吊销原令牌并颁发新令牌，携带 selectedProfile 时为角色选择操作。
// @Tags        Yggdrasil-认证接口
// @Accept      json
// @Produce     json
// @Param       request body apiYgg.RefreshRequest true "刷新令牌请求"
// @Success     200   {object}  apiYgg.RefreshResponse     "刷新成功"
// @Failure     400   {object}  apiYgg.YggdrasilError     "参数错误（角色已绑定仍指定）"
// @Failure     403   {object}  apiYgg.YggdrasilError     "令牌无效或已过期"
// @Failure     500   {object}  apiYgg.YggdrasilError     "服务器内部错误"
// @Router      /authserver/refresh [post]
func (h *ClientHandler) Refresh(ctx *gin.Context) {
	h.Log.Info(ctx, "Refresh - 刷新令牌")

	req := &apiYgg.RefreshRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "请求体格式错误")
		return
	}

	// 提取 selectedProfile UUID
	var selectedProfileID string
	if req.SelectedProfile != nil {
		selectedProfileID = req.SelectedProfile.ID
	}

	// 调用 Logic 层刷新令牌
	newAccessToken, clientToken, selectedProfile, user, xErr := h.Service.Logic().RefreshToken(
		ctx.Request.Context(), req.AccessToken, req.ClientToken, selectedProfileID, req.RequestUser,
	)
	if xErr != nil {
		errMsg := string(xErr.ErrorMessage)
		// 令牌已绑定角色但仍试图指定 → 400 + IllegalArgumentException
		if xErr.GetErrorCode() == xError.OperationDenied {
			apiYgg.AbortYggError(ctx, http.StatusBadRequest, "IllegalArgumentException", errMsg)
		} else if xErr.GetErrorCode() != nil && xErr.GetErrorCode().GetCode() >= 50000 {
			// 服务器内部错误 → 500
			apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", errMsg)
		} else {
			// 其他错误 → 403 + ForbiddenOperationException
			apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", errMsg)
		}
		return
	}

	// 构建选中角色（含 textures 属性和数字签名）
	var selectedResp *apiYgg.ProfileResponse
	if selectedProfile != nil {
		selectedResp = h.Service.Logic().BuildProfileResponse(ctx.Request.Context(), selectedProfile, false)
	}

	// 构建响应
	resp := apiYgg.RefreshResponse{
		AccessToken:     newAccessToken,
		ClientToken:     clientToken,
		SelectedProfile: selectedResp,
	}

	// 附加用户信息
	if user != nil {
		resp.User = &apiYgg.UserResponse{
			ID:         yggLogic.EncodeUnsignedUUID(yggLogic.DeriveUserUUID(user.ID.Int64())),
			Properties: []apiYgg.UserPropertyResponse{},
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

// Validate 验证令牌有效性
//
// @Summary     [客户端] 验证令牌有效性
// @Description 由启动器调用，验证 accessToken 是否有效。有效返回 204 No Content，无效返回 403 错误。
// @Tags        Yggdrasil-认证接口
// @Accept      json
// @Produce     json
// @Param       request body apiYgg.ValidateRequest true "验证令牌请求"
// @Success     204   {object}  nil  "令牌有效"
// @Failure     400   {object}  apiYgg.YggdrasilError  "请求参数错误"
// @Failure     403   {object}  apiYgg.YggdrasilError  "令牌无效或 clientToken 不匹配"
// @Failure     500   {object}  apiYgg.YggdrasilError  "服务器内部错误"
// @Router      /authserver/validate [post]
func (h *ClientHandler) Validate(ctx *gin.Context) {
	h.Log.Info(ctx, "Validate - 验证令牌有效性")

	req := &apiYgg.ValidateRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "请求体格式错误")
		return
	}

	// 调用 Logic 层验证
	token, found, xErr := h.Service.Logic().ValidateGameToken(ctx.Request.Context(), req.AccessToken)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "令牌验证失败")
		return
	}

	if !found {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return
	}

	// 若提供了 clientToken，还需验证匹配
	if req.ClientToken != "" && token.ClientToken != req.ClientToken {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return
	}

	apiYgg.YggNoContent(ctx)
}

// Invalidate 吊销指定令牌
//
// @Summary     [客户端] 吊销指定令牌
// @Description 由启动器调用，吊销指定的 accessToken。无论是否成功，均返回 204 No Content。
// @Tags        Yggdrasil-认证接口
// @Accept      json
// @Produce     json
// @Param       request body apiYgg.InvalidateRequest true "吊销令牌请求"
// @Success     204   {object}  nil  "处理完成"
// @Router      /authserver/invalidate [post]
func (h *ClientHandler) Invalidate(ctx *gin.Context) {
	h.Log.Info(ctx, "Invalidate - 吊销指定令牌")

	req := &apiYgg.InvalidateRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		apiYgg.YggNoContent(ctx)
		return
	}

	invErr := h.Service.Logic().InvalidateToken(ctx.Request.Context(), req.AccessToken)
	if invErr != nil {
		h.Log.Error(ctx, fmt.Sprintf("吊销令牌异常（客户端不可见）: %s", invErr.ErrorMessage))
	}
	apiYgg.YggNoContent(ctx)
}

// Signout 吊销用户所有令牌
//
// @Summary     [客户端] 吊销用户所有令牌
// @Description 由启动器调用，验证用户凭证后吊销该用户的所有有效令牌。成功返回 204 No Content，密码错误返回 403 错误。
// @Tags        Yggdrasil-认证接口
// @Accept      json
// @Produce     json
// @Param       request body apiYgg.SignoutRequest true "登出请求"
// @Success     204   {object}  nil                       "登出成功"
// @Failure     400   {object}  apiYgg.YggdrasilError  "请求参数错误"
// @Failure     403   {object}  apiYgg.YggdrasilError  "密码错误或账户异常"
// @Failure     500   {object}  apiYgg.YggdrasilError  "服务器内部错误"
// @Router      /authserver/signout [post]
func (h *ClientHandler) Signout(ctx *gin.Context) {
	h.Log.Info(ctx, "Signout - 吊销用户所有令牌")

	req := &apiYgg.SignoutRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "请求体格式错误")
		return
	}

	xErr := h.Service.Logic().SignoutUser(ctx.Request.Context(), req.Username, req.Password)
	if xErr != nil {
		errMsg := string(xErr.ErrorMessage)
		if xErr.GetErrorCode() != nil && xErr.GetErrorCode().GetCode() >= 50000 {
			apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", errMsg)
		} else {
			apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", errMsg)
		}
		return
	}

	apiYgg.YggNoContent(ctx)
}

// JoinServer 客户端加入服务器
//
// @Summary     [客户端] 客户端加入服务器
// @Description 由 Minecraft 客户端调用，记录加入服务器的会话信息。验证令牌有效且 selectedProfile 与令牌绑定角色一致。
// @Tags        Yggdrasil-会话接口
// @Accept      json
// @Produce     json
// @Param       request body apiYgg.JoinServerRequest true "加入服务器请求"
// @Success     204   {object}  nil                       "会话记录成功"
// @Failure     400   {object}  apiYgg.YggdrasilError  "请求参数错误"
// @Failure     403   {object}  apiYgg.YggdrasilError  "令牌无效或角色不匹配"
// @Failure     500   {object}  apiYgg.YggdrasilError  "服务器内部错误"
// @Router      /sessionserver/session/minecraft/join [post]
func (h *ClientHandler) JoinServer(ctx *gin.Context) {
	h.Log.Info(ctx, "JoinServer - 客户端加入服务器")

	req := &apiYgg.JoinServerRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		apiYgg.AbortYggError(ctx, http.StatusBadRequest, "BadRequest", "请求体格式错误")
		return
	}

	logic := h.Service.Logic()

	// 验证令牌有效性
	token, found, xErr := logic.ValidateGameToken(ctx.Request.Context(), req.AccessToken)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "令牌验证失败")
		return
	}
	if !found {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return
	}

	// 验证令牌已绑定角色
	if token.BoundProfileID == nil {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return
	}

	// 查询请求的角色，验证与令牌绑定一致
	profile, profileFound, xErr := logic.GetProfileByUUID(ctx.Request.Context(), req.SelectedProfile)
	if xErr != nil || !profileFound {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return
	}
	if profile.ID != *token.BoundProfileID {
		apiYgg.AbortWithPredefinedError(ctx, http.StatusForbidden, apiYgg.ErrForbidden)
		return
	}

	// 记录会话到 Redis
	clientIP := ctx.ClientIP()
	xErr = logic.JoinServer(ctx.Request.Context(), req.AccessToken, req.SelectedProfile, req.ServerID, clientIP)
	if xErr != nil {
		apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", "写入会话失败")
		return
	}

	apiYgg.YggNoContent(ctx)
}
