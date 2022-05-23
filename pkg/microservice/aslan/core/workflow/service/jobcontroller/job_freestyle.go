package jobcontroller

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"go.uber.org/zap"
)

type FreestyleJobCtl struct {
	job           *commonmodels.JobTask
	jobContext    *sync.Map
	globalContext *sync.Map
	logger        *zap.SugaredLogger
	ack           func()
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, globalContext *sync.Map, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	return &FreestyleJobCtl{
		job:           job,
		globalContext: globalContext,
		logger:        logger,
		ack:           ack,
	}
}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	r := rand.Intn(10)
	time.Sleep(time.Duration(r) * time.Second)
	c.logger.Infof("job: %s sleep %d seccod", c.job.Name, r)
	c.job.Status = config.StatusPassed
}
