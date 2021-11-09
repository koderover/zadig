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

type Option struct {
	CodeHostType string
	Address      string
	Namespace    string
	CodeHostID   int
}

func GetCodeHostInfo(option *Option) (*CodeHost, error) {
	codeHosts, err := New().ListCodeHosts()
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

func (c *Client) GetCodeHost(id int) (*CodeHost, error) {
	url := fmt.Sprintf("/codehosts/%d", id)

	res := &CodeHost{}
	_, err := c.Get(url, httpclient.SetResult(res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) ListCodeHosts() ([]*CodeHost, error) {
	url := "/codehosts"

	res := make([]*CodeHost, 0)
	_, err := c.Get(url, httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (c *Client) GetCodeHostByAddressAndOwner(address, owner, source string) (*CodeHost, error) {
	url := "/codehosts"

	res := make([]*CodeHost, 0)

	req := map[string]string{
		"address": address,
		"owner":   owner,
		"source":  source,
	}

	_, err := c.Get(url, httpclient.SetQueryParams(req), httpclient.SetResult(&res))
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("no codehost found")
	} else if len(res) > 1 {
		return nil, fmt.Errorf("more than one codehosts found")
	}

	return res[0], nil
}
