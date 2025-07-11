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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	regstryapiv2 "github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/registry"
	"github.com/docker/go-connections/sockets"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	swr "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/swr/v2/model"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type Endpoint struct {
	Addr      string
	Ak        string
	Sk        string
	Region    string
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
	ValidateRegistry(ep Endpoint, log *zap.SugaredLogger) error
	ListRepoImages(option ListRepoImagesOption, log *zap.SugaredLogger) (*ReposResp, error)
	GetImageInfo(option GetRepoImageDetailOption, log *zap.SugaredLogger) (*commonmodels.DeliveryImage, error)
}

func NewV2Service(provider string, tlsEnabled bool, tlsCert string) Service {
	// since SWR & AWS services are provided by known provider, we assume it is signed by well known CA
	switch provider {
	case config.RegistryTypeSWR:
		return &swrService{}
	case config.RegistryTypeAWS:
		return &ecrService{}
	default:
		return &v2RegistryService{
			EnableHTTPS: tlsEnabled,
			CustomCert:  tlsCert,
		}
	}
}

type v2RegistryService struct {
	EnableHTTPS bool
	CustomCert  string
}

type authClient struct {
	endpoint    Endpoint
	endpointURL *url.URL
	cm          challenge.Manager
	tr          http.RoundTripper

	ctx context.Context
	log *zap.SugaredLogger
}

func (s *v2RegistryService) createClient(ep Endpoint, logger *zap.SugaredLogger) (cli *authClient, err error) {
	endpointURL, err := url.Parse(ep.Addr)
	if err != nil {
		return
	}

	ctx := context.Background()
	direct := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tlsConfig := &tls.Config{}

	if s.EnableHTTPS && s.CustomCert != "" {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(s.CustomCert))
		tlsConfig.RootCAs = caCertPool
	} else if !s.EnableHTTPS {
		tlsConfig.InsecureSkipVerify = true
	}

	base := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         direct.DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
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
	challengeManager, err := registry.PingV2Registry(endpointURL, authTransport)

	if err != nil {
		if responseErr, ok := err.(registry.PingResponseError); ok {
			err = responseErr.Err
		}
		return
	}

	cli = &authClient{
		endpoint:    ep,
		endpointURL: endpointURL,
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

	repo, err = client.NewRepository(repoNameRef, c.endpointURL.String(), tr)
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

func (c *authClient) validateRegistry(repoName string) (err error) {
	repo, err := c.getRepository(repoName + "/test")
	if err != nil {
		return
	}

	// Try to list tags to verify repository access
	_, err = repo.Tags(c.ctx).All(c.ctx)
	if err != nil {
		if strings.Contains(err.Error(), regstryapiv2.ErrorCodeNameUnknown.Message()) {
			return nil
		}
		return errors.Wrap(err, "验证镜像仓库失败")
	}

	return nil
}

func (s *v2RegistryService) ValidateRegistry(ep Endpoint, log *zap.SugaredLogger) (err error) {
	c, err := s.createClient(ep, log)
	if err != nil {
		return
	}

	err = c.validateRegistry(ep.Namespace)
	if err != nil {
		return
	}

	return
}

func (s *v2RegistryService) GetImageInfo(option GetRepoImageDetailOption, log *zap.SugaredLogger) (di *commonmodels.DeliveryImage, err error) {
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

func (s *v2RegistryService) ListRepoImages(option ListRepoImagesOption, log *zap.SugaredLogger) (resp *ReposResp, err error) {
	cli, err := s.createClient(option.Endpoint, log)
	if err != nil {
		return
	}

	var wg wait.Group
	var mutex sync.RWMutex
	resp = &ReposResp{Total: len(option.Repos)}
	resultChan := make(chan *Repo)
	defer close(resultChan)

	for _, repo := range option.Repos {
		name := repo
		wg.Start(func() {
			repoName := fmt.Sprintf("%s/%s", option.Namespace, name)
			tags, err := cli.listTags(repoName)
			if err != nil {
				log.Errorf("failed to list tags of %s: %s", repoName, err)
				return
			}

			var koderoverTags, customTags, sortedTags []string
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
			sortedTags = append(koderoverTags, customTags...)

			mutex.Lock()
			resp.Repos = append(resp.Repos, &Repo{
				Name:      name,
				Namespace: option.Namespace,
				Tags:      sortedTags,
			})
			mutex.Unlock()
		})
	}

	wg.Wait()

	return resp, nil
}

type swrService struct {
}

func (s *swrService) createClient(ep Endpoint) (cli *swr.SwrClient) {
	endpoint := fmt.Sprintf("https://swr-api.%s.myhuaweicloud.com", ep.Region)
	auth := basic.NewCredentialsBuilder().
		WithAk(ep.Ak).
		WithSk(ep.Sk).
		Build()

	client := swr.NewSwrClient(
		swr.SwrClientBuilder().
			WithEndpoint(endpoint).
			WithCredential(auth).
			Build())
	return client
}

func (s *swrService) ValidateRegistry(ep Endpoint, log *zap.SugaredLogger) (err error) {
	svc := s.createClient(ep)

	req := &model.ListNamespacesRequest{}
	_, err = svc.ListNamespaces(req)
	if err != nil {
		return fmt.Errorf("list namespaces error: %s", err)
	}

	return nil
}

func (s *swrService) ListRepoImages(option ListRepoImagesOption, log *zap.SugaredLogger) (resp *ReposResp, err error) {
	swrCli := s.createClient(option.Endpoint)

	var wg wait.Group
	var mutex sync.RWMutex
	resp = &ReposResp{Total: len(option.Repos)}
	resultChan := make(chan *Repo)
	defer close(resultChan)

	for _, repo := range option.Repos {
		name := repo
		wg.Start(func() {
			request := &model.ListReposDetailsRequest{Name: &name, Namespace: &option.Namespace, ContentType: model.GetListReposDetailsRequestContentTypeEnum().APPLICATION_JSONCHARSETUTF_8}
			repoDetails, err := swrCli.ListReposDetails(request)
			if err != nil {
				log.Errorf("failed to list tags of %s: %s", name, err)
				return
			}

			var koderoverTags, customTags, sortedTags []string
			for _, repoResp := range *repoDetails.Body {
				if repoResp.Name != name {
					continue
				}
				for _, tag := range repoResp.Tags {
					tagArray := strings.Split(tag, "-")
					if len(tagArray) > 1 && len(tagArray[0]) == 14 {
						if _, err := time.Parse("20060102150405", tagArray[0]); err == nil {
							koderoverTags = append(koderoverTags, tag)
							continue
						}
					}
					customTags = append(customTags, tag)
				}
			}

			sort.Sort(sort.Reverse(sort.StringSlice(koderoverTags)))
			sortedTags = append(koderoverTags, customTags...)

			mutex.Lock()
			resp.Repos = append(resp.Repos, &Repo{
				Name:      name,
				Namespace: option.Namespace,
				Tags:      sortedTags,
			})
			mutex.Unlock()
		})
	}

	wg.Wait()

	return resp, nil
}

func (s *swrService) GetImageInfo(option GetRepoImageDetailOption, log *zap.SugaredLogger) (di *commonmodels.DeliveryImage, err error) {
	swrCli := s.createClient(option.Endpoint)

	request := &model.ListRepositoryTagsRequest{Tag: &option.Tag, Namespace: option.Namespace, Repository: option.Image}
	repoTags, err := swrCli.ListRepositoryTags(request)
	if err != nil {
		err = errors.Wrapf(err, "failed to get image info of %s:%s", option.Image, option.Tag)
		return
	}

	for _, repoTag := range *repoTags.Body {
		return &commonmodels.DeliveryImage{
			RepoName:     option.Image,
			TagName:      option.Tag,
			CreationTime: repoTag.Created,
			ImageDigest:  repoTag.Digest,
			ImageSize:    repoTag.Size,
		}, nil
	}

	return &commonmodels.DeliveryImage{}, nil
}

type ecrService struct {
}

func (s *ecrService) getECRService(ep Endpoint, log *zap.SugaredLogger) (*ecr.ECR, error) {
	creds := credentials.NewStaticCredentials(ep.Ak, ep.Sk, "")
	config := &aws.Config{
		Region:      aws.String(ep.Region),
		Credentials: creds,
	}
	sess, err := session.NewSession(config)
	if err != nil {
		log.Errorf("Failed to create aws session, err: %s", err)
		return nil, err
	}
	return ecr.New(sess), nil
}

func (s *ecrService) ValidateRegistry(ep Endpoint, log *zap.SugaredLogger) (err error) {
	svc, err := s.getECRService(ep, log)
	if err != nil {
		return err
	}

	req := &ecr.DescribeRegistryInput{}
	_, err = svc.DescribeRegistry(req)
	if err != nil {
		return fmt.Errorf("describe registry error: %s", err)
	}

	return nil
}

func (s *ecrService) ListRepoImages(option ListRepoImagesOption, log *zap.SugaredLogger) (resp *ReposResp, err error) {
	svc, err := s.getECRService(option.Endpoint, log)
	if err != nil {
		return nil, err
	}

	praseNamespace := func(endpoint string) (string, error) {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		endpoint = strings.TrimPrefix(endpoint, "https://")
		parts := strings.Split(endpoint, "/")
		if len(parts) == 2 {
			return parts[1], nil
		}
		return "", fmt.Errorf("endpoint %s has no namespace", endpoint)
	}

	namespace, err := praseNamespace(option.Endpoint.Addr)
	if err != nil {
		return nil, err
	}

	var wg wait.Group
	var mutex sync.RWMutex
	resp = &ReposResp{Total: len(option.Repos)}
	resultChan := make(chan *Repo)
	defer close(resultChan)

	for _, repo := range option.Repos {
		name := repo
		wg.Start(func() {
			input := &ecr.ListImagesInput{
				RepositoryName: aws.String(fmt.Sprintf("%s/%s", namespace, name)),
			}
			result, err := svc.ListImages(input)
			if err != nil {
				log.Errorf("Failed to get image information from aws, error: %s", err)
				return
			}
			var koderoverTags, customTags, sortedTags []string
			for _, image := range result.ImageIds {
				tagArray := strings.Split(aws.StringValue(image.ImageTag), "-")
				if len(tagArray) > 1 && len(tagArray[0]) == 14 {
					if _, err := time.Parse("20060102150405", tagArray[0]); err == nil {
						koderoverTags = append(koderoverTags, aws.StringValue(image.ImageTag))
						continue
					}
				}
				customTags = append(customTags, aws.StringValue(image.ImageTag))
			}

			sort.Sort(sort.Reverse(sort.StringSlice(koderoverTags)))
			sortedTags = append(koderoverTags, customTags...)

			mutex.Lock()
			resp.Repos = append(resp.Repos, &Repo{
				Name: name,
				Tags: sortedTags,
			})
			mutex.Unlock()
		})

	}
	wg.Wait()

	return resp, nil
}

func (s *ecrService) GetImageInfo(option GetRepoImageDetailOption, log *zap.SugaredLogger) (di *commonmodels.DeliveryImage, err error) {
	svc, err := s.getECRService(option.Endpoint, log)
	if err != nil {
		return nil, err
	}
	input := &ecr.DescribeImagesInput{
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(option.Tag),
			},
		},
		RepositoryName: aws.String(option.Image),
	}
	result, err := svc.DescribeImages(input)
	if err != nil {
		err = errors.Wrapf(err, "failed to get image info of %s:%s", option.Image, option.Tag)
		return
	}
	// since only one image tag is passed, only one image detail will be in this detail list
	// so only the first one will be used
	for _, imageDetail := range result.ImageDetails {
		return &commonmodels.DeliveryImage{
			RepoName:     option.Image,
			TagName:      option.Tag,
			CreationTime: imageDetail.ImagePushedAt.String(),
			ImageDigest:  aws.StringValue(imageDetail.ImageDigest),
			ImageSize:    aws.Int64Value(imageDetail.ImageSizeInBytes),
		}, nil
	}
	return &commonmodels.DeliveryImage{}, nil
}
