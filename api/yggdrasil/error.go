package yggdrasil

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// YggdrasilError Yggdrasil 标准错误响应
type YggdrasilError struct {
	Error        string `json:"error"`                  // 错误标识（机器可读）
	ErrorMessage string `json:"errorMessage"`           // 错误详情（人类可读）
	Cause        string `json:"cause,omitempty"`        // 错误原因（可选）
}

var (
	// ErrForbidden 令牌无效错误
	ErrForbidden = YggdrasilError{Error: "ForbiddenOperationException", ErrorMessage: "Invalid token."}

	// ErrInvalidCredentials 凭证无效错误（密码错误或用户被禁）
	ErrInvalidCredentials = YggdrasilError{Error: "ForbiddenOperationException", ErrorMessage: "Invalid credentials. Invalid username or password."}

	// ErrProfileAssigned 令牌已绑定角色但仍试图指定角色错误
	ErrProfileAssigned = YggdrasilError{Error: "IllegalArgumentException", ErrorMessage: "Access token already has a profile assigned."}

	// ErrUnauthorized 未授权错误（缺少 Bearer 令牌）
	ErrUnauthorized = YggdrasilError{Error: "Unauthorized", ErrorMessage: "Bearer token required"}
)

// AbortYggError 中断请求并返回 Yggdrasil 标准错误响应
func AbortYggError(c *gin.Context, httpStatus int, errorStr string, errorMessage string) {
	c.JSON(httpStatus, YggdrasilError{
		Error:        errorStr,
		ErrorMessage: errorMessage,
	})
	c.Abort()
}

// AbortWithPredefinedError 使用预定义错误中断请求
func AbortWithPredefinedError(c *gin.Context, httpStatus int, yggErr YggdrasilError) {
	c.JSON(httpStatus, yggErr)
	c.Abort()
}

// YggNoContent 返回 204 No Content（用于 validate、invalidate 等无响应体接口）
func YggNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
