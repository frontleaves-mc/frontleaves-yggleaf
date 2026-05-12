package admin

import "time"

// AdminUserListRequest 管理员用户列表查询参数。
type AdminUserListRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	PageSize  int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Role      string `form:"role" binding:"omitempty,oneof=SUPER_ADMIN ADMIN PLAYER"`
	Keyword   string `form:"keyword" binding:"omitempty,max=64"`
	StartTime string `form:"start_time"` // RFC3339 格式
	EndTime   string `form:"end_time"`   // RFC3339 格式
}

// AdminUserListResponse 管理员用户列表响应。
type AdminUserListResponse struct {
	List  []AdminUserItem `json:"list"`
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
}

// AdminUserItem 用户列表项（精简字段）。
type AdminUserItem struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     *string   `json:"email"`
	RoleName  *string   `json:"role_name"`
	HasBan    bool      `json:"has_ban"`
	UpdatedAt time.Time `json:"updated_at"`
}
