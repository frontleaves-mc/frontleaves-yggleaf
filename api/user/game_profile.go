package user

type AddGameProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

type ChangeUsernameRequest struct {
	NewName string `json:"new_name" binding:"required"`
}
