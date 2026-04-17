package route

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/middleware"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler"
	"github.com/gin-gonic/gin"
	bSdkMiddle "github.com/phalanx-labs/beacon-sso-sdk/middleware"
)

func (r *route) issueRouter(route gin.IRouter) {
	issueHandler := handler.NewHandler[handler.IssueHandler](r.context, "IssueHandler")

	// ---- 用户路由组（OAuth2 登录）----
	issueGroup := route.Group("/issue")
	issueGroup.Use(bSdkMiddle.CheckAuth(r.context))
	issueGroup.Use(middleware.User(r.context))

	{
		issueGroup.POST("", issueHandler.CreateIssue)
		issueGroup.GET("/list", issueHandler.GetIssueList)
		issueGroup.GET("/:id", issueHandler.GetIssueDetail)
		issueGroup.POST("/:id/reply", issueHandler.ReplyIssue)
		issueGroup.POST("/:id/attachment", issueHandler.UploadAttachment)
		issueGroup.DELETE("/attachment/:id", issueHandler.DeleteAttachment)
	}

	// ---- 管理员路由组 ----
	adminGroup := issueGroup.Group("/admin")
	adminGroup.Use(middleware.Admin(r.context))

	{
		adminGroup.GET("/list", issueHandler.GetIssueListAdmin)
		adminGroup.PUT("/:id/status", issueHandler.UpdateIssueStatus)
		adminGroup.PUT("/:id/priority", issueHandler.UpdateIssuePriority)
		adminGroup.PUT("/:id/note", issueHandler.UpdateIssueNote)
	}

	// ---- 类型路由组 ----
	typeGroup := route.Group("/issue-type")
	{
		typeGroup.GET("/list", issueHandler.ListIssueTypes)
	}

	// 管理员类型管理
	adminTypeGroup := route.Group("/admin/issue-type")
	adminTypeGroup.Use(bSdkMiddle.CheckAuth(r.context))
	adminTypeGroup.Use(middleware.User(r.context))
	adminTypeGroup.Use(middleware.Admin(r.context))

	{
		adminTypeGroup.POST("", issueHandler.CreateIssueType)
		adminTypeGroup.PUT("/:id", issueHandler.UpdateIssueType)
		adminTypeGroup.DELETE("/:id", issueHandler.DeleteIssueType)
	}
}
