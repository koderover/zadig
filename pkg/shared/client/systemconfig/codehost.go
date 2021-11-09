package systemconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type CodeHost struct {
	ID          int    `json:"id"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	AccessToken string `json:"accessToken"`
	Namespace   string `json:"namespace"`
	Region      string `json:"region"`
	AccessKey   string `json:"applicationId"`
	SecretKey   string `json:"clientSecret"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

func GetCodeHostList() ([]*CodeHost, error) {
	var list []*CodeHost
	_, err := New().Post(fmt.Sprintf("/api/v1/codehost"), httpclient.SetResult(&list))
	if err != nil {
		return nil, err
	}
	return list, nil
}

type Option struct {
	CodeHostType string
	Address      string
	Namespace    string
	CodeHostID   int
}

func GetCodeHostInfo(option *Option) (*CodeHost, error) {
	codeHosts, err := GetCodeHostList()
	if err != nil {
		return nil, err
	}

	for _, codeHost := range codeHosts {
		if option.CodeHostID != 0 && codeHost.ID == option.CodeHostID {
			return codeHost, nil
		} else if option.CodeHostID == 0 && option.CodeHostType != "" {
			switch option.CodeHostType {
			case codehost.GitHubProvider:
				ns := strings.ToLower(codeHost.Namespace)
				if strings.Contains(option.Address, codeHost.Address) && strings.ToLower(option.Namespace) == ns {
					return codeHost, nil
				}
			default:
				if strings.Contains(option.Address, codeHost.Address) {
					return codeHost, nil
				}
			}
		}
	}

	return nil, errors.New("not find codeHost")
}

func GetCodeHostInfoByID(id int) (*CodeHost, error) {
	return GetCodeHostInfo(&Option{CodeHostID: id})
}

type Detail struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Owner      string `json:"repoowner"`
	Source     string `json:"source"`
	OauthToken string `json:"oauth_token"`
	Region     string `json:"region"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AccessKey  string `json:"applicationId"`
	SecretKey  string `json:"clientSecret"`
}

func GetCodehostDetail(codehostID int) (*Detail, error) {
	codehost, err := GetCodeHostInfo(&Option{CodeHostID: codehostID})
	if err != nil {
		return nil, err
	}
	detail := &Detail{
		codehostID,
		"",
		codehost.Address,
		codehost.Namespace,
		codehost.Type,
		codehost.AccessToken,
		codehost.Region,
		codehost.Username,
		codehost.Password,
		codehost.AccessKey,
		codehost.SecretKey,
	}

	return detail, nil
}

func ListCodehostDetial() ([]*Detail, error) {
	codehosts, err := GetCodeHostList()
	if err != nil {
		return nil, err
	}

	var details []*Detail

	for _, codehost := range codehosts {
		details = append(details, &Detail{
			codehost.ID,
			"",
			codehost.Address,
			codehost.Namespace,
			codehost.Type,
			codehost.AccessToken,
			codehost.Region,
			codehost.Username,
			codehost.Password,
			codehost.AccessKey,
			codehost.SecretKey,
		})
	}

	return details, nil
}
