package handler

import (
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查接口处理器。
type HealthHandler handler

// Ping 服务存活探针。
//
// @Summary     [公开] 存活探针
// @Description 用于 Docker / Kubernetes 健康检查，无需认证
// @Tags        健康检查
// @Produce     json
// @Success     200 {object} xBase.BaseResponse "服务正常"
// @Router      /health/ping [GET]
func (h *HealthHandler) Ping(ctx *gin.Context) {
	xResult.Success(ctx, "pong")
}
