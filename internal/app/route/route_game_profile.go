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

		// 皮肤/披风装备
		gameProfileGroup.PATCH("/:profile_id/skin/:skin_library_id", gameProfileHandler.EquipSkin)
		gameProfileGroup.DELETE("/:profile_id/skin", gameProfileHandler.UnequipSkin)
		gameProfileGroup.PATCH("/:profile_id/cape/:cape_library_id", gameProfileHandler.EquipCape)
		gameProfileGroup.DELETE("/:profile_id/cape", gameProfileHandler.UnequipCape)
	}
}
