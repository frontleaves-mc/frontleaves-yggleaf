package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx/beacon-sso-sdk/middleware"
)

func (r *route) userRouter(route gin.IRouter) {
	userGroup := route.Group("/user")
	userGroup.Use(bSdkMiddle.CheckAuth(r.context))

	userHandler := handler.NewHandler[handler.UserHandler](r.context, "UserHandler")

	{
		userGroup.GET("/info", userHandler.UserCurrent)
	}
}
