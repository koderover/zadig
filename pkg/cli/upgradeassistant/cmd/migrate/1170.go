/*
Copyright 2022 The KodeRover Authors.

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
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	policymongo "github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func init() {
	upgradepath.RegisterHandler("1.16.0", "1.17.0", V1160ToV1170)
	upgradepath.RegisterHandler("1.16.0", "1.15.0", V1170ToV1160)
}

func V1160ToV1170() error {
	if err := updateRolesForTesting(); err != nil {
		log.Errorf("updateRolesForTesting err:%s", err)
		return err
	}
	if err := migrateJiraInfo(); err != nil {
		log.Errorf("migrateJiraInfo err: %v", err)
		return err
	}
	if err := migrateSharedService(); err != nil {
		log.Errorf("migrateSharedService err: %v", err)
		return err
	}
	return nil
}

func V1170ToV1160() error {
	return nil
}

func updateRolesForTesting() error {
	roles, err := policymongo.NewRoleColl().List()
	if err != nil {
		return fmt.Errorf("list roles error: %s", err)
	}
	var mRoles []mongo.WriteModel
	for _, role := range roles {
		if role.Namespace != "*" {
			continue
		}
		for _, rule := range role.Rules {
			if len(rule.Resources) == 0 {
				continue
			}
			if rule.Resources[0] == "TestCenter" {
				newVerbs := []string{}
				for _, verb := range rule.Verbs {
					if verb == "get_test" {
						newVerbs = append(newVerbs, verb)
					}
				}
				rule.Verbs = newVerbs
			}
		}
		mRoles = append(mRoles,
			mongo.NewUpdateOneModel().
				SetFilter(bson.D{{"namespace", role.Namespace}, {"name", role.Name}}).
				SetUpdate(bson.D{{"$set",
					bson.D{
						{"rules", role.Rules},
					}},
				}),
		)
		if len(mRoles) >= 50 {
			log.Infof("update %d roles", len(mRoles))
			if _, err := policymongo.NewRoleColl().BulkWrite(context.TODO(), mRoles); err != nil {
				return fmt.Errorf("udpate workflowV4s error: %s", err)
			}
			mRoles = []mongo.WriteModel{}
		}
	}
	if len(mRoles) > 0 {
		if _, err := policymongo.NewRoleColl().BulkWrite(context.TODO(), mRoles); err != nil {
			return fmt.Errorf("udpate roles stat error: %s", err)
		}
	}
	return nil
}

func migrateJiraInfo() error {
	list, err := mongodb.NewProjectManagementColl().List()
	if err == nil {
		for _, management := range list {
			if management.Type == setting.PMJira {
				log.Warnf("migrateJiraInfo: find V1170 jira info, skip migrate")
				return nil
			}
		}
	}

	type JiraOld struct {
		Host        string `bson:"host"`
		User        string `bson:"user"`
		AccessToken string `bson:"access_token"`
	}
	coll := mongotool.Database(config.MongoDatabase()).Collection("jira")
	info := &JiraOld{}
	err = coll.FindOne(context.Background(), bson.M{}).Decode(info)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Info("migrateJiraInfo: not found old jira info, done")
			return nil
		}
		return err
	}

	log.Infof("migrateJiraInfo: find info, host: %s user: %s", info.Host, info.User)
	return mongodb.NewProjectManagementColl().Create(&commonmodels.ProjectManagement{
		Type:      setting.PMJira,
		JiraHost:  info.Host,
		JiraUser:  info.User,
		JiraToken: info.AccessToken,
	})
}

func migrateSharedService() error {
	type ServiceInfo struct {
		ServiceName string
		ProductName string
		Deleted     bool
	}

	sharedServiceListInEnvMap := map[string]map[string]*ServiceInfo{}
	sharedServiceInfoList := map[string]*ServiceInfo{}

	// get all shared service in product template
	productTemplates, err := template.NewProductColl().ListWithOption(
		&template.ProductListOpt{DeployType: setting.K8SDeployType},
	)
	if err != nil {
		return errors.Wrap(err, "list product templates")
	}
	for _, productTemplate := range productTemplates {
		for _, sharedService := range productTemplate.SharedServices {
			if _, ok := sharedServiceListInEnvMap[productTemplate.ProductName]; !ok {
				sharedServiceListInEnvMap[productTemplate.ProductName] = make(map[string]*ServiceInfo)
			}
			s := &ServiceInfo{
				ServiceName: sharedService.Name,
				ProductName: sharedService.Owner,
			}
			sharedServiceListInEnvMap[productTemplate.ProductName][sharedService.Name] = s
			sharedServiceInfoList[s.ProductName+"-"+s.ServiceName] = s
		}
	}

	// get all shared service in product env
	// if shared service in product env, but not in product template, should mark it as deleted
	var allProductEnvs []*models.Product
	for _, productTemplate := range productTemplates {
		p, err := mongodb.NewProductColl().List(&mongodb.ProductListOptions{
			Name: productTemplate.ProductName,
		})
		if err != nil {
			return errors.Wrap(err, "find product env")
		}
		allProductEnvs = append(allProductEnvs, p...)
	}
	for _, project := range allProductEnvs {
		needUpdate := false
		for _, services := range project.Services {
			for _, service := range services {
				if service.ProductName != project.ProductName {
					if _, ok := sharedServiceListInEnvMap[project.ProductName]; !ok {
						sharedServiceListInEnvMap[project.ProductName] = make(map[string]*ServiceInfo)
					}
					ownerName := service.ProductName
					// set shared service in product env to private
					service.ProductName = project.ProductName
					needUpdate = true
					if _, ok := sharedServiceListInEnvMap[project.ProductName][service.ServiceName]; ok {
						continue
					}
					s := &ServiceInfo{
						ServiceName: service.ServiceName,
						ProductName: ownerName,
						Deleted:     true,
					}
					sharedServiceListInEnvMap[project.ProductName][service.ServiceName] = s
					sharedServiceInfoList[service.ProductName+"-"+service.ServiceName] = s
					log.Debugf("migrateSharedService: find service %s-%s only in product env %s", service.ProductName, service.ServiceName, project.Namespace)
				}
			}
		}
		// update shared service in product env to private
		if needUpdate {
			err := mongodb.NewProductColl().Update(project)
			if err != nil {
				return errors.Wrap(err, "update shared service in product env")
			}
			log.Infof("migrateSharedService: update shared service in product %s success", project.Namespace)
		}
	}

	// not found shared service in any env, set all service to private
	if len(sharedServiceListInEnvMap) == 0 {
		log.Infof("migrateSharedService: not found shared service in any env")
		if err := setAllServiceVisibilityToPrivate(); err != nil {
			return errors.WithMessage(err, "set all service visibility to private")
		}
		return nil
	}

	// get all shared service all revision data
	sharedServiceAllRevisionData := map[string][]*commonmodels.Service{}
	for key, service := range sharedServiceInfoList {
		result, err := mongodb.NewServiceColl().ListServiceAllRevisionsAndStatus(service.ServiceName, service.ProductName)
		if err != nil {
			log.Warnf("not found shared service %s-%s", service.ProductName, service.ServiceName)
			continue
		}
		sharedServiceAllRevisionData[key] = result
	}

	// copy shared service template all revision data to each product
	for projectName, serviceList := range sharedServiceListInEnvMap {
		for _, service := range serviceList {
			if sharedServiceAllRevision, ok := sharedServiceAllRevisionData[service.ProductName+"-"+service.ServiceName]; ok {
				var updateList []interface{}
				for _, sharedService := range sharedServiceAllRevision {
					sharedService.ProductName = projectName
					sharedService.CreateBy = setting.SystemUser
					sharedService.CreateTime = time.Now().Unix()
					sharedService.ID = [12]byte{}
					if service.Deleted {
						sharedService.Status = setting.ProductStatusDeleting
					}
					updateList = append(updateList, sharedService)
				}
				_, err = mongodb.NewServiceColl().InsertMany(context.Background(), updateList)
				if err != nil {
					return errors.Wrapf(err, "insert service %s-%s data to %s", service.ProductName, service.ServiceName, projectName)
				}
				log.Infof("migrateSharedService: copy service %s-%s data to %s", service.ProductName, service.ServiceName, projectName)
			} else {
				log.Warnf("migrateSharedService: not found shared service %s-%s revision data", service.ProductName, service.ServiceName)
			}
		}
	}

	if err := setAllServiceVisibilityToPrivate(); err != nil {
		return errors.WithMessage(err, "set all service visibility to private")
	}
	if err := clearAllProductTemplateSharedService(); err != nil {
		return errors.WithMessage(err, "clear all product template shared service")
	}

	return nil
}

func setAllServiceVisibilityToPrivate() error {
	updater := &DataBulkUpdater{
		Coll:           mongodb.NewServiceColl().Collection,
		WriteThreshold: 20,
	}
	cursor, err := mongodb.NewServiceColl().Collection.Find(context.Background(), bson.M{})
	if err != nil {
		return errors.Wrap(err, "get service cursor")
	}
	for cursor.Next(context.Background()) {
		var s models.Service
		if err := cursor.Decode(&s); err != nil {
			return errors.Wrap(err, "decode service")
		}
		err = updater.AddModel(mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": s.ID}).
			SetUpdate(bson.M{"$set": bson.M{"visibility": "private"}}).
			SetUpsert(false))
		if err != nil {
			return errors.Wrap(err, "service bulk update visibility")
		}
	}
	err = updater.Write()
	if err != nil {
		return errors.Wrap(err, "service bulk update visibility")
	}
	return nil
}

func clearAllProductTemplateSharedService() error {
	updater := &DataBulkUpdater{
		Coll:           template.NewProductColl().Collection,
		WriteThreshold: 20,
	}
	cursor, err := template.NewProductColl().Collection.Find(context.Background(), bson.M{})
	if err != nil {
		return errors.Wrap(err, "get product cursor")
	}
	for cursor.Next(context.Background()) {
		var p templatemodels.Product
		if err := cursor.Decode(&p); err != nil {
			return errors.Wrap(err, "decode product")
		}
		err = updater.AddModel(mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": p.ID}).
			SetUpdate(bson.M{"$set": bson.M{"shared_services": nil}}).
			SetUpsert(false))
		if err != nil {
			return errors.Wrap(err, "product template bulk update shared service")
		}
	}
	err = updater.Write()
	if err != nil {
		return errors.Wrap(err, "product template bulk update shared service")
	}
	return nil
}
