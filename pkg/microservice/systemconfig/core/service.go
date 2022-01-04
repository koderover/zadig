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

package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/config"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/email/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/systemconfig/core/features/service"
	"github.com/koderover/zadig/pkg/setting"
	gormtool "github.com/koderover/zadig/pkg/tool/gorm"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

func Start(_ context.Context) {
	log.Init(&log.Config{
		Level:       configbase.LogLevel(),
		Filename:    configbase.LogFile(),
		SendToFile:  configbase.SendLogToFile(),
		Development: configbase.Mode() != setting.ReleaseMode,
	})

	initDatabase()
	flagFG, err := service.FlagToFeatureGates(config.Features())
	if err != nil {
		log.Errorf("FlagToFeatureGates err:%s", err)
	}
	dbFG, err := service.DBToFeatureGates()
	if err != nil {
		log.Errorf("DBToFeatureGates err:%s", err)
	}
	service.Features.MergeFeatureGates(flagFG, dbFG)
}

func initDatabase() {
	err := gormtool.Open(configbase.MysqlUser(),
		configbase.MysqlPassword(),
		configbase.MysqlHost(),
		config.MysqlDexDB(),
	)
	if err != nil {
		log.Panicf("Failed to open database %s", config.MysqlDexDB())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mongotool.Init(ctx, config.MongoURI())
	if err := mongotool.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to connect to mongo, error: %s", err))
	}
	idxCtx, idxCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer idxCancel()
	var wg sync.WaitGroup
	for _, r := range []indexer{
		mongodb.NewEmailHostColl(),
	} {

		wg.Add(1)
		go func(r indexer) {
			defer wg.Done()
			if err := r.EnsureIndex(idxCtx); err != nil {
				panic(fmt.Errorf("failed to create index for %s, error: %s", r.GetCollectionName(), err))
			}
		}(r)
	}
	wg.Wait()
}

type indexer interface {
	EnsureIndex(ctx context.Context) error
	GetCollectionName() string
}

func Stop(_ context.Context) {
	gormtool.Close()
}
