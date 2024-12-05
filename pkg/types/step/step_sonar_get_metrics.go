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

package step

import "github.com/koderover/zadig/v2/pkg/tool/sonar"

type StepSonarGetMetricsSpec struct {
	ProjectKey       string        `bson:"project_key"        json:"project_key"        yaml:"project_key"`
	Branch           string        `bson:"branch"             json:"branch"             yaml:"branch"`
	Parameter        string        `bson:"parameter"          json:"parameter"          yaml:"parameter"`
	SonarToken       string        `bson:"sonar_token"        json:"sonar_token"        yaml:"sonar_token"`
	SonarServer      string        `bson:"sonar_server"       json:"sonar_server"       yaml:"sonar_server"`
	CheckDir         string        `bson:"check_dir"          json:"check_dir"          yaml:"check_dir"`
	CheckQualityGate bool          `bson:"check_quality_gate" json:"check_quality_gate" yaml:"check_quality_gate"`
	SonarMetrics     *SonarMetrics `bson:"sonar_metrics"      json:"sonar_metrics"      yaml:"sonar_metrics"`
}

type SonarMetrics struct {
	QualityGateStatus sonar.QualityGateStatus
	Ncloc             string
	Bugs              string
	Vulnerabilities   string
	CodeSmells        string
	Coverage          string
}
