package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/middleware"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx-labs/beacon-sso-sdk/middleware"
)

func (r *route) adminRouter(route gin.IRouter) {
	userHandler := handler.NewHandler[handler.UserHandler](r.context, "UserHandler")

	adminGroup := route.Group("/admin/users")
	adminGroup.Use(bSdkMiddle.CheckAuth(r.context))
	adminGroup.Use(middleware.User(r.context))
	adminGroup.Use(middleware.Admin(r.context))
	{
		adminGroup.GET("", userHandler.ListAdminUsers)
		adminGroup.GET("/:user_id", userHandler.GetAdminUserDetail)
	}
}
