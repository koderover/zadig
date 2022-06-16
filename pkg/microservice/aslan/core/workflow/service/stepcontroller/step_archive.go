package stepcontroller

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/types/step"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type archiveCtl struct {
	step             *commonmodels.StepTask
	toolInstalldSpec *step.StepArchiveSpec
	log              *zap.SugaredLogger
}

func NewArchiveInstallCtl(stepTask *commonmodels.StepTask, log *zap.SugaredLogger) (*archiveCtl, error) {
	yamlString, err := yaml.Marshal(stepTask.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshal tool install spec error: %v", err)
	}
	toolInstallSpec := &step.StepArchiveSpec{}
	if err := yaml.Unmarshal(yamlString, &toolInstallSpec); err != nil {
		return nil, fmt.Errorf("unmarshal git spec error: %v", err)
	}
	return &archiveCtl{toolInstalldSpec: toolInstallSpec, log: log, step: stepTask}, nil
}

func (s *archiveCtl) PreRun(ctx context.Context) {
	spec := &step.StepArchiveSpec{
		FilePath:        s.toolInstalldSpec.FilePath,
		AbsFilePath:     s.toolInstalldSpec.AbsFilePath,
		DestinationPath: s.toolInstalldSpec.DestinationPath,
		S3: &step.S3{
			Ak:        config.S3StorageAK(),
			Sk:        config.S3StorageSK(),
			Endpoint:  config.S3StorageEndpoint(),
			Bucket:    config.S3StorageBucket(),
			Subfolder: config.S3StoragePath(),
			Protocol:  config.S3StorageProtocol(),
		},
	}
	if s.toolInstalldSpec.S3StorageID == "" {
		s.toolInstalldSpec = spec
		return
	}
	objectStorage, _ := mongodb.NewS3StorageColl().Find(s.toolInstalldSpec.S3StorageID)
	if objectStorage != nil {
		spec.S3.Endpoint = objectStorage.Endpoint
		spec.S3.Sk = objectStorage.Sk
		spec.S3.Ak = objectStorage.Ak
		spec.S3.Subfolder = objectStorage.Subfolder
		spec.S3.Bucket = objectStorage.Bucket
		spec.S3.Protocol = "https"
		if objectStorage.Insecure {
			spec.S3.Protocol = "http"
		}
	}
	s.step.Spec = spec
}

func (s *archiveCtl) AfterRun(ctx context.Context) {

}
