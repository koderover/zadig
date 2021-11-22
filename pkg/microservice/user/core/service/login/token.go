package login

import (
	"time"

	"github.com/golang-jwt/jwt"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/user/core"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/pkg/setting"
)

type Claims struct {
	Name              string          `json:"name"`
	Email             string          `json:"email"`
	UID               string          `json:"uid"`
	PreferredUsername string          `json:"preferred_username"`
	FederatedClaims   FederatedClaims `json:"federated_claims"`
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

func GenerateClaimsByUser(user *models.User) *Claims {
	return &Claims{
		Name:              user.Name,
		UID:               user.UID,
		Email:             user.Email,
		PreferredUsername: user.Account,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
		FederatedClaims: FederatedClaims{
			ConnectorId: user.IdentityType,
			UserId:      user.Account,
		},
	}
}

func GenerateClaimsByUserID(userID string, expire time.Duration) (*Claims, error) {
	user, err := orm.GetUserByUid(userID, core.DB)
	if err != nil {
		return nil, err
	}

	if expire == 0 {
		expire = 24 * time.Hour
	}

	return &Claims{
		Name:              user.Name,
		UID:               user.UID,
		Email:             user.Email,
		PreferredUsername: user.Account,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: time.Now().Add(expire).Unix(),
		},
		FederatedClaims: FederatedClaims{
			ConnectorId: user.IdentityType,
			UserId:      user.Account,
		},
	}, nil
}
