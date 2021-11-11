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
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.AddHandler(upgradepath.V160, upgradepath.V170, V160ToV170)
	upgradepath.AddHandler(upgradepath.V170, upgradepath.V160, V170ToV160)
}

// V160ToV170 refreshes the secret of all webhooks
func V160ToV170() error {
	log.Info("Migrating data from 1.6.0 to 1.7.0")

	err := refreshWebHookSecret()
	if err != nil {
		log.Errorf("Failed to refresh webhook secret, err: %s", err)
		return err
	}

	return nil
}

func V170ToV160() error {
	log.Info("Rollback data from 1.7.0 to 1.6.0")
	return nil
}

type hookRefresher interface {
	RefreshWebHookSecret(owner, repo, hookID string) error
}

func refreshWebHookSecret() error {
	hooks, err := mongodb.NewWebHookColl().List()
	if err != nil {
		log.Errorf("Failed to list webhooks, err: %s", err)
		return err
	}

	codeHosts, err := systemconfig.New().ListCodeHosts()
	if err != nil {
		log.Errorf("Failed to list codehosts, err: %s", err)
		return err
	}

	cs := toCodeHostSet(codeHosts)
	for _, hook := range hooks {
		var cl hookRefresher
		ch, ok := cs[codeHostUniqueID{address: hook.Address, owner: hook.Owner}]
		if !ok {
			log.Warnf("Failed to get codehost for %s/%s", hook.Address, hook.Owner)
			continue
		}

		switch ch.Type {
		case setting.SourceFromGithub:
			cl = github.NewClient(ch.accessToken, config.ProxyHTTPSAddr())
		case setting.SourceFromGitlab:
			cl, err = gitlab.NewClient(ch.address, ch.accessToken)
			if err != nil {
				log.Warnf("Failed to create gitlab client, err: %s", err)
				continue
			}
		default:
			log.Warnf("Invalid type: %s", ch.Type)
			continue
		}

		if err = cl.RefreshWebHookSecret(hook.Owner, hook.Repo, hook.HookID); err != nil {
			log.Warnf("Failed to refresh webhook secret for hook %d in %s/%s, err: %s", hook.HookID, hook.Owner, hook.Repo, err)
			continue
		}
	}

	return nil
}

type codeHostUniqueID struct {
	address, owner string
}

type codeHostItem struct {
	codeHostUniqueID
	accessToken string
	Type        string
}

type codeHostSet map[codeHostUniqueID]codeHostItem

// NewCodeHostSet creates a HookSet from a list of values.
func NewCodeHostSet(items ...codeHostItem) codeHostSet {
	ss := codeHostSet{}
	ss.Insert(items...)
	return ss
}

// Insert adds items to the set.
func (s codeHostSet) Insert(items ...codeHostItem) codeHostSet {
	for _, item := range items {
		s[item.codeHostUniqueID] = item
	}
	return s
}

func toCodeHostSet(codeHosts []*systemconfig.CodeHost) codeHostSet {
	res := NewCodeHostSet()
	for _, h := range codeHosts {
		res.Insert(codeHostItem{
			codeHostUniqueID: codeHostUniqueID{
				address: h.Address,
				owner:   h.Namespace,
			},
			accessToken: h.AccessToken,
			Type:        h.Type,
		})
	}

	return res
}
