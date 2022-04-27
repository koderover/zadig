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

package bundle

// TODO: Policy Service should not care about the policy details of a certain service, instead, a service which wants
// to be managed by the Policy Service should register its rules to the Policy Service so that the Policy Service knows
// how to make a decision without being aware of the detailed rules.

type exemptionURLs struct {
	Public     rules `json:"public"`     // public urls are not controlled by AuthN and AuthZ
	Privileged rules `json:"privileged"` // privileged urls can only be visited by system admins
	Registered rules `json:"registered"` // registered urls are the entire list of urls which are controlled by AuthZ, which means that if an url is not in this list, it is not controlled by AuthZ
}

type policyRule struct {
	Methods   []string `json:"methods"`
	Endpoints []string `json:"endpoints"`
}

var publicURLs = []*policyRule{
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/health"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/webhook"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/hub/connect"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/system/registry"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/testing/report"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/cluster/agent/?*/agent.yaml"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/podexec/health"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/login", "api/v1/signup", "api/v1/retrieve", "api/v1/login-enabled"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/codehosts/?*/auth", "api/v1/codehosts/callback"},
	},
	{
		Methods:   []string{"*"},
		Endpoints: []string{"api/v1/callback"},
	},
	{
		Methods:   []string{"*"},
		Endpoints: []string{"dex/**"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"", "signin", "setup", "static/**", "v1/**", "mobile/**", "workflows/**", "favicon.ico"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/directory/codehosts/callback"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/features/?*"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/system/jenkins/exist"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/callback"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/environment/environments/?*/check/workloads/k8services"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/environment/environments/?*/check/sharenv/enable/ready"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/environment/environments/?*/check/sharenv/disable/ready"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/service/services/?*/environments/deployable"},
	},
}

// actions which are allowed for system admins only.
var systemAdminURLs = []*policyRule{
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/v1/features/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/project/products"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/cleanCache/cron"},
	},
	{
		Methods:   []string{"GET", "POST", "PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/sonar/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/picket/projects"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/cluster/clusters"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/cluster/clusters/?*"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/cluster/clusters/?*/disconnect"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/cluster/clusters/?*/reconnect"},
	},
	{
		Methods:   []string{"GET", "POST", "PUT", "DELETE"},
		Endpoints: []string{"api/collaboration/collaborations"},
	},
	{
		Methods:   []string{"POST", "PUT"},
		Endpoints: []string{"api/aslan/system/install"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/system/install/delete"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/proxyManage"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/proxyManage/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/registry/namespaces"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/registry/namespaces/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/s3storage"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/s3storage/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/githubApp"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/aslan/system/githubApp/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/jenkins/integration"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/jenkins/integration/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/jenkins/integration/user/connection"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/basicImages"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/basicImages/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/helm"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/helm/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/privateKey"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/system/privateKey/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/system/announcement"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/system/announcement/update"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/system/announcement/all"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/aslan/system/announcement/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/aslan/system/operation"},
	},
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/system/operation/?*"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/aslan/system/proxy/config"},
	},
	{
		Methods:   []string{"GET", "PUT", "PATCH"},
		Endpoints: []string{"api/v1/users/?*"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/picket/users/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/users"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/public-roles"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/v1/public-roles/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/system-roles"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/system-roles/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/public-policies"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/v1/public-policies/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/system-policies"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/system-policies/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/system-rolebindings"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/system-rolebindings/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/system-policybindings"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/system-policybindings/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/template/yaml"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/template/yaml/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/template/dockerfile"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/template/dockerfile/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/template/charts"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/template/charts/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/aslan/template/build"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/aslan/template/build/?*"},
	},
}

// actions which are allowed for project admins.
var projectAdminURLs = []*policyRule{
	{
		Methods:   []string{"PUT"},
		Endpoints: []string{"api/aslan/project/products"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/v1/picket/projects/?*"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/aslan/project/products/?*"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/users"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/users/search"},
	},
	{
		Methods:   []string{"GET", "POST", "PUT", "DELETE"},
		Endpoints: []string{"api/collaboration/collaborations"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/roles"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/roles/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/roles"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/v1/roles/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/roles/bulk-delete"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/public-roles"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/public-roles/?*"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/policies"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/policies/?*"},
	},
	{
		Methods:   []string{"POST", "DELETE"},
		Endpoints: []string{"api/v1/policies"},
	},
	{
		Methods:   []string{"PUT", "DELETE"},
		Endpoints: []string{"api/v1/policies/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/policies/bulk-delete"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/public-policies"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/public-policies/?*"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/rolebindings"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/picket/rolebindings"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/rolebindings/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/rolebindings/bulk-delete"},
	},
	{
		Methods:   []string{"GET", "POST"},
		Endpoints: []string{"api/v1/policybindings"},
	},
	{
		Methods:   []string{"GET"},
		Endpoints: []string{"api/v1/picket/policybindings"},
	},
	{
		Methods:   []string{"DELETE"},
		Endpoints: []string{"api/v1/policybindings/?*"},
	},
	{
		Methods:   []string{"POST"},
		Endpoints: []string{"api/v1/policybindings/bulk-delete"},
	},
}

var adminURLs = append(systemAdminURLs, projectAdminURLs...)
