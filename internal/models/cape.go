package models

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
)

// CapeDTO 披风数据传输对象。
//
// 由 Logic 层从 entity.CapeLibrary + bucket 解析后的纹理链接构建。
// Handler 层负责将此 DTO 转换为 api/library.CapeResponse。
type CapeDTO struct {
	ID             xSnowflake.SnowflakeID    // 披风库记录 ID
	UserID         *xSnowflake.SnowflakeID   // 创建者/上传者用户 ID
	Name           string                    // 披风名称
	TextureURL     string                    // 纹理文件下载链接（由 bucket.Get 解析）
	TextureHash    string                    // 纹理 SHA256 哈希
	IsPublic       bool                      // 是否公开
	UpdatedAt      time.Time                 // 更新时间
	AssignmentType entityType.AssignmentType // 关联类型（mine 模式下返回）
}

// CapeSimpleDTO 披风精简数据传输对象（仅 ID + Name）。
type CapeSimpleDTO struct {
	ID   xSnowflake.SnowflakeID // 披风库记录 ID
	Name string                 // 披风名称
}
