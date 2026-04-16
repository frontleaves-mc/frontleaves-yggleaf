package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/middleware"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx-labs/beacon-sso-sdk/middleware"
)

func (r *route) libraryRouter(route gin.IRouter) {
	libraryHandler := handler.NewHandler[handler.LibraryHandler](r.context, "LibraryHandler")

	libraryGroup := route.Group("/library")
	libraryGroup.Use(bSdkMiddle.CheckAuth(r.context))
	libraryGroup.Use(middleware.User(r.context))

	{
		// 皮肤相关接口
		skinGroup := libraryGroup.Group("/skins")
		{
			skinGroup.POST("", libraryHandler.CreateSkin)
			skinGroup.GET("", libraryHandler.ListSkins)
			skinGroup.GET("/list", libraryHandler.ListMySkinsSimple)
			skinGroup.PATCH("/:skin_id", libraryHandler.UpdateSkin)
			skinGroup.DELETE("/:skin_id", libraryHandler.DeleteSkin)
		}

		// 披风相关接口
		capeGroup := libraryGroup.Group("/capes")
		{
			capeGroup.POST("", libraryHandler.CreateCape)
			capeGroup.GET("", libraryHandler.ListCapes)
			capeGroup.GET("/list", libraryHandler.ListMyCapesSimple)
			capeGroup.PATCH("/:cape_id", libraryHandler.UpdateCape)
			capeGroup.DELETE("/:cape_id", libraryHandler.DeleteCape)
		}

		// 配额查询接口
		libraryGroup.GET("/quota", libraryHandler.GetQuota)

		// 管理员接口
		adminGroup := libraryGroup.Group("/admin")
		adminGroup.Use(middleware.Admin(r.context))
		{
			// 管理员赠送/撤销皮肤
			adminGroup.POST("/users/:user_id/skins/gift", libraryHandler.GiftSkin)
			adminGroup.DELETE("/users/:user_id/skins/:skin_library_id", libraryHandler.RevokeSkin)

			// 管理员赠送/撤销披风
			adminGroup.POST("/users/:user_id/capes/gift", libraryHandler.GiftCape)
			adminGroup.DELETE("/users/:user_id/capes/:cape_library_id", libraryHandler.RevokeCape)

			// 管理员配额同步
			adminGroup.POST("/users/:user_id/quota/sync", libraryHandler.SyncQuota)

			// 管理员查询用户资源
			adminGroup.GET("/users/:user_id/skins", libraryHandler.ListUserSkins)
			adminGroup.GET("/users/:user_id/capes", libraryHandler.ListUserCapes)
		}
	}
}
