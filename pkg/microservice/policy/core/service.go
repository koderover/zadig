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

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service/bundle"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

const (
	bundleController = iota
)

type Controller interface {
	Run(stopCh <-chan struct{})
}

func StartControllers(stopCh <-chan struct{}) {
	controllers := map[int]Controller{
		bundleController: bundle.NewBundleController(),
	}

	var wg sync.WaitGroup
	for _, c := range controllers {
		wg.Add(1)
		go func(c Controller) {
			defer wg.Done()
			c.Run(stopCh)
		}(c)
	}

	wg.Wait()
}

func Start(ctx context.Context) {
	log.Init(&log.Config{
		Level:       config.LogLevel(),
		Filename:    config.LogFile(),
		SendToFile:  config.SendLogToFile(),
		Development: config.Mode() != setting.ReleaseMode,
	})

	initDatabase(ctx)

	go StartControllers(ctx.Done())

	bundle.GenerateOPABundle()
}

func Stop(ctx context.Context) {
	_ = mongotool.Close(ctx)
}

func initDatabase(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	mongotool.Init(ctx, config.MongoURI())
	if err := mongotool.Ping(ctx); err != nil {
		panic(fmt.Errorf("failed to connect to mongo, error: %s", err))
	}

	idxCtx, idxCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer idxCancel()

	var wg sync.WaitGroup
	for _, r := range []indexer{
		mongodb.NewRoleColl(),
		mongodb.NewRoleBindingColl(),
		mongodb.NewPolicyColl(),
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

func Healthz(ctx context.Context) error {
	return mongotool.Ping(ctx)
}
