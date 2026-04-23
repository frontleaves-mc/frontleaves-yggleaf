package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
)

func (r *route) syncRouter(route gin.IRouter) {
	syncHandler := handler.NewHandler[handler.SyncHandler](r.context, "SyncHandler")

	syncGroup := route.Group("/sync")
	{
		syncGroup.GET("/mods/metadata", syncHandler.ModsMetadata)
		syncGroup.GET("/config/metadata", syncHandler.ConfigMetadata)
		syncGroup.GET("/scripts/metadata", syncHandler.ScriptsMetadata)
		syncGroup.GET("/resourcepacks/metadata", syncHandler.ResourcepacksMetadata)
		syncGroup.GET("/extends/metadata", syncHandler.ExtendsMetadata)
		syncGroup.GET("/download", syncHandler.Download)
	}
}
