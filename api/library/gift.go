package library

// GiftSkinRequest 管理员赠送皮肤请求
type GiftSkinRequest struct {
	SkinLibraryID  string `json:"skin_library_id" binding:"required"` // 皮肤库 ID
	AssignmentType uint8  `json:"assignment_type" binding:"required"` // 分配类型 (2=gift, 3=admin)
}

// GiftCapeRequest 管理员赠送披风请求
type GiftCapeRequest struct {
	CapeLibraryID  string `json:"cape_library_id" binding:"required"` // 披风库 ID
	AssignmentType uint8  `json:"assignment_type" binding:"required"` // 分配类型 (2=gift, 3=admin)
}
