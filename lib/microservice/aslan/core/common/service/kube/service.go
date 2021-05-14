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

package kube

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"text/template"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/crypto"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/multicluster"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func GetKubeClient(clusterId string) (client.Client, error) {
	return multicluster.GetKubeClient(config.HubServerAddress(), clusterId)
}

func GetKubeAPIReader(clusterId string) (client.Reader, error) {
	return multicluster.GetKubeAPIReader(config.HubServerAddress(), clusterId)
}

func GetRESTConfig(clusterId string) (*rest.Config, error) {
	return multicluster.GetRESTConfig(config.HubServerAddress(), clusterId)
}

// GetClientset returns a client to interact with APIServer which implements kubernetes.Interface
func GetClientset(clusterId string) (kubernetes.Interface, error) {
	return multicluster.GetClientset(config.HubServerAddress(), clusterId)
}

type Service struct {
	*multicluster.Agent

	coll *repo.K8SClusterColl
}

func NewService(hubServerAddr string) (*Service, error) {
	agent, err := multicluster.NewAgent(hubServerAddr)
	if err != nil {
		return nil, err
	}

	return &Service{
		coll:  repo.NewK8SClusterColl(),
		Agent: agent,
	}, nil
}

func (s *Service) ListClusters(clusterType string, logger *xlog.Logger) ([]*models.K8SCluster, error) {
	clusters, err := s.coll.Find(clusterType)
	if err != nil {
		logger.Errorf("failed to list clusters %v", err)
		return nil, e.ErrListK8SCluster.AddErr(err)
	}

	if len(clusters) == 0 {
		return make([]*models.K8SCluster, 0), nil
	} else {
		for _, cluster := range clusters {
			token, err := crypto.AesEncrypt(cluster.ID.Hex())
			if err != nil {
				return nil, err
			}
			cluster.Token = token
		}
	}

	return clusters, nil
}

func (s *Service) CreateCluster(cluster *models.K8SCluster, logger *xlog.Logger) (*models.K8SCluster, error) {
	_, err := s.coll.FindByName(cluster.Name)

	if err == nil {
		logger.Errorf("failed to create cluster %s %v", cluster.Name, err)
		return nil, e.ErrCreateCluster.AddDesc(e.DuplicateClusterNameFound)
	}

	cluster.Status = config.Pending

	err = s.coll.Create(cluster)

	if err != nil {
		return nil, e.ErrCreateCluster.AddErr(err)
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Service) UpdateCluster(cluster *models.K8SCluster, logger *xlog.Logger) (*models.K8SCluster, error) {
	_, err := s.coll.Get(cluster.ID.Hex())

	if err != nil {
		return nil, e.ErrUpdateCluster.AddErr(e.ErrClusterNotFound.AddDesc(cluster.Name))
	}

	if existed, err := s.coll.HasDuplicateName(cluster.ID.Hex(), cluster.Name); existed || err != nil {
		if err != nil {
			logger.Warnf("failed to find duplicated name %v", err)
		}

		return nil, e.ErrUpdateCluster.AddDesc(e.DuplicateClusterNameFound)
	}

	err = s.coll.UpdateMutableFields(cluster)
	if err != nil {
		logger.Errorf("failed to update mutable fields %v", err)
		return nil, e.ErrUpdateCluster.AddErr(err)
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Service) DeleteCluster(user string, id string, logger *xlog.Logger) error {
	_, err := s.coll.Get(id)
	if err != nil {
		return e.ErrDeleteCluster.AddErr(e.ErrClusterNotFound.AddDesc(id))
	}

	err = s.coll.Delete(id)
	if err != nil {
		logger.Errorf("failed to delete cluster by id %s %v", id, err)
		return e.ErrDeleteCluster.AddErr(err)
	}

	return nil
}

func (s *Service) GetCluster(id string, logger *xlog.Logger) (*models.K8SCluster, error) {
	cluster, err := s.coll.Get(id)
	if err != nil {
		return nil, e.ErrClusterNotFound.AddErr(err)
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token
	return cluster, nil
}

func (s *Service) GetClusterByToken(token string, logger *xlog.Logger) (*models.K8SCluster, error) {
	id, err := crypto.AesDecrypt(token)
	if err != nil {
		return nil, err
	}

	return s.GetCluster(id, logger)
}

func (s *Service) ListConnectedClusters(logger *xlog.Logger) ([]*models.K8SCluster, error) {
	clusters, err := s.coll.FindConnectedClusters()
	if err != nil {
		logger.Errorf("failed to list connected clusters %v", err)
		return nil, e.ErrListK8SCluster.AddErr(err)
	}

	if len(clusters) == 0 {
		return make([]*models.K8SCluster, 0), nil
	} else {
		for _, cluster := range clusters {
			token, err := crypto.AesEncrypt(cluster.ID.Hex())
			if err != nil {
				return nil, err
			}
			cluster.Token = token
		}
	}

	return clusters, nil
}

func (s *Service) GetYaml(id string, agentImage, aslanURL, hubUri string, useDeployment bool, logger *xlog.Logger) ([]byte, error) {
	var (
		cluster *models.K8SCluster
		err     error
	)
	if cluster, err = s.GetCluster(id, logger); err != nil {
		return nil, err
	}

	var hubBase *url.URL
	hubBase, err = url.Parse(fmt.Sprintf("%s%s", aslanURL, hubUri))
	if err != nil {
		return nil, err
	}

	if strings.ToLower(hubBase.Scheme) == "https" {
		hubBase.Scheme = "wss"
	} else {
		hubBase.Scheme = "ws"
	}

	buffer := bytes.NewBufferString("")
	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}

	if cluster.Namespace == "" {
		err = YamlTemplate.Execute(buffer, TemplateSchema{
			HubAgentImage:     agentImage,
			ClientToken:       token,
			HubServerBaseAddr: hubBase.String(),
			UseDeployment:     useDeployment,
		})
	} else {
		err = YamlTemplateForNamespace.Execute(buffer, TemplateSchema{
			HubAgentImage:     agentImage,
			ClientToken:       token,
			HubServerBaseAddr: hubBase.String(),
			UseDeployment:     useDeployment,
			Namespace:         cluster.Namespace,
		})
	}

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

type TemplateSchema struct {
	HubAgentImage     string
	ClientToken       string
	HubServerBaseAddr string
	Namespace         string
	UseDeployment     bool
}

var YamlTemplate = template.Must(template.New("agentYaml").Parse(`
---

apiVersion: v1
kind: Namespace
metadata:
  name: koderover-agent

---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: koderover-agent
  namespace: koderover-agent

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: koderover-agent-admin-binding
  namespace: koderover-agent
subjects:
- kind: ServiceAccount
  name: koderover-agent
  namespace: koderover-agent
roleRef:
  kind: ClusterRole
  name: koderover-agent-admin
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: koderover-agent-admin
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - '*'

---
apiVersion: apps/v1
{{- if .UseDeployment }}
kind: Deployment
{{- else }}
kind: DaemonSet
{{- end }}
metadata:
    name: koderover-agent-node-agent
    namespace: koderover-agent
spec:
  selector:
    matchLabels:
      app: koderover-agent-agent
  template:
    metadata:
      labels:
        app: koderover-agent-agent
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
{{- if .UseDeployment }}
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              topologyKey: kubernetes.io/hostname
{{- end }}
      hostNetwork: true
      serviceAccountName: koderover-agent
      containers:
      - name: agent
        image: {{.HubAgentImage}}
        imagePullPolicy: Always
        env:
        - name: AGENT_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: HUB_AGENT_TOKEN
          value: "{{.ClientToken}}"
        - name: HUB_SERVER_BASE_ADDR
          value: "{{.HubServerBaseAddr}}"
        resources:
          limits:
            cpu: 1000m
            memory: 1Gi
          requests:
            cpu: 100m
            memory: 256Mi 
{{- if .UseDeployment }}
  replicas: 1
{{- else }}
  updateStrategy:
    type: RollingUpdate
{{- end }}
`))

var YamlTemplateForNamespace = template.Must(template.New("agentYaml").Parse(`
---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: koderover-agent-sa
  namespace: {{.Namespace}}

---

apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: koderover-agent-admin-binding
  namespace: {{.Namespace}}
subjects:
- kind: ServiceAccount
  name: koderover-agent-sa
  namespace: {{.Namespace}}
roleRef:
  kind: Role
  name: koderover-agent-admin-role
  apiGroup: rbac.authorization.k8s.io

---

apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: koderover-agent-admin-role
  namespace: {{.Namespace}}
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'

---
apiVersion: apps/v1
{{- if .UseDeployment }}
kind: Deployment
{{- else }}
kind: DaemonSet
{{- end }}
metadata:
    name: koderover-agent-node-agent
    namespace: {{.Namespace}}
spec:
  selector:
    matchLabels:
      app: koderover-agent-agent
  template:
    metadata:
      labels:
        app: koderover-agent-agent
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                - key: beta.kubernetes.io/os
                  operator: NotIn
                  values:
                    - windows
{{- if .UseDeployment }}
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              topologyKey: kubernetes.io/hostname
{{- end }}
      hostNetwork: true
      serviceAccountName: koderover-agent-sa
      containers:
      - name: agent
        image: {{.HubAgentImage}}
        imagePullPolicy: Always
        env:
        - name: AGENT_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: HUB_AGENT_TOKEN
          value: "{{.ClientToken}}"
        - name: HUB_SERVER_BASE_ADDR
          value: "{{.HubServerBaseAddr}}"
        resources:
          limits:
            cpu: 1000m
            memory: 1Gi
          requests:
            cpu: 100m
            memory: 256Mi 
{{- if .UseDeployment }}
  replicas: 1
{{- else }}
  updateStrategy:
    type: RollingUpdate
{{- end }}
`))
