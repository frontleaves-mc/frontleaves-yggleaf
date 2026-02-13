package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx/beacon-sso-sdk/middleware"
)

func (r *route) gameProfileRouter(route gin.IRouter) {
	gameProfileHandler := handler.NewHandler[handler.GameProfileHandler](r.context, "GameProfileHandler")

	gameProfileGroup := route.Group("/game-profile")
	gameProfileGroup.Use(bSdkMiddle.CheckAuth(r.context))

	{
		gameProfileGroup.POST("", gameProfileHandler.AddGameProfile)
		gameProfileGroup.PATCH("/:profile_id/username", gameProfileHandler.ChangeUsername)
	}
}
