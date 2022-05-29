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

package zgctl

import (
	"context"

	"github.com/gin-gonic/gin"
)

type ZgCtler interface {
	// StartDevMode starts dev mode.
	StartDevMode(ctx context.Context, projectName, envName, serviceName, dataDir, devImage string) error

	// StopDevMode stops dev mode.
	StopDevMode(ctx context.Context, projectName, envName, serviceName string) error

	// DevImages provides image list for dev mode.
	DevImages() []string

	// ConfigKubeconfig stores relations between env and kubeconfig.
	ConfigKubeconfig(projectName, envName, kubeconfigPath string) error
}

type ZgCtlHandler interface {
	// StartDevMode is a gin handler that starts dev mode.
	StartDevMode(*gin.Context)

	// StopDevMode is a gin handler that stops dev mode.
	StopDevMode(*gin.Context)

	// DevImages is a gin handler that provides image list for dev mode.
	DevImages(*gin.Context)

	// ConfigKubeconfig is a gin handler that stores relations between env and kubeconfig.
	ConfigKubeconfig(*gin.Context)
}
