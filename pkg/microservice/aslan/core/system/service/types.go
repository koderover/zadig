package service

type ExternalSystemDetail struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Server   string `json:"server"`
	APIToken string `json:"api_token,omitempty"`
}
