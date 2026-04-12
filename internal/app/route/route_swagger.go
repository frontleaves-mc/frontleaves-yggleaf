package route

import (
	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/frontleaves-mc/frontleaves-yggleaf/docs"
	"github.com/gin-gonic/gin"
	_ "github.com/phalanx-labs/beacon-sso-sdk/docs"
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
func swaggerRegister() func(gin.IRouter) {
	docs.SwaggerInfofrontleaves_yggleaf.BasePath = "/api/v1"
	docs.SwaggerInfofrontleaves_yggleaf.Title = "FrontLeavesYggleaf"
	docs.SwaggerInfofrontleaves_yggleaf.Description = "锋楪技术（深圳）有限公司 - 我的世界用户中心"
	docs.SwaggerInfofrontleaves_yggleaf.Version = "v1.0.0"
	docs.SwaggerInfofrontleaves_yggleaf.Host = xEnv.GetEnvString(xEnv.Host, "localhost") + ":" + xEnv.GetEnvString(xEnv.Port, "5566")
	docs.SwaggerInfofrontleaves_yggleaf.Schemes = []string{"http", "https"}

	return func(r gin.IRouter) {
		swaggerGroup := r.Group("/swagger")
		swaggerGroup.GET("/yggleaf/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("frontleaves_yggleaf")))
		swaggerGroup.GET("/sso/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.InstanceName("beacon_sso_sdk")))
	}
}
