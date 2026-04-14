package user

import "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"

type AddGameProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

type ChangeUsernameRequest struct {
	NewName string `json:"new_name" binding:"required"`
}

// GameProfileListResponse 游戏档案列表响应
type GameProfileListResponse struct {
	Items []entity.GameProfile `json:"items"` // 游戏档案列表
}
