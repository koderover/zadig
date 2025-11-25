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

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	typesregistry "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/v2/pkg/microservice/packager/config"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type Packager struct {
	Ctx *Context
}

type PackageResult struct {
	ServiceName string       `json:"service_name"`
	Result      string       `json:"result"`
	ErrorMsg    string       `json:"error_msg"`
	ImageData   []*ImageData `json:"image_data"`
}

func NewPackager() (*Packager, error) {
	contextData, err := os.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, err
	}

	var ctx *Context
	if err := yaml.Unmarshal(contextData, &ctx); err != nil {
		return nil, err
	}

	packager := &Packager{
		Ctx: ctx,
	}

	return packager, nil
}

func buildRegistryMap(registries []*DockerRegistry) map[string]*DockerRegistry {
	ret := make(map[string]*DockerRegistry)
	for _, registry := range registries {
		ret[registry.RegistryID] = registry
	}
	return ret
}

func base64EncodeAuth(auth *typesregistry.AuthConfig) (string, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(auth); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf.Bytes()), nil
}

func buildTargetImage(imageName, imageTag, host, nameSpace string) string {
	ret := ""
	if len(nameSpace) > 0 {
		ret = fmt.Sprintf("%s/%s/%s:%s", host, nameSpace, imageName, imageTag)
	} else {
		ret = fmt.Sprintf("%s/%s:%s", host, imageName, imageTag)
	}
	ret = strings.TrimPrefix(ret, "http://")
	ret = strings.TrimPrefix(ret, "https://")
	return ret
}

func ExtractErrorDetail(in io.Reader) error {
	dec := json.NewDecoder(in)
	for dec.More() {
		var jm jsonmessage.JSONMessage
		if err := dec.Decode(&jm); err != nil {
			return err
		}
		if jm.Error != nil {
			return jm.Error
		}
	}
	return nil
}

func pullImage(dockerClient *client.Client, imageUrl string, options *types.ImagePullOptions) error {
	log.Infof("pulling image: %s", imageUrl)
	pullResponse, err := dockerClient.ImagePull(context.TODO(), imageUrl, *options)
	if err != nil {
		return err
	}

	defer pullResponse.Close()

	err = ExtractErrorDetail(pullResponse)
	return err
}

func pushImage(dockerClient *client.Client, targetImageUrl string, options *types.ImagePushOptions) error {
	log.Infof("pushing image: %s", targetImageUrl)
	pushResponse, err := dockerClient.ImagePush(context.TODO(), targetImageUrl, *options)
	if err != nil {
		return errors.Wrapf(err, "failed to push image: %s", targetImageUrl)
	}

	defer pushResponse.Close()

	err = ExtractErrorDetail(pushResponse)
	return err
}

func handleSingleService(imageByService *ImagesByService, allRegistries map[string]*DockerRegistry, targetRegistries []*DockerRegistry, dockerClient *client.Client) ([]*ImageData, error) {
	targetImageUrlByRepo := make(map[string][]string)
	retImages := make([]*ImageData, 0)

	for _, singleImage := range imageByService.Images {
		options := types.ImagePullOptions{}
		// for images from public repo，registryID won't be appointed
		if len(singleImage.RegistryID) > 0 {
			registryInfo, ok := allRegistries[singleImage.RegistryID]
			if !ok {
				return nil, fmt.Errorf("failed to find source registry for image: %s", singleImage.ImageUrl)
			}
			encodedAuth, err := base64EncodeAuth(&typesregistry.AuthConfig{
				Username:      registryInfo.UserName,
				Password:      registryInfo.Password,
				ServerAddress: registryInfo.Host,
			})
			if err != nil {
				return nil, errors.Wrapf(err, "faied to create docker pull auth data")
			}
			options.RegistryAuth = encodedAuth
		}

		// pull image
		err := pullImage(dockerClient, singleImage.ImageUrl, &options)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to pull image: %s", singleImage.ImageUrl)
		}

		// tag image
		for _, registry := range targetRegistries {
			targetImage := buildTargetImage(singleImage.ImageName, singleImage.CustomTag, registry.Host, registry.Namespace)
			err = dockerClient.ImageTag(context.TODO(), singleImage.ImageUrl, targetImage)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to tag image from: %s to: %s", singleImage.ImageUrl, targetImage)
			}
			targetImageUrlByRepo[registry.RegistryID] = append(targetImageUrlByRepo[registry.RegistryID], targetImage)
			retImages = append(retImages, &ImageData{
				ImageUrl:   targetImage,
				ImageName:  singleImage.ImageName,
				ImageTag:   singleImage.ImageTag,
				CustomTag:  singleImage.CustomTag,
				RegistryID: singleImage.RegistryID,
			})
		}
	}

	// push image
	for _, registry := range targetRegistries {
		encodedAuth, err := base64EncodeAuth(&typesregistry.AuthConfig{
			Username:      registry.UserName,
			Password:      registry.Password,
			ServerAddress: registry.Host,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "faied to create docker push auth data")
		}
		options := types.ImagePushOptions{
			RegistryAuth: encodedAuth,
		}

		for _, targetImageUrl := range targetImageUrlByRepo[registry.RegistryID] {
			err = pushImage(dockerClient, targetImageUrl, &options)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to push image: %s", targetImageUrl)
			}
		}
	}
	return retImages, nil
}

func (p *Packager) Exec() error {

	// init docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Errorf("failed to init docker client %s", err)
		return err
	}

	defer func() {
		err = cli.Close()
		if err != nil {
			log.Errorf("failed to close client: %s", err)
		}
	}()

	// run docker info to ensure docker daemon connection
	for i := 0; i < 120; i++ {
		_, err := cli.Info(context.TODO())
		if err == nil {
			break
		}
		log.Errorf("failed tor run docker info, try index: %d, err: %s", i, err)
		time.Sleep(time.Second * 1)
	}

	// create log file
	if len(p.Ctx.ProgressFile) >= 0 {
		if err := os.MkdirAll(filepath.Dir(p.Ctx.ProgressFile), 0770); err != nil {
			return errors.Wrapf(err, "failed to create progress file dir")
		}
		file, err := os.Create(p.Ctx.ProgressFile)
		if err != nil {
			return errors.Wrapf(err, "failed to create progress file")
		}
		if err = file.Close(); err != nil {
			return errors.Wrapf(err, "failed to close progress file")
		}
	}

	allRegistries := append(p.Ctx.SourceRegistries, p.Ctx.TargetRegistries...)
	realTimeProgress := make([]*PackageResult, 0)

	for _, imageByService := range p.Ctx.Images {
		result := &PackageResult{
			ServiceName: imageByService.ServiceName,
		}
		images, err := handleSingleService(imageByService, buildRegistryMap(allRegistries), p.Ctx.TargetRegistries, cli)

		if err != nil {
			result.Result = "failed"
			result.ErrorMsg = err.Error()
			log.Errorf("[result][fail][%s][%s]", imageByService.ServiceName, err)
		} else {
			result.Result = "success"
			result.ImageData = images
			log.Infof("[result][success][%s]", imageByService.ServiceName)
		}

		realTimeProgress = append(realTimeProgress, result)

		if len(p.Ctx.ProgressFile) > 0 {
			bs, err := json.Marshal(realTimeProgress)
			if err != nil {
				log.Errorf("failed to marshal progress data %s", err)
				continue
			}
			err = os.WriteFile(p.Ctx.ProgressFile, bs, 0644)
			if err != nil {
				log.Errorf("failed to write progress data %s", err)
				continue
			}
		}
	}

	// keep job alive for extra 10 seconds to make the runner be able to catch all progress info
	// TODO need optimize
	time.Sleep(time.Second * 10)
	return nil
}
