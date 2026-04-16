package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/middleware"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx-labs/beacon-sso-sdk/middleware"
)

func (r *route) gameProfileRouter(route gin.IRouter) {
	gameProfileHandler := handler.NewHandler[handler.GameProfileHandler](r.context, "GameProfileHandler")

	gameProfileGroup := route.Group("/game-profile")
	gameProfileGroup.Use(bSdkMiddle.CheckAuth(r.context))
	gameProfileGroup.Use(middleware.User(r.context))

	{
		gameProfileGroup.GET("", gameProfileHandler.ListGameProfiles)
		gameProfileGroup.POST("", gameProfileHandler.AddGameProfile)
		gameProfileGroup.GET("/quota", gameProfileHandler.GetQuota)
		gameProfileGroup.GET("/:profile_id", gameProfileHandler.GetGameProfileDetail)
		gameProfileGroup.PATCH("/:profile_id/username", gameProfileHandler.ChangeUsername)

		// --- 统一设置接口 ---
		gameProfileGroup.PATCH("/:profile_id/skin", gameProfileHandler.SetSkin)
		gameProfileGroup.PATCH("/:profile_id/cape", gameProfileHandler.SetCape)
	}
}
