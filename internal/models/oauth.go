package models

type GoogleAuthURLResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type GoogleCallbackRequest struct {
	Code  string `json:"code" validate:"required"`
	State string `json:"state" validate:"required"`
}
