package library

import (
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
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

// CapeResponse 披风响应
type CapeResponse struct {
	entity.CapeLibrary
	AssignmentType entityType.AssignmentType `json:"assignment_type,omitempty"` // 关联类型（mine 模式下返回）
}

// CapeListResponse 披风列表响应
type CapeListResponse struct {
	Total int64          `json:"total"` // 总数
	Items []CapeResponse `json:"items"` // 披风列表
}
