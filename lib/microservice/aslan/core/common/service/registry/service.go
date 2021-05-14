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

package registry

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/sockets"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/net/proxy"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/tool/pool"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type Endpoint struct {
	Addr      string
	Ak        string
	Sk        string
	Namespace string
}

type ListRepoImagesOption struct {
	Endpoint
	Repos []string
}

type GetRepoImageDetailOption struct {
	Endpoint
	Image string
	Tag   string
}

type Service interface {
	ListRepoImages(option ListRepoImagesOption, log *xlog.Logger) (*ReposResp, error)
	GetImageInfo(option GetRepoImageDetailOption, log *xlog.Logger) (*commonmodels.DeliveryImage, error)
}

func NewV2Service() Service {
	return &v2RegistryService{}
}

type v2RegistryService struct {
}

type authClient struct {
	endpoint    Endpoint
	endpointUrl *url.URL
	cm          challenge.Manager
	tr          http.RoundTripper

	ctx context.Context
	log *xlog.Logger
}

func (s *v2RegistryService) createClient(ep Endpoint, logger *xlog.Logger) (cli *authClient, err error) {
	endpointUrl, err := url.Parse(ep.Addr)
	if err != nil {
		return
	}

	ctx := context.Background()
	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	base := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         direct.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}

	proxyDialer, err := sockets.DialerFromEnvironment(direct)
	if err == nil {
		switch pd := proxyDialer.(type) {
		case proxy.ContextDialer:
			base.DialContext = pd.DialContext
		default:
			//noinspection GoDeprecation
			base.Dial = proxyDialer.Dial
		}
	}

	authTransport := transport.NewTransport(base)
	challengeManager, _, err := registry.PingV2Registry(endpointUrl, authTransport)

	if err != nil {
		if responseErr, ok := err.(registry.PingResponseError); ok {
			err = responseErr.Err
		}
		return
	}

	cli = &authClient{
		endpoint:    ep,
		endpointUrl: endpointUrl,
		cm:          challengeManager,
		tr:          authTransport,
		ctx:         ctx,
		log:         logger,
	}

	return
}

func (c *authClient) getRepository(repoName string) (repo distribution.Repository, err error) {
	repoNameRef, err := reference.WithName(repoName)
	if err != nil {
		return
	}

	creds := registry.NewStaticCredentialStore(&types.AuthConfig{
		Username:      c.endpoint.Ak,
		Password:      c.endpoint.Sk,
		ServerAddress: c.endpoint.Addr,
	})

	basicHandler := auth.NewBasicHandler(creds)
	scope := auth.RepositoryScope{
		Repository: repoName,
		Actions:    []string{"pull"},
		Class:      "",
	}

	tokenHandlerOptions := auth.TokenHandlerOptions{
		Transport:   c.tr,
		Credentials: creds,
		Scopes:      []auth.Scope{scope},
		ClientID:    registry.AuthClientID,
	}

	tokenHandler := auth.NewTokenHandlerWithOptions(tokenHandlerOptions)
	modifier := auth.NewAuthorizer(c.cm, tokenHandler, basicHandler)
	tr := transport.NewTransport(c.tr, modifier)

	repo, err = client.NewRepository(c.ctx, repoNameRef, c.endpointUrl.String(), tr)
	if err != nil {
		return
	}

	return
}

func (c *authClient) listTags(repoName string) (tags []string, err error) {
	repo, err := c.getRepository(repoName)
	if err != nil {
		return
	}

	tags, err = repo.Tags(c.ctx).All(c.ctx)
	if err != nil {
		return
	}

	return
}

type containerInfo struct {
	Architecture  string        `json:"architecture"`
	Created       string        `json:"created"`
	Os            string        `json:"os"`
	Digest        digest.Digest `json:"-"`
	Size          int64         `json:"-"`
	DockerVersion string        `json:"docker_version"`
}

func (c *authClient) getImageInfo(repoName, tag string) (ci *containerInfo, err error) {
	repo, err := c.getRepository(repoName)
	if err != nil {
		return
	}

	manifestService, err := repo.Manifests(c.ctx)
	if err != nil {
		return
	}

	var sha digest.Digest

	m, err := manifestService.Get(c.ctx, "", distribution.WithTag(tag), client.ReturnContentDigest(&sha))
	if err != nil {
		return
	}

	// 只支持schema2
	v2, ok := m.(*schema2.DeserializedManifest)
	if !ok {
		err = errors.New("got non v2 manifest")
		return
	}

	for _, ref := range m.References() {
		if ref.MediaType == "application/vnd.docker.container.image.v1+json" {
			blobService := repo.Blobs(c.ctx)
			var data []byte
			data, err = blobService.Get(c.ctx, ref.Digest)
			if err != nil {
				return
			}
			err = json.Unmarshal(data, &ci)
			if err != nil {
				return
			}

			ci.Digest = sha

			for _, layer := range v2.Manifest.Layers {
				ci.Size += layer.Size
			}
			return
		}
	}

	err = errors.New("no container info found")
	return
}

func (s *v2RegistryService) GetImageInfo(option GetRepoImageDetailOption, log *xlog.Logger) (di *commonmodels.DeliveryImage, err error) {
	cli, err := s.createClient(option.Endpoint, log)
	if err != nil {
		return
	}

	img := strings.Join([]string{option.Namespace, option.Image}, "/")
	ci, err := cli.getImageInfo(img, option.Tag)
	if err != nil {
		err = errors.Wrapf(err, "failed to get image info of %s:%s", img, option.Tag)
		return
	}

	return &commonmodels.DeliveryImage{
		RepoName:      img,
		TagName:       option.Tag,
		Architecture:  ci.Architecture,
		CreationTime:  ci.Created,
		Os:            ci.Os,
		ImageDigest:   ci.Digest.String(),
		ImageSize:     ci.Size,
		DockerVersion: ci.DockerVersion,
	}, nil
}

type ReverseStringSlice []string

// Len is the number of elements in the collection.
func (rss ReverseStringSlice) Len() int {
	return len(rss)
}

// Less reports whether the element with
// index i should sort before the element with index j.
func (rss ReverseStringSlice) Less(i, j int) bool {
	return i > j
}

// Swap swaps the elements with indexes i and j.
func (rss ReverseStringSlice) Swap(i, j int) {
	rss[i], rss[j] = rss[j], rss[i]
}

func (s *v2RegistryService) ListRepoImages(option ListRepoImagesOption, log *xlog.Logger) (resp *ReposResp, err error) {

	cli, err := s.createClient(option.Endpoint, log)
	if err != nil {
		return
	}

	var args []pool.TaskArg
	for _, name := range option.Repos {
		args = append(args, name)
	}

	resultChan := make(chan *Repo)
	tasks := pool.MapTask(func(arg pool.TaskArg) func() error {
		return func() error {
			var err error
			name := arg.(string)
			repoName := strings.Join([]string{option.Namespace, name}, "/")

			defer func() {
				if err != nil {
					log.Infof("failed to list tags of %s: %v", repoName, err)
				}
			}()

			tags, err := cli.listTags(repoName)
			if err != nil {
				return err
			}

			koderoverTags := make([]string, 0)
			customTags := make([]string, 0)
			sortedTags := make([]string, 0)
			for _, tag := range tags {
				tagArray := strings.Split(tag, "-")
				if len(tagArray) > 1 && len(tagArray[0]) == 14 {
					if _, err := time.Parse("20060102150405", tagArray[0]); err == nil {
						koderoverTags = append(koderoverTags, tag)
						continue
					}
				}
				customTags = append(customTags, tag)
			}

			sort.Sort(sort.Reverse(sort.StringSlice(koderoverTags)))
			sortedTags = append(sortedTags, koderoverTags...)
			sortedTags = append(sortedTags, customTags...)

			resultChan <- &Repo{
				Name:      name,
				Namespace: option.Namespace,
				Tags:      sortedTags,
			}
			return nil
		}
	}, args)

	executor := pool.NewPool(tasks, 20)
	go func() {
		executor.Run()
		close(resultChan)
	}()

	resp = &ReposResp{}

	for result := range resultChan {
		resp.Repos = append(resp.Repos, result)
		resp.Total += 1
	}

	return resp, nil
}
