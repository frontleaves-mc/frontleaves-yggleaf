package models

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
)

// SkinDTO 皮肤数据传输对象。
//
// 由 Logic 层从 entity.SkinLibrary + bucket 解析后的纹理链接构建。
// Handler 层负责将此 DTO 转换为 api/library.SkinResponse。
type SkinDTO struct {
	ID             xSnowflake.SnowflakeID    // 皮肤库记录 ID
	UserID         *xSnowflake.SnowflakeID   // 创建者/上传者用户 ID
	Name           string                    // 皮肤名称
	TextureURL     string                    // 纹理文件下载链接（由 bucket.Get 解析）
	TextureHash    string                    // 纹理 SHA256 哈希
	Model          entity.ModelType          // 皮肤模型 (1=classic, 2=slim)
	IsPublic       bool                      // 是否公开
	UpdatedAt      time.Time                 // 更新时间
	AssignmentType entityType.AssignmentType // 关联类型（mine 模式下返回）
}

// SkinSimpleDTO 皮肤精简数据传输对象（仅 ID + Name）。
type SkinSimpleDTO struct {
	ID   xSnowflake.SnowflakeID // 皮肤库记录 ID
	Name string                 // 皮肤名称
}
