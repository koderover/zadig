/*
Copyright 2026 The KodeRover Authors.

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

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	servicerepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

type deletedServiceForModuleRecovery struct {
	ServiceName  string                    `bson:"service_name"`
	ProductName  string                    `bson:"product_name"`
	Revision     int64                     `bson:"revision"`
	Type         string                    `bson:"type"`
	Yaml         string                    `bson:"yaml,omitempty"`
	VariableYaml string                    `bson:"variable_yaml,omitempty"`
	Containers   []*commonmodels.Container `bson:"containers,omitempty"`
}

func init() {
	rootCmd.AddCommand(recoverDeletedServiceModulesCmd)
}

var recoverDeletedServiceModulesCmd = &cobra.Command{
	Use:   "recover-deleted-service-modules",
	Short: "Recover auto service modules removed with deleted service templates",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return preRun()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		testRecovered, testSkipped, err := recoverDeletedServiceModules(
			ctx,
			commonrepo.NewServiceColl().Collection,
			commonrepo.NewServiceModuleColl(),
			"template_service",
			false,
		)
		if err != nil {
			return err
		}
		prodRecovered, prodSkipped, err := recoverDeletedServiceModules(
			ctx,
			commonrepo.NewProductionServiceColl().Collection,
			commonrepo.NewProductionServiceModuleColl(),
			"production_template_service",
			true,
		)
		if err != nil {
			return err
		}

		log.Infof("recovered modules for %d test + %d production deleted service revisions", testRecovered, prodRecovered)
		if testSkipped+prodSkipped > 0 {
			log.Warnf("skipped recovering modules for %d deleted service revisions; inspect the logs above", testSkipped+prodSkipped)
		}
		return nil
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := postRun(); err != nil {
			log.Errorf("failed to close mongo connection: %s", err)
		}
	},
}

func recoverDeletedServiceModules(
	ctx context.Context,
	serviceColl *mongo.Collection,
	moduleColl *commonrepo.ServiceModuleColl,
	label string,
	production bool,
) (int, int, error) {
	cursor, err := serviceColl.Find(ctx, bson.M{
		"status": setting.ProductStatusDeleting,
		"type":   setting.K8SDeployType,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list deleted services from %s: %s", label, err)
	}
	defer cursor.Close(ctx)

	recovered := 0
	skipped := 0
	for cursor.Next(ctx) {
		service := new(deletedServiceForModuleRecovery)
		if err := cursor.Decode(service); err != nil {
			log.Errorf("failed to decode deleted service from %s: %s", label, err)
			skipped++
			continue
		}

		existing, err := moduleColl.CountDocuments(ctx, bson.M{
			"project_name":   service.ProductName,
			"service_name":   service.ServiceName,
			"is_manual":      false,
			"revision_bound": service.Revision,
		})
		if err != nil {
			return recovered, skipped, fmt.Errorf("failed to check modules for %s/%s revision %d: %s", service.ProductName, service.ServiceName, service.Revision, err)
		}
		if existing > 0 {
			continue
		}

		svc := &commonmodels.Service{
			ServiceName:  service.ServiceName,
			ProductName:  service.ProductName,
			Revision:     service.Revision,
			Type:         service.Type,
			Yaml:         service.Yaml,
			VariableYaml: service.VariableYaml,
			Containers:   service.Containers,
		}
		if len(svc.Containers) == 0 {
			if strings.TrimSpace(svc.Yaml) == "" {
				log.Errorf("deleted service %s/%s revision %d has no YAML to recover modules from", service.ProductName, service.ServiceName, service.Revision)
				skipped++
				continue
			}
			// Render go-template variables before parsing, mirroring
			// ensureServiceTmpl, so services created from templates resolve the
			// same way they did when their modules were originally synced.
			rendered, err := commonutil.RenderK8sSvcYaml(svc.Yaml, svc.ProductName, svc.ServiceName, svc.VariableYaml)
			if err != nil {
				log.Errorf("failed to render deleted service %s/%s revision %d: %s", service.ProductName, service.ServiceName, service.Revision, err)
				skipped++
				continue
			}
			svc.KubeYamls = util.SplitYaml(util.ReplaceWrapLine(rendered))
			if err := commonutil.SetCurrentContainerImages(svc); err != nil {
				log.Errorf("failed to parse deleted service %s/%s revision %d: %s", service.ProductName, service.ServiceName, service.Revision, err)
				skipped++
				continue
			}
		}
		if len(svc.Containers) == 0 {
			log.Errorf("deleted service %s/%s revision %d has no containers to recover", service.ProductName, service.ServiceName, service.Revision)
			skipped++
			continue
		}

		if err := servicerepo.SyncAutoServiceModules(ctx, svc, production); err != nil {
			log.Errorf("failed to recover deleted service %s/%s revision %d: %s", service.ProductName, service.ServiceName, service.Revision, err)
			skipped++
			continue
		}
		recovered++
	}
	if err := cursor.Err(); err != nil {
		return recovered, skipped, fmt.Errorf("cursor over %s ended in error: %s", label, err)
	}
	return recovered, skipped, nil
}
