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

type StepSonarCheckSpec struct {
	Parameter   string `bson:"parameter"       json:"parameter"         yaml:"parameter"`
	SonarToken  string `bson:"sonar_token"     json:"sonar_token"       yaml:"sonar_token"`
	SonarServer string `bson:"sonar_server"    json:"sonar_server"      yaml:"sonar_server"`
	CheckDir    string `bson:"check_dir"       json:"check_dir"         yaml:"check_dir"`
}
