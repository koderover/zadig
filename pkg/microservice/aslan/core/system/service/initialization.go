package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/plutusvendor"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/tool/rsa"
	"go.uber.org/zap"
)

type SystemInitializationStatus struct {
	Initialized   bool   `json:"initialized"`
	IsEnterprise  bool   `json:"is_enterprise"`
	LicenseStatus string `json:"license_status"`
	SystemID      string `json:"system_id"`
}

func GetSystemInitializationStatus(logger *zap.SugaredLogger) (*SystemInitializationStatus, error) {
	// first check if the system is enterprise version
	isEnterprise := config.Enterprise()

	// then check if the user has been initialized
	userCountInfo, err := user.New().CountUsers()
	if err != nil {
		logger.Errorf("failed to get user count, error: %s", err)
		return nil, fmt.Errorf("failed to check if the user is initialized, error: %s", err)
	}

	resp := &SystemInitializationStatus{
		IsEnterprise: isEnterprise,
	}

	if userCountInfo.TotalUser > 0 {
		resp.Initialized = true
	} else {
		resp.Initialized = false
	}

	// if it is an enterprise system, check about the license information
	if isEnterprise {
		licenseInfo, err := plutusvendor.New().CheckZadigXLicenseStatus()
		if err != nil {
			logger.Errorf("failed to get enterprise license info, error: %s", err)
			return nil, fmt.Errorf("failed to check enterprise license info, error: %s", err)
		}
		resp.LicenseStatus = licenseInfo.Status
		resp.SystemID = licenseInfo.SystemID
	}

	return resp, nil
}

func InitializeUser(username, password, company, email string, phone int64, reason, address string, logger *zap.SugaredLogger) error {
	userInfo, err := user.New().CreateUser(&user.CreateUserArgs{
		Name:     username,
		Password: password,
		Email:    email,
		Phone:    strconv.FormatInt(phone, 10),
		Account:  username,
	})

	if err != nil {
		logger.Errorf("failed to create user, error: %s", err)
		return fmt.Errorf("user initialization error: failed to create user, err: %s", err)
	}

	initializeInfo := &InitializeInfo{
		CreatedAt: time.Now().Unix(),
		Username:  username,
		Phone:     phone,
		Email:     email,
		Company:   company,
		Reason:    reason,
		Address:   address,
		Domain:    config.SystemAddress(),
	}

	err = reportRegister(initializeInfo)
	if err != nil {
		// don't stop the whole initialization process if the upload fails
		logger.Errorf("failed to upload initialization info, error: %s", err)
	}

	role, found, err := mongodb.NewRoleColl().Get("*", string(setting.SystemAdmin))
	if err != nil {
		logger.Errorf("Failed to get role %s in namespace %s, err: %s", string(setting.SystemAdmin), "*", err)
		return err
	} else if !found {
		logger.Errorf("Role %s is not found in namespace %s", string(setting.SystemAdmin), "*")
		return fmt.Errorf("role %s not found", string(setting.SystemAdmin))
	}

	args := &models.RoleBinding{
		Name:      config.RoleBindingNameFromUIDAndRole(userInfo.Uid, setting.SystemAdmin, "*"),
		Namespace: "*",
		Subjects:  []*models.Subject{{Kind: models.UserKind, UID: userInfo.Uid}},
		RoleRef: &models.RoleRef{
			Name:      role.Name,
			Namespace: role.Namespace,
		},
	}
	return mongodb.NewRoleBindingColl().UpdateOrCreate(args)
}

type InitializeInfo struct {
	CreatedAt int64  `json:"created_at"`
	Username  string `json:"username"`
	Phone     int64  `json:"phone,omitempty"`
	Email     string `json:"email"`
	Company   string `json:"company"`
	Reason    string `json:"reason,omitempty"`
	Address   string `json:"address,omitempty"`
	Domain    string `json:"domain"`
}

type Operation struct {
	Data string `json:"data"`
}

func reportRegister(info *InitializeInfo) error {
	registerByte, _ := json.Marshal(info)
	encrypt, err := rsa.EncryptByDefaultPublicKey(registerByte)
	if err != nil {
		log.Errorf("RSAEncrypt err: %s", err)
		return err
	}
	encodeString := base64.StdEncoding.EncodeToString(encrypt)
	reqBody := Operation{Data: encodeString}
	_, err = httpclient.Post("https://api.koderover.com/api/operation/admin/user", httpclient.SetBody(reqBody))
	return err
}
