package protocol

type CreateUserRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
}

type RenameUserRequest struct {
	NewName string `json:"new_name"`
}
type DeleteUserRequest struct {
	ConfirmName string `json:"confirm_name"`
}
