package types

type AuthType string

const (
	SSHAuthType                AuthType = "SSH"
	PrivateAccessTokenAuthType AuthType = "PrivateAccessToken"
)
