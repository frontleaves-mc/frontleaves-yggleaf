package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx/beacon-sso-sdk/middleware"
)

func (r *route) libraryRouter(route gin.IRouter) {
	libraryHandler := handler.NewHandler[handler.LibraryHandler](r.context, "LibraryHandler")

	libraryGroup := route.Group("/library")
	libraryGroup.Use(bSdkMiddle.CheckAuth(r.context))

	{
		// 皮肤相关接口
		skinGroup := libraryGroup.Group("/skins")
		{
			skinGroup.POST("", libraryHandler.CreateSkin)
			skinGroup.GET("", libraryHandler.ListSkins)
			skinGroup.PATCH("/:skin_id", libraryHandler.UpdateSkin)
			skinGroup.DELETE("/:skin_id", libraryHandler.DeleteSkin)
		}

		// 披风相关接口
		capeGroup := libraryGroup.Group("/capes")
		{
			capeGroup.POST("", libraryHandler.CreateCape)
			capeGroup.GET("", libraryHandler.ListCapes)
			capeGroup.PATCH("/:cape_id", libraryHandler.UpdateCape)
			capeGroup.DELETE("/:cape_id", libraryHandler.DeleteCape)
		}
	}
}
