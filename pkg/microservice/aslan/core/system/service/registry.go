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
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/registry"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/util"
)

type RepoImgResp struct {
	Host    string `json:"host"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	Created string `json:"created"`
	Digest  string `json:"digest"`
}

type RepoInfo struct {
	RegType      string `json:"regType"`
	RegProvider  string `json:"regProvider"`
	RegNamespace string `json:"regNamespace"`
	RegAddr      string `json:"regAddr"`
	ID           string `json:"id"`
}

const (
	PERPAGE = 100
	PAGE    = 1
)

// ImageTagsReqStatus its a global map to store the status of the image list request
var ImageTagsReqStatus = struct {
	M    sync.Mutex
	List map[string]struct{}
}{
	M:    sync.Mutex{},
	List: make(map[string]struct{}),
}

func ListRegistriesByProject(projectName string, log *zap.SugaredLogger) ([]*commonmodels.RegistryNamespace, error) {
	registryNamespaces, err := commonservice.ListRegistryByProject(projectName, log)
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return registryNamespaces, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}

	for _, registryNamespace := range registryNamespaces {
		registryNamespace.AccessKey = ""
		registryNamespace.SecretKey = ""
		registryNamespace.Projects = nil
	}
	return registryNamespaces, nil
}

// ListRegistries 为了抹掉ak和sk的数据
func ListRegistries(log *zap.SugaredLogger) ([]*commonmodels.RegistryNamespace, error) {
	registryNamespaces, err := commonservice.ListRegistryNamespaces("", false, log)
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return registryNamespaces, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}
	for _, registryNamespace := range registryNamespaces {
		registryNamespace.AccessKey = ""
		registryNamespace.SecretKey = ""
	}
	return registryNamespaces, nil
}

func CreateRegistryNamespace(username string, args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	regOps := new(commonrepo.FindRegOps)
	regOps.IsDefault = true
	defaultReg, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			log.Errorf("failed to find default default registry")
			return err
		}
	}
	if args.IsDefault {
		if defaultReg != nil {
			defaultReg.IsDefault = false
			err := UpdateRegistryNamespaceDefault(defaultReg, log)
			if err != nil {
				log.Errorf("updateRegistry error: %v", err)
				return fmt.Errorf("RegistryNamespace.Create error: %v", err)
			}
		}
	}

	args.UpdateBy = username
	args.Namespace = strings.TrimSpace(args.Namespace)

	if err := commonrepo.NewRegistryNamespaceColl().Create(args); err != nil {
		log.Errorf("RegistryNamespace.Create error: %v", err)
		return fmt.Errorf("RegistryNamespace.Create error: %v", err)
	}

	return commonutil.SyncDinDForRegistries()
}

func UpdateRegistryNamespace(username, id string, args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	defaultReg, err := commonservice.FindDefaultRegistry(false, log)
	if err != nil {
		if err != mongo.ErrNoDocuments && err != mongo.ErrNilDocument {
			log.Errorf("failed to find default default registry")
			return err
		}
	}
	if args.IsDefault {
		if defaultReg != nil {
			defaultReg.IsDefault = false
			err := UpdateRegistryNamespaceDefault(defaultReg, log)
			if err != nil {
				log.Errorf("updateRegistry error: %v", err)
				return fmt.Errorf("RegistryNamespace.Update error: %v", err)
			}
		}
	}

	originReg, err := commonrepo.NewRegistryNamespaceColl().Find(&commonrepo.FindRegOps{ID: id})
	if err != nil {
		return fmt.Errorf("failed to find registry, id: %s, err: %v", id, err)
	}

	args.UpdateBy = username
	args.Namespace = strings.TrimSpace(args.Namespace)

	if err := commonrepo.NewRegistryNamespaceColl().Update(id, args); err != nil {
		log.Errorf("RegistryNamespace.Update error: %v", err)
		return fmt.Errorf("RegistryNamespace.Update error: %v", err)
	}

	util.Go(func() {
		if originReg.RegAddr == args.RegAddr && originReg.AccessKey == args.AccessKey && originReg.SecretKey == args.SecretKey {
			return
		}

		envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{})
		if err != nil {
			log.Errorf("[UpdateRegistryNamespace] failed to list all envs, err: %v", err)
			return
		}

		for _, env := range envs {
			if env.RegistryID == id {
				kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(env.ClusterID)
				if err != nil {
					log.Errorf("[UpdateRegistryNamespace] GetKubeClient %s error: %v", env.ClusterID, err)
					continue
				}

				err = kube.CreateOrUpdateDefaultRegistrySecret(env.Namespace, args, kubeClient)
				if err != nil {
					log.Errorf("[UpdateRegistryNamespaces] CreateOrUpdateDefaultRegistrySecret, namespace: %s, regID: %s error: %s", env.Namespace, id, err)
				}
			}
		}
	})

	return commonutil.SyncDinDForRegistries()
}

func ValidateRegistryNamespace(args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	regService := registry.NewV2Service(args.RegProvider, args.AdvancedSetting.TLSEnabled, args.AdvancedSetting.TLSCert)
	endPoint := registry.Endpoint{
		Addr:      args.RegAddr,
		Ak:        args.AccessKey,
		Sk:        args.SecretKey,
		Namespace: args.Namespace,
		Region:    args.Region,
	}

	err := regService.ValidateRegistry(endPoint, log)
	if err != nil {
		return fmt.Errorf("验证镜像仓库失败: %s", err)
	}

	return nil
}

func DeleteRegistryNamespace(id string, log *zap.SugaredLogger) error {
	registries, err := commonrepo.NewRegistryNamespaceColl().FindAll(&commonrepo.FindRegOps{})
	if err != nil {
		log.Errorf("RegistryNamespace.FindAll error: %s", err)
		return err
	}
	var (
		isDefault          = false
		registryNamespaces []*commonmodels.RegistryNamespace
	)
	envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		ExcludeStatus: []string{setting.ProductStatusDeleting},
	})

	for _, env := range envs {
		if env.RegistryID == id {
			return errors.New("The registry cannot be deleted, it's being used by environment")
		}
	}

	// whether it is the default registry
	for _, registry := range registries {
		if registry.ID.Hex() == id && registry.IsDefault {
			isDefault = true
			continue
		}
		registryNamespaces = append(registryNamespaces, registry)
	}

	if err := commonrepo.NewRegistryNamespaceColl().Delete(id); err != nil {
		log.Errorf("RegistryNamespace.Delete error: %s", err)
		return err
	}

	if isDefault && len(registryNamespaces) > 0 {
		registryNamespaces[0].IsDefault = true
		if err := commonrepo.NewRegistryNamespaceColl().Update(registryNamespaces[0].ID.Hex(), registryNamespaces[0]); err != nil {
			log.Errorf("RegistryNamespace.Update error: %s", err)
			return err
		}
	}
	return commonutil.SyncDinDForRegistries()
}

func ListAllRepos(log *zap.SugaredLogger) ([]*RepoInfo, error) {
	repoInfos := make([]*RepoInfo, 0)
	resp, err := commonservice.ListRegistryNamespaces("", false, log)
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %v", err)
		return nil, fmt.Errorf("RegistryNamespace.List error: %v", err)
	}

	for _, rn := range resp {
		repoInfo := new(RepoInfo)
		repoInfo.RegProvider = rn.RegProvider
		repoInfo.RegType = rn.RegType
		repoInfo.RegNamespace = rn.Namespace
		repoInfo.RegAddr = rn.RegAddr
		repoInfo.ID = rn.ID.Hex()

		repoInfos = append(repoInfos, repoInfo)
	}
	return repoInfos, nil
}

type imageReqJob struct {
	regService   registry.Service
	endPoint     registry.Endpoint
	registryInfo *models.RegistryNamespace
	repoImages   *registry.Repo
	tag          string
	failedTimes  int
}

func goWithRecover(fn func(), logger *zap.SugaredLogger) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("Recovered from panic: %v", r)
			}
		}()

		fn()
	}()
}

// ListReposTags TODO: need to be optimized
func ListReposTags(registryInfo *commonmodels.RegistryNamespace, names []string, logger *zap.SugaredLogger) ([]*RepoImgResp, error) {
	var regService registry.Service
	images := make([]*RepoImgResp, 0)
	if registryInfo.AdvancedSetting != nil {
		regService = registry.NewV2Service(registryInfo.RegProvider, registryInfo.AdvancedSetting.TLSEnabled, registryInfo.AdvancedSetting.TLSCert)
	} else {
		regService = registry.NewV2Service(registryInfo.RegProvider, true, "")
	}

	endPoint := registry.Endpoint{
		Addr:      registryInfo.RegAddr,
		Ak:        registryInfo.AccessKey,
		Sk:        registryInfo.SecretKey,
		Namespace: registryInfo.Namespace,
		Region:    registryInfo.Region,
	}

	repos, err := regService.ListRepoImages(registry.ListRepoImagesOption{
		Endpoint: endPoint,
		Repos:    names,
	}, logger)
	if err != nil {
		return images, e.ErrListImages.AddErr(err)
	}

	resultChan := make(chan []*RepoImgResp, 1)
	errChan := make(chan error, 1)

	goWithRecover(func() {
		imagesProcessor(repos, registryInfo, regService, resultChan, errChan, logger)
	}, logger)

	ticker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-ticker.C:
			logger.Infof("ListReposTags: timeout, only update image_tags db and return image list by old sort method")
			// sort tags by old method
			for _, repo := range repos.Repos {
				for _, tag := range repo.Tags {
					img := &RepoImgResp{
						Host:  util.TrimURLScheme(registryInfo.RegAddr),
						Owner: repo.Namespace,
						Name:  repo.Name,
						Tag:   tag,
					}
					images = append(images, img)
				}
			}
			return images, nil
		case images = <-resultChan:
			// sort tags by created time
			sort.Slice(images, func(i, j int) bool {
				return images[i].Created > images[j].Created
			})
			return images, nil
		case err := <-errChan:
			return nil, err
		}
	}
}

func CheckReqLimit(key string) bool {
	// check the imageList request status quickly
	if _, ok := ImageTagsReqStatus.List[key]; !ok {
		ImageTagsReqStatus.M.Lock()
		defer ImageTagsReqStatus.M.Unlock()
		// double check
		if _, ok := ImageTagsReqStatus.List[key]; !ok {
			ImageTagsReqStatus.List[key] = struct{}{}
			return true
		} else {
			return false
		}
	}
	return false
}

func imagesProcessor(repos *registry.ReposResp, registryInfo *commonmodels.RegistryNamespace, regService registry.Service, result chan []*RepoImgResp, errChan chan error, logger *zap.SugaredLogger) {
	// need to consider whether close the channel directly can cause memory leak
	defer func() {
		close(result)
		close(errChan)
	}()

	images := make([]*RepoImgResp, 0)
	for _, repo := range repos.Repos {
		// find all tags from image tags db
		opts := commonrepo.ImageTagsFindOption{
			RegistryID:  registryInfo.ID.Hex(),
			RegProvider: registryInfo.RegProvider,
			ImageName:   repo.Name,
			Namespace:   registryInfo.Namespace,
		}
		key := fmt.Sprintf("%s-%s-%s-%s", registryInfo.ID.Hex(), repo.Name, registryInfo.RegProvider, registryInfo.Namespace)
		dbTags, err := getImageTagListFromDB(opts, key, 0, logger)
		if err != nil {
			logger.Errorf("get image[%s] tags from db error: %v", repo.Name, err)
			errChan <- err
			return
		}

		tags := ImageListGetter(repo, registryInfo, dbTags, regService, logger)
		// update all imageTags to db,
		goWithRecover(func() {
			insertNewTags2DB(tags, registryInfo, key, logger)
		}, logger)

		images = append(images, tags...)
	}

	result <- images
}

func getImageTagListFromDB(opts commonrepo.ImageTagsFindOption, key string, repTime int, logger *zap.SugaredLogger) (map[string]*commonmodels.ImageTag, error) {
	imageTags, err := commonrepo.NewImageTagsCollColl().Find(&opts)
	dbTags := make(map[string]*commonmodels.ImageTag, 0)
	if err != nil {
		// if the error is not mongo.ErrNoDocuments or mongo.ErrNilDocument, we write the error to log and get image tags detail from registry directly.
		if err == mongo.ErrNoDocuments || err == mongo.ErrNilDocument {
			if !CheckReqLimit(key) {
				if repTime > 0 {
					return nil, fmt.Errorf("getting the image list timed out, please try again later")
				}
				ticker := time.NewTicker(time.Second * 20)
				for {
					select {
					case <-ticker.C:
						return nil, fmt.Errorf("getting the image list timed out, please try again later")
					default:
						// there is no need to get lock, because another goroutine get all image tags from registry and write to db,then delete the key from ImageTagsReqStatus.List
						if _, ok := ImageTagsReqStatus.List[key]; !ok {
							// go to get image tag detail
							logger.Infof("start to get image tags detail again")
							return getImageTagListFromDB(opts, key, repTime+1, logger)
						} else {
							// wait for image tag detail
							logger.Infof("start to sleep 500ms for waitting for image tags detail")
							time.Sleep(time.Millisecond * 500)
						}
					}
				}
			}
		} else {
			return nil, err
		}
	} else {
		for _, imageTag := range imageTags.ImageTags {
			dbTags[imageTag.TagName] = imageTag
		}
	}
	return dbTags, nil
}

func ImageListGetter(repo *registry.Repo, registryInfo *commonmodels.RegistryNamespace, dbTags map[string]*models.ImageTag, regService registry.Service, logger *zap.SugaredLogger) []*RepoImgResp {
	jobChan := make(chan *imageReqJob, len(repo.Tags))
	resultChan := make(chan *RepoImgResp, 10)
	defer func() {
		close(jobChan)
		close(resultChan)
	}()
	images := make([]*RepoImgResp, 0)

	jobCount := 0
	for _, tag := range repo.Tags {
		if t, ok := dbTags[tag]; ok && t.Digest != "" && t.Created != "" {
			resp := &RepoImgResp{
				Host:    util.TrimURLScheme(registryInfo.RegAddr),
				Owner:   repo.Namespace,
				Name:    repo.Name,
				Tag:     t.TagName,
				Created: t.Created,
				Digest:  t.Digest,
			}
			images = append(images, resp)
			continue
		}

		jobCount++
		jobChan <- &imageReqJob{
			regService: regService,
			endPoint: registry.Endpoint{
				Addr:      registryInfo.RegAddr,
				Ak:        registryInfo.AccessKey,
				Sk:        registryInfo.SecretKey,
				Namespace: registryInfo.Namespace,
				Region:    registryInfo.Region,
			},
			registryInfo: registryInfo,
			repoImages:   repo,
			tag:          tag,
		}
	}

	if jobCount > 0 {
		// create gcount goroutines to get tags from registry
		gcount := 50
		if jobCount < 50 {
			gcount = jobCount
		}
		for i := 0; i < gcount; i++ {
			goWithRecover(func() {
				getNewTagsFromReg(jobChan, resultChan, logger)
			}, logger)
		}

		for jobCount > 0 {
			select {
			case result := <-resultChan:
				images = append(images, result)
				jobCount--
			}
		}
	}

	return images
}

func getNewTagsFromReg(job chan *imageReqJob, result chan *RepoImgResp, logger *zap.SugaredLogger) {
	for data := range job {
		option := registry.GetRepoImageDetailOption{
			Image:    data.repoImages.Name,
			Tag:      data.tag,
			Endpoint: data.endPoint,
		}
		details, err := data.regService.GetImageInfo(option, logger)
		if err != nil {
			// if get image detail failed, we will retry 3 times
			if data.failedTimes < 3 {
				data.failedTimes++
				job <- data
				continue
			}
			logger.Errorf("get image[%s:%s] detail from registry failed, the error is: %v", data.repoImages.Name, data.tag, err)
		}

		img := &RepoImgResp{
			Host:  util.TrimURLScheme(data.registryInfo.RegAddr),
			Owner: data.repoImages.Namespace,
			Name:  data.repoImages.Name,
			Tag:   data.tag,
		}
		if details != nil {
			img.Created = details.CreationTime
			img.Digest = details.ImageDigest
		}

		result <- img
	}
}

func insertNewTags2DB(images []*RepoImgResp, registryInfo *commonmodels.RegistryNamespace, key string, logger *zap.SugaredLogger) {
	defer func() {
		// update the ImageTagsReqStatus after insert or update the image tags to db
		if key != "" {
			ImageTagsReqStatus.M.Lock()
			delete(ImageTagsReqStatus.List, key)
			ImageTagsReqStatus.M.Unlock()
		}
	}()

	if registryInfo == nil && (images == nil || len(images) == 0) {
		return
	}

	imageTag := &models.ImageTags{}
	imageTag.ImageName = images[0].Name
	imageTag.Namespace = images[0].Owner
	imageTag.RegistryID = registryInfo.ID.Hex()
	imageTag.RegProvider = registryInfo.RegProvider
	for _, image := range images {
		imageTag.ImageTags = append(imageTag.ImageTags, &models.ImageTag{
			TagName: image.Tag,
			Created: image.Created,
			Digest:  image.Digest,
		})
	}
	err := commonrepo.NewImageTagsCollColl().InsertOrUpdate(imageTag)
	if err != nil {
		logger.Errorf("failed to update or insert the image tags 2 image_tags db %s, the error is: %v", imageTag.ImageName, err)
		return
	}
}

func GetRepoTags(registryInfo *commonmodels.RegistryNamespace, name string, log *zap.SugaredLogger) (*registry.ImagesResp, error) {
	var resp *registry.ImagesResp
	var regService registry.Service
	if registryInfo.AdvancedSetting != nil {
		regService = registry.NewV2Service(registryInfo.RegProvider, registryInfo.AdvancedSetting.TLSEnabled, registryInfo.AdvancedSetting.TLSCert)
	} else {
		regService = registry.NewV2Service(registryInfo.RegProvider, true, "")
	}
	repos, err := regService.ListRepoImages(registry.ListRepoImagesOption{
		Endpoint: registry.Endpoint{
			Addr:      registryInfo.RegAddr,
			Ak:        registryInfo.AccessKey,
			Sk:        registryInfo.SecretKey,
			Namespace: registryInfo.Namespace,
			Region:    registryInfo.Region,
		},
		Repos: []string{name},
	}, log)

	if err != nil {
		err = e.ErrListImages.AddErr(err)
	} else {
		if len(repos.Repos) == 0 {
			err = e.ErrListImages.AddDesc(fmt.Sprintf("找不到Repo %s", name))
		} else {
			repo := repos.Repos[0]
			var images []registry.Image
			for _, tag := range repo.Tags {
				images = append(images, registry.Image{Tag: tag})
			}

			resp = &registry.ImagesResp{
				NameSpace: registryInfo.Namespace,
				Name:      name,
				Total:     len(repo.Tags),
				Images:    images,
			}
		}
	}

	return resp, err
}

func UpdateRegistryNamespaceDefault(args *commonmodels.RegistryNamespace, log *zap.SugaredLogger) error {
	if err := commonrepo.NewRegistryNamespaceColl().Update(args.ID.Hex(), args); err != nil {
		log.Errorf("UpdateRegistryNamespaceDefault.Update error: %v", err)
		return fmt.Errorf("UpdateRegistryNamespaceDefault.Update error: %v", err)
	}
	return nil
}
