package library

import (
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
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

// SkinResponse 皮肤响应
type SkinResponse struct {
	entity.SkinLibrary
	AssignmentType entityType.AssignmentType `json:"assignment_type,omitempty"` // 关联类型（mine 模式下返回）
}

// SkinListResponse 皮肤列表响应
type SkinListResponse struct {
	Total int64          `json:"total"` // 总数
	Items []SkinResponse `json:"items"` // 皮肤列表
}
