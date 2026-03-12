/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package login

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/service/common"
	"github.com/koderover/zadig/v2/pkg/setting"
	zadigCache "github.com/koderover/zadig/v2/pkg/tool/cache"
	zadigcrypto "github.com/koderover/zadig/v2/pkg/tool/crypto"
)

type MFARequiredAction string

const (
	MFARequiredActionEnroll MFARequiredAction = "enroll"
	MFARequiredActionVerify MFARequiredAction = "verify"
)

const (
	mfaChallengePrefix            = "mfa:login:challenge"
	mfaChallengeTTL               = 5 * time.Minute
	mfaChallengeMaxFailedAttempts = 5
	mfaUserEnabledCachePrefix     = "mfa:user:enabled"
	mfaUserEnabledCacheTTL        = 30 * time.Second

	mfaTOTPPeriod      = 30
	mfaTOTPSkew        = 1
	mfaRecoveryCodeNum = 10
)

type mfaLoginChallenge struct {
	UID               string            `json:"uid"`
	RequiredAction    MFARequiredAction `json:"required_action"`
	PendingSecret     string            `json:"pending_secret"`
	FailedAttempts    int               `json:"failed_attempts"`
	ExpiresAt         int64             `json:"expires_at"`
	LastRefreshUnixTs int64             `json:"last_refresh_unix_ts"`
}

type MFASetupArgs struct {
	MFAChallengeToken string `json:"mfa_challenge_token"`
}

type MFASetupResp struct {
	RequiredAction    string `json:"required_action"`
	MFAChallengeToken string `json:"mfa_challenge_token"`
	MFAExpiresAt      int64  `json:"mfa_expires_at"`
	Secret            string `json:"secret"`
	OTPAuthURL        string `json:"otpauth_url"`
	QRCode            string `json:"qrcode"`
}

type MFAEnrollArgs struct {
	MFAChallengeToken string `json:"mfa_challenge_token"`
	OTPCode           string `json:"otp_code"`
}

type MFAVerifyArgs struct {
	MFAChallengeToken string `json:"mfa_challenge_token"`
	OTPCode           string `json:"otp_code,omitempty"`
	RecoveryCode      string `json:"recovery_code,omitempty"`
}

type MFADisableArgs struct {
	OTPCode      string `json:"otp_code,omitempty"`
	RecoveryCode string `json:"recovery_code,omitempty"`
}

type UserMFAStatus struct {
	UID     string `json:"uid"`
	Enabled bool   `json:"enabled"`
}

type userMFAEnabledCache struct {
	Enabled bool `json:"enabled"`
}

func IsMFARequiredForUser(uid string, logger *zap.SugaredLogger) (bool, error) {
	required, _, err := GetMFAPolicyForUser(uid, logger)
	return required, err
}

func GetMFAPolicyForUser(uid string, logger *zap.SugaredLogger) (bool, bool, error) {
	if uid == "" {
		return false, false, fmt.Errorf("uid is empty")
	}

	settings, err := common.GetSystemSecuritySettings(logger)
	if err != nil {
		return false, false, err
	}

	enabled, err := getUserMFAEnabled(uid, logger)
	if err != nil {
		return false, false, err
	}

	return settings.MFAEnabled || enabled, enabled, nil
}

func getUserMFAEnabled(uid string, logger *zap.SugaredLogger) (bool, error) {
	cacheRaw, err := zadigCache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).GetString(userMFAEnabledCacheKey(uid))
	if err == nil {
		cacheData := &userMFAEnabledCache{}
		if unmarshalErr := json.Unmarshal([]byte(cacheRaw), cacheData); unmarshalErr == nil {
			return cacheData.Enabled, nil
		} else if logger != nil {
			logger.Warnf("failed to unmarshal user mfa cache, uid: %s, err: %v", uid, unmarshalErr)
		}
	} else if err != redis.Nil && logger != nil {
		logger.Warnf("failed to get user mfa cache, uid: %s, err: %v", uid, err)
	}

	userMFA, err := orm.GetUserMFA(uid, repository.DB)
	if err != nil {
		return false, err
	}
	enabled := userMFA != nil && userMFA.Enabled
	if err := setUserMFAEnabledCache(uid, enabled); err != nil && logger != nil {
		logger.Warnf("failed to set user mfa cache, uid: %s, err: %v", uid, err)
	}
	return enabled, nil
}

func setUserMFAEnabledCache(uid string, enabled bool) error {
	cacheData, err := json.Marshal(&userMFAEnabledCache{Enabled: enabled})
	if err != nil {
		return err
	}
	return zadigCache.NewRedisCache(configbase.RedisCommonCacheTokenDB()).Write(
		userMFAEnabledCacheKey(uid),
		string(cacheData),
		mfaUserEnabledCacheTTL,
	)
}

func SyncUserMFAEnabledCache(uid string, enabled bool) error {
	if uid == "" {
		return fmt.Errorf("uid is empty")
	}
	return setUserMFAEnabledCache(uid, enabled)
}

func userMFAEnabledCacheKey(uid string) string {
	return fmt.Sprintf("%s:%s", mfaUserEnabledCachePrefix, uid)
}

func PrepareMFALoginChallengeForUser(uid string, logger *zap.SugaredLogger) (MFARequiredAction, string, int64, error) {
	mfaConfig, err := orm.GetUserMFA(uid, repository.DB)
	if err != nil {
		return "", "", 0, err
	}

	action := MFARequiredActionEnroll
	if mfaConfig != nil && mfaConfig.Enabled {
		action = MFARequiredActionVerify
	}

	challengeToken, expiresAt, err := CreateMFALoginChallenge(uid, action, logger)
	if err != nil {
		return "", "", 0, err
	}
	return action, challengeToken, expiresAt, nil
}

func CreateMFALoginChallenge(uid string, action MFARequiredAction, logger *zap.SugaredLogger) (string, int64, error) {
	if uid == "" {
		return "", 0, fmt.Errorf("empty user id")
	}
	if action != MFARequiredActionEnroll && action != MFARequiredActionVerify {
		return "", 0, fmt.Errorf("invalid mfa action")
	}

	challengeToken := uuid.NewString()
	expiresAt := time.Now().Add(mfaChallengeTTL).Unix()
	challenge := &mfaLoginChallenge{
		UID:               uid,
		RequiredAction:    action,
		FailedAttempts:    0,
		ExpiresAt:         expiresAt,
		LastRefreshUnixTs: time.Now().Unix(),
	}

	if err := saveMFALoginChallenge(challengeToken, challenge); err != nil {
		if logger != nil {
			logger.Errorf("CreateMFALoginChallenge save challenge failed, uid: %s, err: %s", uid, err)
		}
		return "", 0, err
	}
	return challengeToken, expiresAt, nil
}

func SetupMFA(args *MFASetupArgs, logger *zap.SugaredLogger) (*MFASetupResp, error) {
	_ = logger
	if args == nil || args.MFAChallengeToken == "" {
		return nil, fmt.Errorf("mfa challenge token is required")
	}

	challenge, err := getMFALoginChallenge(args.MFAChallengeToken)
	if err != nil {
		return nil, err
	}
	if challenge.RequiredAction != MFARequiredActionEnroll {
		return nil, fmt.Errorf("current mfa challenge does not support setup")
	}

	user, err := orm.GetUserByUid(challenge.UID, repository.DB)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	userMFA, err := orm.GetUserMFA(challenge.UID, repository.DB)
	if err != nil {
		return nil, err
	}
	if userMFA != nil && userMFA.Enabled {
		return nil, fmt.Errorf("mfa already enabled")
	}

	// when setup endpoint is called repeatedly for the same challenge, reuse the pending secret.
	secret := ""
	if challenge.PendingSecret != "" {
		secret, err = zadigcrypto.AesDecrypt(challenge.PendingSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to decode pending mfa secret")
		}
	} else {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      setting.ProductName,
			AccountName: user.Account,
			Period:      mfaTOTPPeriod,
			Digits:      otp.DigitsSix,
			Algorithm:   otp.AlgorithmSHA1,
		})
		if err != nil {
			return nil, err
		}

		secret = key.Secret()
		challenge.PendingSecret, err = zadigcrypto.AesEncrypt(secret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt mfa secret")
		}
		challenge.LastRefreshUnixTs = time.Now().Unix()

		if err = saveMFALoginChallenge(args.MFAChallengeToken, challenge); err != nil {
			return nil, err
		}
	}

	otpAuthURL := buildOTPAuthURL(user.Account, secret)
	key, err := otp.NewKeyFromURL(otpAuthURL)
	if err != nil {
		return nil, err
	}

	qrCode, err := keyToBase64QRCode(key)
	if err != nil {
		return nil, err
	}

	return &MFASetupResp{
		RequiredAction:    string(challenge.RequiredAction),
		MFAChallengeToken: args.MFAChallengeToken,
		MFAExpiresAt:      challenge.ExpiresAt,
		Secret:            secret,
		OTPAuthURL:        otpAuthURL,
		QRCode:            qrCode,
	}, nil
}

func EnrollMFA(args *MFAEnrollArgs, logger *zap.SugaredLogger) (*User, error) {
	if args == nil || args.MFAChallengeToken == "" {
		return nil, fmt.Errorf("mfa challenge token is required")
	}
	if args.OTPCode == "" {
		return nil, fmt.Errorf("otp code is required")
	}

	challenge, err := getMFALoginChallenge(args.MFAChallengeToken)
	if err != nil {
		return nil, err
	}
	if challenge.RequiredAction != MFARequiredActionEnroll {
		return nil, fmt.Errorf("current mfa challenge does not support enroll")
	}
	if challenge.PendingSecret == "" {
		return nil, fmt.Errorf("mfa setup is required before enroll")
	}

	secret, err := zadigcrypto.AesDecrypt(challenge.PendingSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode mfa secret")
	}
	if !validateTOTPCode(secret, args.OTPCode) {
		if recordErr := recordMFALoginChallengeFailure(args.MFAChallengeToken, challenge); recordErr != nil {
			return nil, recordErr
		}
		return nil, fmt.Errorf("invalid otp code")
	}

	recoveryCodes, recoveryCodeHashes, err := generateRecoveryCodes()
	if err != nil {
		return nil, err
	}
	recoveryCodesJSON, err := json.Marshal(recoveryCodeHashes)
	if err != nil {
		return nil, err
	}

	if err := orm.EnableUserMFA(challenge.UID, challenge.PendingSecret, string(recoveryCodesJSON), repository.DB); err != nil {
		if errors.Is(err, orm.ErrUserMFAAlreadyEnabled) {
			if cacheErr := setUserMFAEnabledCache(challenge.UID, true); cacheErr != nil && logger != nil {
				logger.Warnf("failed to sync user mfa cache for already-enabled user, uid: %s, err: %v", challenge.UID, cacheErr)
			}
			_ = consumeMFALoginChallenge(args.MFAChallengeToken)
			return nil, fmt.Errorf("mfa already enabled")
		}
		return nil, err
	}
	if cacheErr := setUserMFAEnabledCache(challenge.UID, true); cacheErr != nil && logger != nil {
		logger.Warnf("failed to sync user mfa cache after enroll, uid: %s, err: %v", challenge.UID, cacheErr)
	}

	if err := consumeMFALoginChallenge(args.MFAChallengeToken); err != nil && err != redis.Nil {
		return nil, err
	}

	userResp, err := issueLoginTokenByUID(challenge.UID, true, logger)
	if err != nil {
		return nil, err
	}
	userResp.RecoveryCodes = recoveryCodes
	userResp.MFARequired = false
	userResp.RequiredAction = ""
	userResp.MFAChallengeToken = ""
	userResp.MFAExpiresAt = 0

	return userResp, nil
}

func VerifyMFA(args *MFAVerifyArgs, logger *zap.SugaredLogger) (*User, error) {
	if args == nil || args.MFAChallengeToken == "" {
		return nil, fmt.Errorf("mfa challenge token is required")
	}
	if args.OTPCode == "" && args.RecoveryCode == "" {
		return nil, fmt.Errorf("otp code or recovery code is required")
	}

	challenge, err := getMFALoginChallenge(args.MFAChallengeToken)
	if err != nil {
		return nil, err
	}
	if challenge.RequiredAction != MFARequiredActionVerify {
		return nil, fmt.Errorf("current mfa challenge does not support verify")
	}

	userMFA, err := orm.GetUserMFA(challenge.UID, repository.DB)
	if err != nil {
		return nil, err
	}
	if userMFA == nil || !userMFA.Enabled {
		return nil, fmt.Errorf("mfa enrollment required")
	}

	valid := false
	if args.OTPCode != "" {
		secret, err := zadigcrypto.AesDecrypt(userMFA.SecretCipher)
		if err != nil {
			return nil, fmt.Errorf("failed to decode mfa secret")
		}
		valid = validateTOTPCode(secret, args.OTPCode)
	} else {
		valid, err = consumeRecoveryCode(userMFA, args.RecoveryCode, logger)
		if err != nil {
			return nil, err
		}
	}

	if !valid {
		if recordErr := recordMFALoginChallengeFailure(args.MFAChallengeToken, challenge); recordErr != nil {
			return nil, recordErr
		}
		return nil, fmt.Errorf("invalid mfa verification code")
	}

	if err := consumeMFALoginChallenge(args.MFAChallengeToken); err != nil && err != redis.Nil {
		return nil, err
	}

	userResp, err := issueLoginTokenByUID(challenge.UID, true, logger)
	if err != nil {
		return nil, err
	}
	userResp.MFARequired = false
	userResp.RequiredAction = ""
	userResp.MFAChallengeToken = ""
	userResp.MFAExpiresAt = 0
	return userResp, nil
}

func SetupUserMFA(uid string, logger *zap.SugaredLogger) (*MFASetupResp, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid is empty")
	}
	challengeToken, _, err := CreateMFALoginChallenge(uid, MFARequiredActionEnroll, logger)
	if err != nil {
		return nil, err
	}
	return SetupMFA(&MFASetupArgs{MFAChallengeToken: challengeToken}, logger)
}

func EnableUserMFA(uid string, args *MFAEnrollArgs, logger *zap.SugaredLogger) (*User, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid is empty")
	}
	if args == nil || args.MFAChallengeToken == "" {
		return nil, fmt.Errorf("mfa challenge token is required")
	}

	challenge, err := getMFALoginChallenge(args.MFAChallengeToken)
	if err != nil {
		return nil, err
	}
	if challenge.UID != uid {
		return nil, fmt.Errorf("mfa challenge does not belong to current user")
	}

	return EnrollMFA(args, logger)
}

func DisableUserMFA(uid string, args *MFADisableArgs, logger *zap.SugaredLogger) error {
	if uid == "" {
		return fmt.Errorf("uid is empty")
	}
	if args == nil {
		return fmt.Errorf("disable mfa args are required")
	}
	if args.OTPCode == "" && args.RecoveryCode == "" {
		return fmt.Errorf("otp code or recovery code is required")
	}

	settings, err := common.GetSystemSecuritySettings(logger)
	if err != nil {
		return err
	}
	if settings.MFAEnabled {
		return fmt.Errorf("mfa is enforced by administrator")
	}

	userMFA, err := orm.GetUserMFA(uid, repository.DB)
	if err != nil {
		return err
	}
	if userMFA == nil || !userMFA.Enabled {
		return fmt.Errorf("mfa not enabled")
	}

	valid := false
	if args.OTPCode != "" {
		secret, err := zadigcrypto.AesDecrypt(userMFA.SecretCipher)
		if err != nil {
			return fmt.Errorf("failed to decode mfa secret")
		}
		valid = validateTOTPCode(secret, args.OTPCode)
	} else {
		valid, err = consumeRecoveryCode(userMFA, args.RecoveryCode, logger)
		if err != nil {
			return err
		}
	}
	if !valid {
		return fmt.Errorf("invalid mfa verification code")
	}

	if err := orm.DeleteUserMFA(uid, repository.DB); err != nil {
		return err
	}
	if cacheErr := setUserMFAEnabledCache(uid, false); cacheErr != nil && logger != nil {
		logger.Warnf("failed to sync user mfa cache during disable, uid: %s, err: %v", uid, cacheErr)
	}
	if err := zadigCache.NewRedisCache(config.RedisUserTokenDB()).Delete(uid); err != nil && err != redis.Nil {
		if logger != nil {
			logger.Warnf("failed to clear user token cache during mfa disable, uid: %s, err: %v", uid, err)
		}
	}
	return nil
}

func GetUserMFAStatus(uid string, logger *zap.SugaredLogger) (*UserMFAStatus, error) {
	_ = logger
	if uid == "" {
		return nil, fmt.Errorf("uid is empty")
	}
	userMFA, err := orm.GetUserMFA(uid, repository.DB)
	if err != nil {
		return nil, err
	}
	return &UserMFAStatus{
		UID:     uid,
		Enabled: userMFA != nil && userMFA.Enabled,
	}, nil
}

func ResetUserMFA(uid string, logger *zap.SugaredLogger) error {
	if uid == "" {
		return fmt.Errorf("uid is empty")
	}
	if err := orm.DeleteUserMFA(uid, repository.DB); err != nil {
		return err
	}
	if cacheErr := setUserMFAEnabledCache(uid, false); cacheErr != nil && logger != nil {
		logger.Warnf("failed to sync user mfa cache during reset, uid: %s, err: %v", uid, cacheErr)
	}
	if err := zadigCache.NewRedisCache(config.RedisUserTokenDB()).Delete(uid); err != nil && err != redis.Nil {
		if logger != nil {
			logger.Warnf("failed to clear login token cache during mfa reset, uid: %s, err: %v", uid, err)
		}
	}
	return nil
}

func saveMFALoginChallenge(challengeToken string, challenge *mfaLoginChallenge) error {
	if challengeToken == "" {
		return fmt.Errorf("empty mfa challenge token")
	}
	payload, err := json.Marshal(challenge)
	if err != nil {
		return err
	}
	ttl := time.Until(time.Unix(challenge.ExpiresAt, 0))
	if ttl <= 0 {
		return fmt.Errorf("mfa challenge expired")
	}
	return zadigCache.NewRedisCache(config.RedisUserTokenDB()).Write(mfaChallengeKey(challengeToken), string(payload), ttl)
}

func getMFALoginChallenge(challengeToken string) (*mfaLoginChallenge, error) {
	if challengeToken == "" {
		return nil, fmt.Errorf("empty mfa challenge token")
	}

	raw, err := zadigCache.NewRedisCache(config.RedisUserTokenDB()).GetString(mfaChallengeKey(challengeToken))
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("mfa challenge not found or expired")
		}
		return nil, err
	}

	resp := &mfaLoginChallenge{}
	if err := json.Unmarshal([]byte(raw), resp); err != nil {
		return nil, err
	}
	if time.Now().Unix() >= resp.ExpiresAt {
		_ = consumeMFALoginChallenge(challengeToken)
		return nil, fmt.Errorf("mfa challenge not found or expired")
	}
	return resp, nil
}

func consumeMFALoginChallenge(challengeToken string) error {
	return zadigCache.NewRedisCache(config.RedisUserTokenDB()).Delete(mfaChallengeKey(challengeToken))
}

func recordMFALoginChallengeFailure(challengeToken string, challenge *mfaLoginChallenge) error {
	challenge.FailedAttempts++
	challenge.LastRefreshUnixTs = time.Now().Unix()
	if challenge.FailedAttempts >= mfaChallengeMaxFailedAttempts {
		_ = consumeMFALoginChallenge(challengeToken)
		return fmt.Errorf("mfa challenge exceeded max attempts, please login again")
	}
	return saveMFALoginChallenge(challengeToken, challenge)
}

func mfaChallengeKey(challengeToken string) string {
	return fmt.Sprintf("%s:%s", mfaChallengePrefix, challengeToken)
}

func buildMFARedirectURL(challengeToken string, action MFARequiredAction, expiresAt int64) string {
	query := url.Values{}
	query.Set("mfa_required", "true")
	query.Set("required_action", string(action))
	query.Set("mfa_challenge_token", challengeToken)
	query.Set("mfa_expires_at", fmt.Sprintf("%d", expiresAt))
	return "/mfa-enroll?" + query.Encode()
}

func buildOTPAuthURL(account, secret string) string {
	label := url.QueryEscape(fmt.Sprintf("%s:%s", setting.ProductName, account))
	issuer := url.QueryEscape(setting.ProductName)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&period=%d&digits=6&algorithm=SHA1", label, secret, issuer, mfaTOTPPeriod)
}

func keyToBase64QRCode(key *otp.Key) (string, error) {
	img, err := key.Image(256, 256)
	if err != nil {
		return "", err
	}
	buffer := new(bytes.Buffer)
	if err := png.Encode(buffer, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func validateTOTPCode(secret, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    mfaTOTPPeriod,
		Skew:      mfaTOTPSkew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	return err == nil && valid
}

func generateRecoveryCodes() ([]string, []string, error) {
	plaintextCodes := make([]string, 0, mfaRecoveryCodeNum)
	hashCodes := make([]string, 0, mfaRecoveryCodeNum)
	seen := make(map[string]struct{}, mfaRecoveryCodeNum)

	for len(plaintextCodes) < mfaRecoveryCodeNum {
		code, err := randomRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		plaintextCodes = append(plaintextCodes, code)
		hashCodes = append(hashCodes, hashRecoveryCode(code))
	}
	return plaintextCodes, hashCodes, nil
}

func randomRecoveryCode() (string, error) {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	encoded = strings.ToUpper(encoded)
	if len(encoded) < 10 {
		return "", fmt.Errorf("failed to generate recovery code")
	}
	code := encoded[:10]
	return code[:5] + "-" + code[5:], nil
}

func consumeRecoveryCode(userMFA *models.UserMFA, recoveryCode string, logger *zap.SugaredLogger) (bool, error) {
	if userMFA == nil {
		return false, nil
	}

	hashes := make([]string, 0)
	if userMFA.RecoveryCodesJSON != "" {
		if err := json.Unmarshal([]byte(userMFA.RecoveryCodesJSON), &hashes); err != nil {
			return false, err
		}
	}
	if len(hashes) == 0 {
		return false, nil
	}

	target := hashRecoveryCode(recoveryCode)
	matchedIdx := -1
	for i, hash := range hashes {
		if subtle.ConstantTimeCompare([]byte(target), []byte(hash)) == 1 {
			matchedIdx = i
			break
		}
	}

	if matchedIdx < 0 {
		return false, nil
	}

	newHashes := make([]string, 0, len(hashes)-1)
	newHashes = append(newHashes, hashes[:matchedIdx]...)
	newHashes = append(newHashes, hashes[matchedIdx+1:]...)
	newHashesJSON, err := json.Marshal(newHashes)
	if err != nil {
		return false, err
	}

	if err := orm.UpsertUserMFA(userMFA.UID, &models.UserMFA{
		UID:               userMFA.UID,
		Enabled:           userMFA.Enabled,
		SecretCipher:      userMFA.SecretCipher,
		RecoveryCodesJSON: string(newHashesJSON),
	}, repository.DB); err != nil {
		return false, err
	}
	if cacheErr := setUserMFAEnabledCache(userMFA.UID, userMFA.Enabled); cacheErr != nil && logger != nil {
		logger.Warnf("failed to sync user mfa cache during recovery code consume, uid: %s, err: %v", userMFA.UID, cacheErr)
	}
	return true, nil
}

func hashRecoveryCode(code string) string {
	normalized := normalizeRecoveryCode(code)
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(normalized))
	_, _ = hasher.Write([]byte(":"))
	_, _ = hasher.Write([]byte(configbase.SecretKey()))
	return hex.EncodeToString(hasher.Sum(nil))
}

func normalizeRecoveryCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return strings.ToUpper(code)
}
