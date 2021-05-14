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

package helmclient

import (
	"context"

	"helm.sh/helm/v3/pkg/repo"

	"github.com/koderover/zadig/lib/tool/xlog"
)

type Client interface {
	AddOrUpdateChartRepo(entry repo.Entry, isUpdate bool, log *xlog.Logger) error
	UpdateChartRepos(log *xlog.Logger) error
	InstallOrUpgradeChart(ctx context.Context, spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) error
	DeleteChartFromCache(spec *ChartSpec, chartOption *ChartOption) error
	UninstallRelease(spec *ChartSpec) error
	TemplateChart(spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) ([]byte, error)
	LintChart(spec *ChartSpec, chartOption *ChartOption, log *xlog.Logger) error
}
