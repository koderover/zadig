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
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/template"

	"github.com/koderover/zadig/pkg/tool/log"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/crypto"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/multicluster"
)

func GetKubeAPIReader(clusterID string) (client.Reader, error) {
	return kubeclient.GetKubeAPIReader(config.HubServerAddress(), clusterID)
}

func GetRESTConfig(clusterID string) (*rest.Config, error) {
	return kubeclient.GetRESTConfig(config.HubServerAddress(), clusterID)
}

// GetClientset returns a client to interact with APIServer which implements kubernetes.Interface
func GetClientset(clusterID string) (kubernetes.Interface, error) {
	return kubeclient.GetClientset(config.HubServerAddress(), clusterID)
}

type Service struct {
	*multicluster.Agent

	coll *mongodb.K8SClusterColl
}

func NewService(hubServerAddr string) (*Service, error) {
	if hubServerAddr == "" {
		return &Service{coll: mongodb.NewK8SClusterColl()}, nil
	}

	agent, err := multicluster.NewAgent(hubServerAddr)
	if err != nil {
		return nil, err
	}

	return &Service{
		coll:  mongodb.NewK8SClusterColl(),
		Agent: agent,
	}, nil
}

func (s *Service) ListClusters(clusterType string, logger *zap.SugaredLogger) ([]*models.K8SCluster, error) {
	clusters, err := s.coll.Find(clusterType)
	if err != nil {
		logger.Errorf("failed to list clusters %v", err)
		return nil, e.ErrListK8SCluster.AddErr(err)
	}

	if len(clusters) == 0 {
		return make([]*models.K8SCluster, 0), nil
	}
	for _, cluster := range clusters {
		token, err := crypto.AesEncrypt(cluster.ID.Hex())
		if err != nil {
			return nil, err
		}
		cluster.Token = token
	}

	return clusters, nil
}

func (s *Service) CreateCluster(cluster *models.K8SCluster, id string, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	_, err := s.coll.FindByName(cluster.Name)
	if err == nil {
		logger.Errorf("failed to create cluster %s %v", cluster.Name, err)
		return nil, e.ErrCreateCluster.AddDesc(e.DuplicateClusterNameFound)
	}

	cluster.Status = setting.Pending
	if cluster.Type == setting.KubeConfigClusterType {
		// since we will always be able to connect with direct connection
		cluster.Status = setting.Normal
	}
	if id == setting.LocalClusterID {
		cluster.Status = setting.Normal
		cluster.Local = true
		cluster.AdvancedConfig = &commonmodels.AdvancedConfig{
			Strategy: "normal",
		}
		cluster.DindCfg = nil
	}
	err = s.coll.Create(cluster, id)
	if err != nil {
		return nil, e.ErrCreateCluster.AddErr(err)
	}

	if cluster.AdvancedConfig != nil {
		for _, projectName := range cluster.AdvancedConfig.ProjectNames {
			err = commonrepo.NewProjectClusterRelationColl().Create(&commonmodels.ProjectClusterRelation{
				ProjectName: projectName,
				ClusterID:   cluster.ID.Hex(),
				CreatedBy:   cluster.CreatedBy,
			})
			if err != nil {
				logger.Errorf("Failed to create projectClusterRelation err:%s", err)
			}
		}
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}
	cluster.Token = token

	if cluster.Type == setting.KubeConfigClusterType {
		err := InitializeExternalCluster(config.HubServerAddress(), cluster.ID.Hex())
		if err != nil {
			s.coll.Delete(cluster.ID.Hex())
			return nil, err
		}
		cluster.Status = setting.Normal
	}

	return cluster, nil
}

func (s *Service) UpdateCluster(id string, cluster *models.K8SCluster, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	_, err := s.coll.Get(id)

	if err != nil {
		return nil, e.ErrUpdateCluster.AddErr(e.ErrClusterNotFound.AddDesc(cluster.Name))
	}

	if existed, err := s.coll.HasDuplicateName(id, cluster.Name); existed || err != nil {
		if err != nil {
			logger.Warnf("failed to find duplicated name %v", err)
		}

		return nil, e.ErrUpdateCluster.AddDesc(e.DuplicateClusterNameFound)
	}

	err = s.coll.UpdateMutableFields(cluster, id)
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

func (s *Service) DeleteCluster(user string, id string, logger *zap.SugaredLogger) error {
	_, err := s.coll.Get(id)
	if err != nil {
		return e.ErrDeleteCluster.AddErr(e.ErrClusterNotFound.AddDesc(id))
	}

	err = RemoveClusterResources(config.HubServerAddress(), id)

	err = s.coll.Delete(id)
	if err != nil {
		logger.Errorf("failed to delete cluster by id %s %v", id, err)
		return e.ErrDeleteCluster.AddErr(err)
	}

	return nil
}

func (s *Service) GetCluster(id string, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	cluster, err := s.coll.Get(id)
	if err != nil {
		return nil, e.ErrClusterNotFound.AddErr(err)
	}

	token, err := crypto.AesEncrypt(cluster.ID.Hex())
	if err != nil {
		return nil, err
	}

	if cluster.DindCfg == nil {
		cluster.DindCfg = &commonmodels.DindCfg{
			Replicas: DefaultDindReplicas,
			Resources: &commonmodels.Resources{
				Limits: &commonmodels.Limits{
					CPU:    DefaultDindLimitsCPU,
					Memory: DefaultDindLimitsMemory,
				},
			},
			Storage: &commonmodels.DindStorage{
				Type: DefaultDindStorageType,
			},
		}
	}

	cluster.Token = token
	return cluster, nil
}

func (s *Service) GetClusterByToken(token string, logger *zap.SugaredLogger) (*models.K8SCluster, error) {
	id, err := crypto.AesDecrypt(token)
	if err != nil {
		return nil, err
	}

	return s.GetCluster(id, logger)
}

func (s *Service) ListConnectedClusters(logger *zap.SugaredLogger) ([]*models.K8SCluster, error) {
	clusters, err := s.coll.FindConnectedClusters()
	if err != nil {
		logger.Errorf("failed to list connected clusters %v", err)
		return nil, e.ErrListK8SCluster.AddErr(err)
	}

	if len(clusters) == 0 {
		return make([]*models.K8SCluster, 0), nil
	}
	for _, cluster := range clusters {
		token, err := crypto.AesEncrypt(cluster.ID.Hex())
		if err != nil {
			return nil, err
		}
		cluster.Token = token
	}

	return clusters, nil
}

func (s *Service) GetYaml(id, agentImage, rsImage, aslanURL, hubURI string, useDeployment bool, logger *zap.SugaredLogger) ([]byte, error) {
	var (
		cluster *models.K8SCluster
		err     error
	)
	if cluster, err = s.GetCluster(id, logger); err != nil {
		return nil, err
	}

	var hubBase *url.URL
	hubBase, err = url.Parse(fmt.Sprintf("%s%s", aslanURL, hubURI))
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

	dindReplicas, dindLimitsCPU, dindLimitsMemory, dindEnablePV, dindSCName, dindStorageSizeInGiB := getDindCfg(cluster)

	if cluster.Namespace == "" {
		err = YamlTemplate.Execute(buffer, TemplateSchema{
			HubAgentImage:        agentImage,
			ResourceServerImage:  rsImage,
			ClientToken:          token,
			HubServerBaseAddr:    hubBase.String(),
			AslanBaseAddr:        config2.SystemAddress(),
			UseDeployment:        useDeployment,
			DindReplicas:         dindReplicas,
			DindLimitsCPU:        dindLimitsCPU,
			DindLimitsMemory:     dindLimitsMemory,
			DindImage:            config.DindImage(),
			DindEnablePV:         dindEnablePV,
			DindStorageClassName: dindSCName,
			DindStorageSizeInGiB: dindStorageSizeInGiB,
		})
	} else {
		err = YamlTemplateForNamespace.Execute(buffer, TemplateSchema{
			HubAgentImage:        agentImage,
			ResourceServerImage:  rsImage,
			ClientToken:          token,
			HubServerBaseAddr:    hubBase.String(),
			AslanBaseAddr:        config2.SystemAddress(),
			UseDeployment:        useDeployment,
			Namespace:            cluster.Namespace,
			DindReplicas:         dindReplicas,
			DindLimitsCPU:        dindLimitsCPU,
			DindLimitsMemory:     dindLimitsMemory,
			DindImage:            config.DindImage(),
			DindEnablePV:         dindEnablePV,
			DindStorageClassName: dindSCName,
			DindStorageSizeInGiB: dindStorageSizeInGiB,
		})
	}

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func (s *Service) UpdateUpgradeAgentInfo(id, updateHubagentErrorMsg string) error {
	_, err := s.coll.Get(id)
	if err != nil {
		return err
	}
	err = s.coll.UpdateUpgradeAgentInfo(id, updateHubagentErrorMsg)
	return err
}

func getDindCfg(cluster *models.K8SCluster) (replicas int, limitsCPU, limitsMemory string, enablePV bool, scName string, storageSizeInGiB int) {
	replicas = DefaultDindReplicas
	limitsCPU = strconv.Itoa(DefaultDindLimitsCPU) + setting.CpuUintM
	limitsMemory = strconv.Itoa(DefaultDindLimitsMemory) + setting.MemoryUintMi
	enablePV = DefaultDindEnablePV
	scName = DefaultDindStorageClassName
	storageSizeInGiB = DefaultDindStorageSizeInGiB

	if cluster.DindCfg != nil {
		if cluster.DindCfg.Replicas > 0 {
			replicas = cluster.DindCfg.Replicas
		}

		if cluster.DindCfg.Resources != nil && cluster.DindCfg.Resources.Limits != nil {
			if cluster.DindCfg.Resources.Limits.CPU > 0 {
				limitsCPU = strconv.Itoa(cluster.DindCfg.Resources.Limits.CPU) + setting.CpuUintM
			}
			if cluster.DindCfg.Resources.Limits.Memory > 0 {
				limitsMemory = strconv.Itoa(cluster.DindCfg.Resources.Limits.Memory) + setting.MemoryUintMi
			}
		}

		if cluster.DindCfg.Storage != nil && cluster.DindCfg.Storage.Type == commonmodels.DindStorageDynamic {
			enablePV = true
			scName = cluster.DindCfg.Storage.StorageClass
			storageSizeInGiB = int(cluster.DindCfg.Storage.StorageSizeInGiB)
		}
	}

	return
}

// InitializeExternalCluster initialized the resources in the cluster for zadig to run correctly.
// if the cluster is of type kubeconfig, we need to create following resource:
// Namespace: koderover-agent
// Deployment: resource-server
// Service:    resource-server, dind
// StatefulSet: dind
func InitializeExternalCluster(hubserverAddr, clusterID string) error {
	clientset, err := kubeclient.GetKubeClientSet(hubserverAddr, clusterID)
	if err != nil {
		return err
	}

	// if no namespace named "koderover-agent" exists, we create one
	if _, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "koderover-agent", metav1.GetOptions{}); err != nil {
		namespaceSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "koderover-agent",
		}}

		_, err := clientset.CoreV1().Namespaces().Create(context.TODO(), namespaceSpec, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create namespace \"koderover-agent\" in the new cluster, error: %s", err)
		}
	}

	resourceServerLabelMap := map[string]string{
		"app.kubernetes.io/component": "resource-server",
		"app.kubernetes.io/name":      "zadig",
	}

	// Then we create the resource-server deployment
	resourceServerDeploymentSpec := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "resource-server",
			Labels: resourceServerLabelMap,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: resourceServerLabelMap,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: resourceServerLabelMap,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "resource-server",
							Image: config.ResourceServerImage(),
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(500) + setting.CpuUintM),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(500) + setting.MemoryUintMi),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(100) + setting.CpuUintM),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(100) + setting.MemoryUintMi),
								},
							},
							ImagePullPolicy: "Always",
						},
					},
				},
			},
		},
	}

	_, err = clientset.AppsV1().Deployments("koderover-agent").Create(context.TODO(), resourceServerDeploymentSpec, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create resource server deployment to initialize cluster, err: %s", err)
	}

	// Resource-server service
	resourceServerService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "resource-server",
			Labels: resourceServerLabelMap,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
			Selector: resourceServerLabelMap,
			Type:     "ClusterIP",
		},
	}

	_, err = clientset.CoreV1().Services("koderover-agent").Create(context.TODO(), resourceServerService, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create resource server service to initialize cluster, err: %s", err)
	}

	dindLabelMap := map[string]string{
		"app.kubernetes.io/component": "dind",
		"app.kubernetes.io/name":      "zadig",
	}

	privileged := true

	// Dind Stateful Set
	dindSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "dind",
			Labels: dindLabelMap,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "dind",
			Selector: &metav1.LabelSelector{
				MatchLabels: dindLabelMap,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: dindLabelMap,
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name:  "dind",
							Image: config.DindImage(),
							Ports: []corev1.ContainerPort{
								{
									Protocol:      "TCP",
									ContainerPort: 2375,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "DOCKER_TLS_CERTDIR",
									Value: "",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(2)),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(2) + setting.MemoryUintGi),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(strconv.Itoa(100) + setting.CpuUintM),
									corev1.ResourceMemory: resource.MustParse(strconv.Itoa(128) + setting.MemoryUintMi),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
				},
			},
		},
	}

	_, err = clientset.AppsV1().StatefulSets("koderover-agent").Create(context.TODO(), dindSts, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create dind sts to initialize cluster, err: %s", err)
	}

	// dind service
	dindService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "dind",
			Labels: dindLabelMap,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "dind",
					Protocol:   "TCP",
					Port:       2375,
					TargetPort: intstr.FromInt(2375),
				},
			},
			ClusterIP: "None",
			Selector:  dindLabelMap,
		},
	}

	_, err = clientset.CoreV1().Services("koderover-agent").Create(context.TODO(), dindService, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create dind service to initialize cluster, err: %s", err)
	}

	return nil
}

// RemoveClusterResources Removes all the resources in the koderover-agent namespace along with the namespace itself
func RemoveClusterResources(hubserverAddr, clusterID string) error {
	clientset, err := kubeclient.GetKubeClientSet(hubserverAddr, clusterID)
	if err != nil {
		return err
	}

	err = clientset.CoreV1().Services("koderover-agent").Delete(context.TODO(), "dind", metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("failed to delete dind service, err: %s", err)
	}

	err = clientset.CoreV1().Services("koderover-agent").Delete(context.TODO(), "resource-server", metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("failed to delete dind service, err: %s", err)
	}

	err = clientset.AppsV1().Deployments("koderover-agent").Delete(context.TODO(), "resource-server", metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("failed to delete resource-server deployment, err: %s", err)
	}

	err = clientset.AppsV1().StatefulSets("koderover-agent").Delete(context.TODO(), "dind", metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("failed to delete dind statefulset, err: %s", err)
	}

	err = clientset.CoreV1().Namespaces().Delete(context.TODO(), "koderover-agent", metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("failed to delete koderover-agent ns, err: %s", err)
	}

	return nil
}

type TemplateSchema struct {
	HubAgentImage        string
	ResourceServerImage  string
	ClientToken          string
	HubServerBaseAddr    string
	Namespace            string
	UseDeployment        bool
	AslanBaseAddr        string
	DindReplicas         int
	DindLimitsCPU        string
	DindLimitsMemory     string
	DindImage            string
	DindEnablePV         bool
	DindStorageClassName string
	DindStorageSizeInGiB int
}

const (
	DefaultDindReplicas         int                          = 1
	DefaultDindLimitsCPU        int                          = 4000
	DefaultDindLimitsMemory     int                          = 8192
	DefaultDindStorageType      commonmodels.DindStorageType = commonmodels.DindStorageRootfs
	DefaultDindEnablePV         bool                         = false
	DefaultDindStorageClassName string                       = ""
	DefaultDindStorageSizeInGiB int                          = 10
)

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

apiVersion: rbac.authorization.k8s.io/v1
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

apiVersion: v1
kind: Service
metadata:
  name: hub-agent
  namespace: koderover-agent
  labels:
    app: koderover-agent-agent
spec:
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app: koderover-agent-agent

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
        - name: ASLAN_BASE_ADDR
          value: "{{.AslanBaseAddr}}"
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

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: resource-server
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: resource-server
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: resource-server
        app.kubernetes.io/name: zadig
    spec:
      containers:
        - image: {{.ResourceServerImage}}
          imagePullPolicy: Always
          name: resource-server
          resources:
            limits:
              cpu: 500m
              memory: 500Mi
            requests:
              cpu: 100m
              memory: 100Mi

---

apiVersion: v1
kind: Service
metadata:
  name: resource-server
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig
spec:
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig

---

apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dind
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  serviceName: dind
  replicas: {{.DindReplicas}}
  selector:
    matchLabels:
      app.kubernetes.io/component: dind
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: dind
        app.kubernetes.io/name: zadig
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
      containers:
        - name: dind
          image: {{.DindImage}}
          env:
            - name: DOCKER_TLS_CERTDIR
              value: ""
          securityContext:
            privileged: true
          ports:
            - protocol: TCP
              containerPort: 2375
          resources:
            limits:
              cpu: {{.DindLimitsCPU}}
              memory: {{.DindLimitsMemory}}
            requests:
              cpu: 100m
              memory: 128Mi
{{- if .DindEnablePV }}
          volumeMounts:
          - name: zadig-docker
            mountPath: /var/lib/docker
  volumeClaimTemplates:
  - metadata:
      name: zadig-docker
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: {{.DindStorageClassName}}
      resources:
        requests:
          storage: {{.DindStorageSizeInGiB}}Gi
{{- end }}

---

apiVersion: v1
kind: Service
metadata:
  name: dind
  namespace: koderover-agent
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  ports:
    - name: dind
      protocol: TCP
      port: 2375
      targetPort: 2375
  clusterIP: None
  selector:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
`))

var YamlTemplateForNamespace = template.Must(template.New("agentYaml").Parse(`
---

apiVersion: v1
kind: ServiceAccount
metadata:
  name: koderover-agent-sa
  namespace: {{.Namespace}}

---

apiVersion: rbac.authorization.k8s.io/v1
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

apiVersion: v1
kind: Service
metadata:
  name: hub-agent
  namespace: {{.Namespace}}
  labels:
    app: koderover-agent-agent
spec:
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app: koderover-agent-agent

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
        - name: ASLAN_BASE_ADDR
          value: "{{.AslanBaseAddr}}"
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

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: resource-server
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: resource-server
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: resource-server
        app.kubernetes.io/name: zadig
    spec:
      containers:
        - image: {{.ResourceServerImage}}
          imagePullPolicy: Always
          name: resource-server
          resources:
            limits:
              cpu: 500m
              memory: 500Mi
            requests:
              cpu: 100m
              memory: 100Mi

---

apiVersion: v1
kind: Service
metadata:
  name: resource-server
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/instance: zadig-zadig
    app.kubernetes.io/name: zadig
spec:
  type: ClusterIP
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
  selector:
    app.kubernetes.io/component: resource-server
    app.kubernetes.io/name: zadig

---

apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: dind
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  serviceName: dind
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/component: dind
      app.kubernetes.io/name: zadig
  template:
    metadata:
      labels:
        app.kubernetes.io/component: dind
        app.kubernetes.io/name: zadig
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                topologyKey: kubernetes.io/hostname
      containers:
        - name: dind
          image: {{.DindImage}}
          env:
            - name: DOCKER_TLS_CERTDIR
              value: ""
          securityContext:
            privileged: true
          ports:
            - protocol: TCP
              containerPort: 2375
          resources:
            limits:
              cpu: "4"
              memory: 8Gi
            requests:
              cpu: 100m
              memory: 128Mi
{{- if .DindEnablePV }}
          volumeMounts:
          - name: zadig-docker
            mountPath: /var/lib/docker
  volumeClaimTemplates:
  - metadata:
      name: zadig-docker
    spec:
      accessModes: [ "ReadWriteOnce" ]
      storageClassName: {{.DindStorageClassName}}
      resources:
        requests:
          storage: {{.DindStorageSizeInGiB}}Gi
{{- end }}

---

apiVersion: v1
kind: Service
metadata:
  name: dind
  namespace: {{.Namespace}}
  labels:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
spec:
  ports:
    - name: dind
      protocol: TCP
      port: 2375
      targetPort: 2375
  clusterIP: None
  selector:
    app.kubernetes.io/component: dind
    app.kubernetes.io/name: zadig
`))
