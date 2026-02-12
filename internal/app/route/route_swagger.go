package route

import (
	xEnv "github.com/bamboo-services/bamboo-base-go/env"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/frontleaves-mc/frontleaves-yggleaf/docs"
	"github.com/gin-gonic/gin"
)

// swaggerRegister 注册 Swagger 文档接口到指定的路由组。
//
// 该函数通过 `SwaggerInfo` 配置 API 文档的基本信息，包括版本、地址等，
// 并绑定 `/swagger/*any` 路由展示 Swagger UI 页面。
//
// 注意:
//   - `SwaggerInfo` 的字段可通过环境变量动态设置以适配不同环境。
//   - 此函数建议仅在开发调试模式下调用以避免生产环境暴露内部文档。
//
// 参数说明:
//   - r: Gin 路由组实例，用于注册 Swagger 路由。
func swaggerRegister(r gin.IRouter) {
	docs.SwaggerInfo.BasePath = "/api/v1"
	docs.SwaggerInfo.Title = "FrontLeavesYggleaf"
	docs.SwaggerInfo.Description = "锋楪技术（深圳）有限公司我的世界用户中心"
	docs.SwaggerInfo.Version = "v1.0.0"
	docs.SwaggerInfo.Host = xEnv.GetEnvString(xEnv.Host, "localhost") + ":" + xEnv.GetEnvString(xEnv.Port, "5566")
	docs.SwaggerInfo.Schemes = []string{"http", "https"}

	swaggerGroup := r.Group("/swagger")
	swaggerGroup.GET("/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
