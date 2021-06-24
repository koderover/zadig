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

package executor

import (
	"time"

	"github.com/hashicorp/go-multierror"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/reaper"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
)

func Execute() error {
	log.Init(&log.Config{
		Level:       commonconfig.LogLevel(),
		NoCaller:    true,
		NoLogLevel:  true,
		Development: commonconfig.Mode() != setting.ReleaseMode,
	})

	start := time.Now()
	log.Info("build start")

	r, err := reaper.NewReaper()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		// when dog is feed, the following log has been printed
		if !r.DogFeed() {
			log.Infof("build end. duration: %.2f seconds", time.Since(start).Seconds())
		}
	}()

	var errs *multierror.Error
	if err = r.BeforeExec(); err != nil {
		log.Fatal(err)
	}
	if err = r.Exec(); err != nil {
		errs = multierror.Append(errs, err)
	}
	if err = r.AfterExec(err); err != nil {
		errs = multierror.Append(errs, err)
	}

	return errs.ErrorOrNil()
}
