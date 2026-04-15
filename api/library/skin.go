package library

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
)

// CreateSkinRequest 创建皮肤请求
type CreateSkinRequest struct {
	Name     string `json:"name" binding:"required"`            // 皮肤名称
	Model    uint8  `json:"model" binding:"required,oneof=1 2"` // 皮肤模型 (1=classic, 2=slim)
	Texture  string `json:"texture" binding:"required"`         // 皮肤纹理文件 base64
	IsPublic *bool  `json:"is_public,omitempty"`                // 是否公开（可选，默认 false）
}

// UpdateSkinRequest 更新皮肤请求
type UpdateSkinRequest struct {
	Name     *string `json:"name,omitempty"`      // 皮肤名称（可选）
	IsPublic *bool   `json:"is_public,omitempty"` // 是否公开（可选）
}

// SkinResponse 皮肤响应 DTO。
//
// 不再嵌入 entity.SkinLibrary，改为显式字段定义。
// Texture 字段从数据库的 int64 文件 ID 变更为 beacon-bucket 返回的下载链接。
type SkinResponse struct {
	ID             xSnowflake.SnowflakeID    `json:"id"`                          // 皮肤库记录 ID
	UserID         *xSnowflake.SnowflakeID   `json:"user_id,omitempty"`           // 创建者/上传者用户 ID
	Name           string                    `json:"name"`                        // 皮肤名称
	TextureURL     string                    `json:"texture_url"`                 // 纹理文件下载链接（由 bucket.Get 解析）
	TextureHash    string                    `json:"texture_hash"`                // 纹理 SHA256 哈希
	Model          entity.ModelType          `json:"model"`                       // 皮肤模型 (1=classic, 2=slim)
	IsPublic       bool                      `json:"is_public"`                   // 是否公开
	UpdatedAt      time.Time                 `json:"updated_at"`                  // 更新时间
	AssignmentType entityType.AssignmentType `json:"assignment_type,omitempty"`   // 关联类型（mine 模式下返回）
}

// SkinListResponse 皮肤列表响应
type SkinListResponse struct {
	Total int64          `json:"total"` // 总数
	Items []SkinResponse `json:"items"` // 皮肤列表
}
