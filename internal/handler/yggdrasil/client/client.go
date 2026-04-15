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

// Authenticate 密码登录认证。
//
// POST /api/v1/yggdrasil/authserver/authenticate
//
// 该接口由启动器调用，用于用户登录。支持邮箱或手机号作为登录凭证。
// 单角色时自动绑定到令牌并返回 selectedProfile，多角色时通过 refresh 选择。
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

	// 构建选中角色
	var selectedResp *apiYgg.ProfileResponse
	if selectedProfile != nil {
		selectedResp = &apiYgg.ProfileResponse{
			ID:   yggLogic.EncodeUnsignedUUID(selectedProfile.UUID),
			Name: selectedProfile.Name,
		}
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

// Refresh 刷新令牌。
//
// POST /api/v1/yggdrasil/authserver/refresh
//
// 该接口由启动器调用，用于刷新令牌。吊销原令牌，颁发新令牌。
// 携带 selectedProfile 时为角色选择操作。
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

	// 构建选中角色
	var selectedResp *apiYgg.ProfileResponse
	if selectedProfile != nil {
		selectedResp = &apiYgg.ProfileResponse{
			ID:   yggLogic.EncodeUnsignedUUID(selectedProfile.UUID),
			Name: selectedProfile.Name,
		}
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

// Validate 验证令牌有效性。
//
// POST /api/v1/yggdrasil/authserver/validate
//
// 该接口由启动器调用，验证 accessToken 是否有效。
// 有效返回 204 No Content，无效返回标准错误响应。
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

// Invalidate 吊销指定令牌。
//
// POST /api/v1/yggdrasil/authserver/invalidate
//
// 该接口由启动器调用，吊销指定的 accessToken。
// 无论是否成功，均返回 204 No Content。
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

// Signout 吊销用户所有令牌。
//
// POST /api/v1/yggdrasil/authserver/signout
//
// 该接口由启动器调用，验证用户凭证后吊销该用户的所有有效令牌。
// 成功返回 204 No Content，密码错误返回标准错误响应。
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

// JoinServer 客户端加入服务器。
//
// POST /api/v1/yggdrasil/sessionserver/session/minecraft/join
//
// 该接口由 Minecraft 客户端调用，记录加入服务器的会话信息。
// 验证令牌有效且 selectedProfile 与令牌绑定角色一致。
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
