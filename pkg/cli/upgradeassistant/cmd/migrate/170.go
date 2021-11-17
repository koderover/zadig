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

package migrate

import (
	internalmodels "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/models"
	internalmongodb "github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/repository/mongodb"
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.AddHandler(upgradepath.V160, upgradepath.V170, V160ToV170)
	upgradepath.AddHandler(upgradepath.V170, upgradepath.V160, V170ToV160)
}

// V160ToV170 refreshes the secret of all webhooks
func V160ToV170() error {
	log.Info("Migrating data from 1.6.0 to 1.7.0")

	err := changeCodehostType()
	if err != nil {
		log.Errorf("Failed to change codehost type, err: %s", err)
		return err
	}

	err = refreshWebHookSecret(gitservice.GetHookSecret())
	if err != nil {
		log.Errorf("Failed to refresh webhook secret, err: %s", err)
		return err
	}

	return nil
}

func V170ToV160() error {
	log.Info("Rollback data from 1.7.0 to 1.6.0")
	err := rollbackCodehostType()
	if err != nil {
		log.Errorf("Failed to rollback codehost type, err: %s", err)
		return err
	}
	token := getWebHookTokenFromOrganization()
	if token == "" {
		return nil
	}

	log.Info("Start to rollback all webhook secrets")
	err = refreshWebHookSecret(token)
	if err != nil {
		log.Errorf("Failed to refresh webhook secret, err: %s", err)
		return err
	}

	return nil
}

type hookRefresher interface {
	RefreshWebHookSecret(secret, owner, repo, hookID string) error
}

func refreshWebHookSecret(secret string) error {
	hooks, err := mongodb.NewWebHookColl().List()
	if err != nil {
		log.Errorf("Failed to list webhooks, err: %s", err)
		return err
	}

	codeHosts, err := internalmongodb.NewCodehostColl().List()
	if err != nil {
		log.Errorf("Failed to list codehosts, err: %s", err)
		return err
	}

	for _, hook := range hooks {
		var cl hookRefresher
		ch := getCodeHostByAddressAndOwner(hook.Address, hook.Owner, codeHosts)
		if ch == nil {
			log.Warnf("Failed to get codehost for %s/%s", hook.Address, hook.Owner)
			continue
		}

		switch ch.Type {
		case setting.SourceFromGithub:
			cl = github.NewClient(ch.AccessToken, config.ProxyHTTPSAddr())
		case setting.SourceFromGitlab:
			cl, err = gitlab.NewClient(ch.Address, ch.AccessToken)
			if err != nil {
				log.Warnf("Failed to create gitlab client, err: %s", err)
				continue
			}
		default:
			log.Warnf("Invalid type: %s", ch.Type)
			continue
		}

		if err = cl.RefreshWebHookSecret(secret, hook.Owner, hook.Repo, hook.HookID); err != nil {
			log.Warnf("Failed to refresh webhook secret for hook %d in %s/%s, err: %s", hook.HookID, hook.Owner, hook.Repo, err)
			continue
		}
	}

	return nil
}

func getCodeHostByAddressAndOwner(address, owner string, all []*internalmodels.CodeHost) *internalmodels.CodeHost {
	for _, one := range all {
		if one.Address != address {
			continue
		}
		// it is a limitation, we support only one gitlab account before
		if one.Type == "gitlab" {
			return one
		}
		if one.Namespace == owner {
			return one
		}
	}

	return nil
}

// change type "1,2,3,4" to "github,gitlab..."
func changeCodehostType() error {
	// get all codehosts
	codeHosts, err := internalmongodb.NewCodehostColl().List()
	if err != nil {
		log.Errorf("fail to list codehosts, err: %s", err)
		return err
	}
	var finalErr error
	// change type to readable string
	for _, v := range codeHosts {
		if err := internalmongodb.NewCodehostColl().ChangeType(v.ID, v.Type); err != nil {
			log.Warnf("fail to change id:%d type:%s , err: %s", v.ID, v.Type, err)
			finalErr = err
		}
	}
	return finalErr
}

// rollback type "github,gitlab..." to "1,2..."
func rollbackCodehostType() error {
	// get all codehosts
	codeHosts, err := internalmongodb.NewCodehostColl().List()
	if err != nil {
		log.Errorf("fail to list codehosts, err: %s", err)
		return err
	}
	var finalErr error
	// rollback change type to readable string
	for _, v := range codeHosts {
		if err := internalmongodb.NewCodehostColl().RollbackType(v.ID, v.Type); err != nil {
			log.Warnf("fail to rollback id:%d type:%s , err: %s", v.ID, v.Type, err)
			finalErr = err
			continue
		}
	}
	return finalErr
}

func getWebHookTokenFromOrganization() string {
	org, found, err := internalmongodb.NewOrganizationColl().Get(1)
	if err != nil || !found {
		log.Warnf("Failed to get organization, err: %s", err)
		return ""
	}

	return org.Token
}
