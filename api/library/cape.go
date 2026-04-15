package library

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
)

// CreateCapeRequest 创建披风请求
type CreateCapeRequest struct {
	Name     string `json:"name" binding:"required"`    // 披风名称
	Texture  string `json:"texture" binding:"required"` // 披风纹理文件 base64
	IsPublic *bool  `json:"is_public,omitempty"`        // 是否公开（可选，默认 false）
}

// UpdateCapeRequest 更新披风请求
type UpdateCapeRequest struct {
	Name     *string `json:"name,omitempty"`      // 披风名称（可选）
	IsPublic *bool   `json:"is_public,omitempty"` // 是否公开（可选）
}

// CapeResponse 披风响应 DTO。
//
// 不再嵌入 entity.CapeLibrary，改为显式字段定义。
// Texture 字段从数据库的 int64 文件 ID 变更为 beacon-bucket 返回的下载链接。
type CapeResponse struct {
	ID             xSnowflake.SnowflakeID    `json:"id"`                          // 披风库记录 ID
	UserID         *xSnowflake.SnowflakeID   `json:"user_id,omitempty"`           // 创建者/上传者用户 ID
	Name           string                    `json:"name"`                        // 披风名称
	TextureURL     string                    `json:"texture_url"`                 // 纹理文件下载链接（由 bucket.Get 解析）
	TextureHash    string                    `json:"texture_hash"`                // 纹理 SHA256 哈希
	IsPublic       bool                      `json:"is_public"`                   // 是否公开
	UpdatedAt      time.Time                 `json:"updated_at"`                  // 更新时间
	AssignmentType entityType.AssignmentType `json:"assignment_type,omitempty"`   // 关联类型（mine 模式下返回）
}

// CapeListResponse 披风列表响应
type CapeListResponse struct {
	Total int64          `json:"total"` // 总数
	Items []CapeResponse `json:"items"` // 披风列表
}
