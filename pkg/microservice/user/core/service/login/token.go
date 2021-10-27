package login

import (
	"github.com/golang-jwt/jwt"

	"github.com/koderover/zadig/pkg/config"
)

type Claims struct {
	Name            string          `json:"name"`
	Email           string          `json:"email"`
	Uid             string          `json:"uid"`
	FederatedClaims FederatedClaims `json:"federated_claims"`
	jwt.StandardClaims
}

type FederatedClaims struct {
	ConnectorId string `json:"connector_id"`
	UserId      string `json:"user_id"`
}

func CreateToken(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.SecretKey()))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
