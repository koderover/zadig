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

package service

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	commonconfig "github.com/koderover/zadig/v2/pkg/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/kube/resource"
	"github.com/koderover/zadig/v2/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	kubeutil "github.com/koderover/zadig/v2/pkg/tool/kube/util"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

type serviceInfo struct {
	Name           string    `json:"name"`
	ModifiedBy     string    `json:"modifiedBy"`
	LastUpdateTime time.Time `json:"-"`
}

type ServiceMatchedDeploymentContainers struct {
	ServiceName string `json:"service_name"`
	Deployment  struct {
		DeploymentName string   `json:"deployments_name"`
		ContainerNames []string `json:"container_names"`
	} `json:"deployment"`
}

type FetchResourceArgs struct {
	EnvName       string `form:"envName"`
	Page          int    `form:"page"`
	PageSize      int    `form:"pageSize"`
	ProjectName   string `form:"projectName"`
	ResourceTypes string `form:"-"`
	Type          string `form:"type"`
	Name          string `form:"name"`
	Production    bool   `form:"-"`
}

type WorkloadCommonData struct {
	Name           string
	WorkloadDetail *resource.Workload
	Services       []*resource.Service
	Ingresses      []*resource.Ingress
}

type WorkloadDetailResp struct {
	Name     string                    `json:"name"`
	Type     string                    `json:"type"`
	Replicas int32                     `json:"replicas"`
	Images   []resource.ContainerImage `json:"images"`
	Pods     []*resource.Pod           `json:"pods"`
	Ingress  []*resource.Ingress       `json:"ingress"`
	Services []*resource.Service       `json:"service_endpoints"`
}

type AvailableNamespace struct {
	*resource.Namespace
	Used bool `json:"used_by_other_env"`
}

func ListKubeEvents(env string, productName string, name string, rtype string, log *zap.SugaredLogger) ([]*resource.Event, error) {
	res := make([]*resource.Event, 0)
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: env,
	})
	if err != nil {
		return res, err
	}

	// cached client does not support label/field selector which has more than one kv, so we need a apiReader
	// here to read from API Server directly.
	kubeClient, err := kube.GetKubeAPIReader(product.ClusterID)
	if err != nil {
		return res, err
	}

	selector := fields.Set{"involvedObject.name": name, "involvedObject.kind": rtype}.AsSelector()
	events, err := getter.ListEvents(product.Namespace, selector, kubeClient)

	if err != nil {
		log.Errorf("failed to list kube events %s/%s/%s, err: %s", product.Namespace, rtype, name, err)
		return res, e.ErrListPodEvents.AddErr(err)
	}

	for _, evt := range events {
		res = append(res, wrapper.Event(evt).Resource())
	}

	return res, err
}

func ListPodEvents(envName, productName, podName string, production bool, log *zap.SugaredLogger) ([]*resource.Event, error) {
	res := make([]*resource.Event, 0)
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return res, err
	}

	// cached client does not support label/field selector which has more than one kv, so we need a apiReader
	// here to read from API Server directly.
	kubeClient, err := kube.GetKubeAPIReader(product.ClusterID)
	if err != nil {
		return res, err
	}

	selector := fields.Set{"involvedObject.name": podName, "involvedObject.kind": setting.Pod}.AsSelector()
	events, err := getter.ListEvents(product.Namespace, selector, kubeClient)
	if err != nil {
		log.Error(err)
		return res, err
	}

	for _, evt := range events {
		res = append(res, wrapper.Event(evt).Resource())
	}

	return res, nil
}

func FindNsUseEnvs(productInfo *commonmodels.Product, log *zap.SugaredLogger) ([]*SharedNSEnvs, error) {
	clusterID, namespace := productInfo.ClusterID, productInfo.Namespace
	resp := make([]*SharedNSEnvs, 0)
	envs, err := commonrepo.NewProductColl().ListEnvByNamespace(clusterID, namespace)
	if err != nil {
		log.Errorf("Failed to list existed namespace from the env List, error: %s", err)
		return resp, err
	}
	for _, env := range envs {
		if env.String() == productInfo.String() {
			continue
		}
		resp = append(resp, &SharedNSEnvs{
			ProjectName: env.ProductName,
			EnvName:     env.EnvName,
			Production:  env.Production,
		})
	}
	return resp, nil
}

// ListAvailableNamespaces lists available namespaces created by non-koderover
func ListAvailableNamespaces(clusterID, listType string, log *zap.SugaredLogger) ([]*AvailableNamespace, error) {
	resp := make([]*AvailableNamespace, 0)
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("ListNamespaces clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	namespaces, err := getter.ListNamespaces(kubeClient)
	if err != nil {
		log.Errorf("ListNamespaces err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}

	filterK8sNamespaces := sets.NewString("kube-node-lease", "kube-public", "kube-system")

	usedNSList, err := commonrepo.NewProductColl().ListNamespace(clusterID)
	if err != nil {
		return nil, err
	}
	usedNSSet := sets.NewString(usedNSList...)

	filter := func(namespace *corev1.Namespace) bool {
		if filterK8sNamespaces.Has(namespace.Name) {
			return false
		}
		return true
	}

	for _, namespace := range namespaces {
		if !filter(namespace) {
			continue
		}
		nsResource := &AvailableNamespace{
			Namespace: wrapper.Namespace(namespace).Resource(),
		}
		if usedNSSet.Has(namespace.Name) {
			nsResource.Used = true
		}
		resp = append(resp, nsResource)
	}
	return resp, nil
}

func DeletePod(envName, productName, podName string, production bool, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return e.ErrDeletePod.AddErr(err)
	}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(product.ClusterID)
	if err != nil {
		return e.ErrDeletePod.AddErr(err)
	}

	namespace := product.Namespace
	err = updater.DeletePod(namespace, podName, kubeClient)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] delete pod %s error: %v", namespace, podName, err)
		log.Error(errMsg)
		return e.ErrDeletePod.AddDesc(errMsg)
	}
	return nil
}

func getPrefix(file string) string {
	return strings.TrimLeft(file, "/")
}

func stripPathShortcuts(p string) string {
	newPath := path.Clean(p)
	trimmed := strings.TrimPrefix(newPath, "../")

	for trimmed != newPath {
		newPath = trimmed
		trimmed = strings.TrimPrefix(newPath, "../")
	}

	// trim leftover {".", ".."}
	if newPath == "." || newPath == ".." {
		newPath = ""
	}

	if len(newPath) > 0 && string(newPath[0]) == "/" {
		return newPath[1:]
	}

	return newPath
}

func unTarAll(reader io.Reader, destDir, prefix string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		evaledPath, err := filepath.EvalSymlinks(baseName)
		if err != nil {
			return err
		}

		if mode&os.ModeSymlink != 0 {
			linkname := header.Linkname

			if !filepath.IsAbs(linkname) {
				_ = filepath.Join(evaledPath, linkname)
			}

			if err := os.Symlink(linkname, destFileName); err != nil {
				return err
			}
		} else {
			outFile, err := os.Create(destFileName)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func execPodCopy(clusterID string, cmd []string, filePath, targetDir, namespace, podName, containerName string) (string, error) {
	kubeClient, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		return "", err
	}

	req := kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	executor, err := clientmanager.NewKubeClientManager().GetSPDYExecutor(clusterID, req.URL())
	if err != nil {
		log.Errorf("NewSPDYExecutor err: %v", err)
		return "", err
	}

	reader, outStream := io.Pipe()

	go func() {
		defer outStream.Close()
		err = executor.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
		})
		if err != nil {
			log.Errorf("steam failed: %s", err)
		}
	}()

	prefix := getPrefix(filePath)
	prefix = path.Clean(prefix)
	prefix = stripPathShortcuts(prefix)
	destPath := path.Join(targetDir, path.Base(prefix))
	err = unTarAll(reader, destPath, prefix)
	return destPath, err
}

func podFileTmpPath(envName, productName, podName, container string) string {
	return filepath.Join(commonconfig.DataPath(), "podfile", productName, envName, podName, container)
}

func DownloadFile(envName, productName, podName, container, path string, production bool, log *zap.SugaredLogger) ([]byte, string, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return nil, "", err
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(product.ClusterID)
	if err != nil {
		return nil, "", e.ErrGetPodFile.AddErr(err)
	}

	_, exist, err := getter.GetPod(product.Namespace, podName, kubeClient)
	if err != nil {
		return nil, "", e.ErrGetPodFile.AddErr(err)
	}
	if !exist {
		return nil, "", e.ErrGetPodFile.AddDesc(fmt.Sprintf("pod: %s not exits", podName))
	}

	localPath, err := execPodCopy(product.ClusterID, []string{"tar", "cf", "-", path}, path, podFileTmpPath(envName, productName, podName, container), product.Namespace, podName, container)
	if err != nil {
		return nil, "", e.ErrGetPodFile.AddErr(err)
	}

	fileBytes, err := os.ReadFile(localPath)
	return fileBytes, localPath, err
}

// getServiceFromObjectMetaList returns a set of services which are modified since last update.
// Input is all modified objects, if there are more than one objects are changed, only the last one is taken into account.
func getModifiedServiceFromObjectMetaList(oms []metav1.Object) []*serviceInfo {
	sis := make(map[string]*serviceInfo)
	for _, om := range oms {
		si := getModifiedServiceFromObjectMeta(om)
		if si.ModifiedBy == "" || si.Name == "" {
			continue
		}
		if old, ok := sis[si.Name]; ok {
			if !si.LastUpdateTime.After(old.LastUpdateTime) {
				continue
			}
		}
		sis[si.Name] = si
	}

	var res []*serviceInfo
	for _, si := range sis {
		res = append(res, si)
	}

	return res
}

func getModifiedServiceFromObjectMeta(om metav1.Object) *serviceInfo {
	ls := om.GetLabels()
	as := om.GetAnnotations()
	t, _ := kubeutil.ParseTime(as[setting.LastUpdateTimeAnnotation])
	return &serviceInfo{
		Name:           ls[setting.ServiceLabel],
		ModifiedBy:     as[setting.ModifiedByAnnotation],
		LastUpdateTime: t,
	}
}

func ListAvailableNodes(clusterID string, log *zap.SugaredLogger) (*NodeResp, error) {
	resp := new(NodeResp)
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("ListAvailableNodes clusterID:%s err:%s", clusterID, err)
		return resp, err
	}

	nodes, err := getter.ListNodes(kubeClient)
	if err != nil {
		log.Errorf("ListNodes err:%s", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}

	nodeInfos := make([]*resource.Node, 0)
	labels := sets.NewString()
	for _, node := range nodes {
		nodeResource := &resource.Node{
			Ready:  nodeReady(node),
			Labels: nodeLabel(node),
			IP:     node.Name,
		}
		nodeInfos = append(nodeInfos, nodeResource)
		labels.Insert(nodeResource.Labels...)
	}
	resp.Nodes = nodeInfos
	resp.Labels = labels.List()
	return resp, nil
}

// Ready indicates that the node is ready for traffic.
func nodeReady(node *corev1.Node) bool {
	cs := node.Status.Conditions
	for _, c := range cs {
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func nodeLabel(node *corev1.Node) []string {
	labels := make([]string, 0, len(node.Labels))
	labelM := node.Labels
	for key, value := range labelM {
		labels = append(labels, fmt.Sprintf("%s:%s", key, value))
	}
	return labels
}

func ListNamespace(clusterID string, log *zap.SugaredLogger) ([]string, error) {
	resp := make([]string, 0)
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("ListNamespaces clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	namespaces, err := getter.ListNamespaces(kubeClient)
	if err != nil {
		log.Errorf("ListNamespaces err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}
	for _, namespace := range namespaces {
		resp = append(resp, namespace.Name)
	}
	return resp, nil
}

func ListDeploymentNames(clusterID, namespace string, log *zap.SugaredLogger) ([]string, error) {
	resp := make([]string, 0)
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("ListDeployment clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	deployments, err := getter.ListDeployments(namespace, labels.Everything(), kubeClient)
	if err != nil {
		log.Errorf("ListDeployment err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}
	for _, deployment := range deployments {
		resp = append(resp, deployment.Name)
	}
	return resp, nil
}

type WorkloadInfo struct {
	WorkloadType  string `json:"workload_type"`
	WorkloadName  string `json:"workload_name"`
	ContainerName string `json:"container_name"`
}

// for now,only support deployment
func ListWorkloadsInfo(clusterID, namespace string, log *zap.SugaredLogger) ([]*WorkloadInfo, error) {
	resp := make([]*WorkloadInfo, 0)
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("ListDeployments clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	deployments, err := getter.ListDeployments(namespace, labels.Everything(), kubeClient)
	if err != nil {
		log.Errorf("ListDeployments err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}
	for _, deployment := range deployments {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			resp = append(resp, &WorkloadInfo{
				WorkloadType:  setting.Deployment,
				WorkloadName:  deployment.Name,
				ContainerName: container.Name,
			})
		}
	}
	return resp, nil
}

type WorkloadImageTarget struct {
	Target    string `json:"target"`
	ImageName string `json:"image_name"`
}

func ListCustomWorkload(clusterID, namespace string, log *zap.SugaredLogger) ([]*WorkloadImageTarget, error) {
	resp := make([]*WorkloadImageTarget, 0)
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("ListCustomWorkload clusterID:%s err:%v", clusterID, err)
		return resp, err
	}
	deployments, err := getter.ListDeployments(namespace, labels.Everything(), kubeClient)
	if err != nil {
		log.Errorf("ListDeployments err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}
	for _, deployment := range deployments {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			resp = append(resp, &WorkloadImageTarget{strings.Join([]string{setting.Deployment, deployment.Name, container.Name}, "/"), util.ExtractImageName(container.Image)})
		}
	}
	statefulsets, err := getter.ListStatefulSets(namespace, labels.Everything(), kubeClient)
	if err != nil {
		log.Errorf("ListStatefulSets err:%v", err)
		if apierrors.IsForbidden(err) {
			return resp, err
		}
		return resp, err
	}
	for _, statefulset := range statefulsets {
		for _, container := range statefulset.Spec.Template.Spec.Containers {
			resp = append(resp, &WorkloadImageTarget{strings.Join([]string{setting.StatefulSet, statefulset.Name, container.Name}, "/"), util.ExtractImageName(container.Image)})
		}
	}

	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		log.Errorf("get client set error: %v", err)
		return resp, err
	}
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("get server version error: %v", err)
		return resp, err
	}
	cronJobs, cronJobBetas, err := getter.ListCronJobs(namespace, labels.Everything(), kubeClient, VersionLessThan121(versionInfo))
	if err != nil {
		log.Errorf("list cronjobs error: %v", err)
		return resp, err
	}
	for _, cronJob := range cronJobs {
		for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			resp = append(resp, &WorkloadImageTarget{strings.Join([]string{setting.CronJob, cronJob.Name, container.Name}, "/"), util.ExtractImageName(container.Image)})
		}
	}
	for _, cronJobBeta := range cronJobBetas {
		for _, container := range cronJobBeta.Spec.JobTemplate.Spec.Template.Spec.Containers {
			resp = append(resp, &WorkloadImageTarget{strings.Join([]string{setting.CronJob, cronJobBeta.Name, container.Name}, "/"), util.ExtractImageName(container.Image)})
		}
	}
	return resp, nil
}

// list service and matched deployment containers for canary and blue-green deployment.
func ListCanaryDeploymentServiceInfo(clusterID, namespace string, log *zap.SugaredLogger) ([]*ServiceMatchedDeploymentContainers, error) {
	resp := []*ServiceMatchedDeploymentContainers{}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("get kubeclient error: %v, clusterID: %s", err, clusterID)
		return resp, err
	}
	services, err := getter.ListServices(namespace, labels.Everything(), kubeClient)
	if err != nil {
		log.Errorf("list services error: %v", err)
		return resp, err
	}
	for _, service := range services {
		if service.Spec.Selector == nil {
			continue
		}
		deploymentContainers := &ServiceMatchedDeploymentContainers{
			ServiceName: service.Name,
		}
		selector := labels.SelectorFromSet(service.Spec.Selector)
		deployments, err := getter.ListDeployments(namespace, selector, kubeClient)
		if err != nil {
			log.Errorf("ListDeployments err:%v", err)
			return resp, err
		}
		// one service should only match one deployment
		if len(deployments) != 1 {
			continue
		}
		deployment := deployments[0]
		deploymentContainers.Deployment.DeploymentName = deployment.Name
		for _, container := range deployment.Spec.Template.Spec.Containers {
			deploymentContainers.Deployment.ContainerNames = append(deploymentContainers.Deployment.ContainerNames, container.Name)
		}
		resp = append(resp, deploymentContainers)
	}
	return resp, nil
}

type K8sResource struct {
	ResourceName    string `json:"resource_name"`
	ResourceKind    string `json:"resource_kind"`
	ResourceGroup   string `json:"resource_group"`
	ResourceVersion string `json:"resource_version"`
}

func ListAllK8sResourcesInNamespace(clusterID, namespace string, log *zap.SugaredLogger) ([]*K8sResource, error) {
	resp := []*K8sResource{}
	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		log.Errorf("get kubeclient error: %v, clusterID: %s", err, clusterID)
		return resp, err
	}
	discoveryCli, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		log.Errorf("get discovery client clusterID:%s error:%v", clusterID, err)
		return resp, err
	}
	discoveryCli.ServerGroups()
	apiResources, err := discoveryCli.ServerPreferredNamespacedResources()
	if err != nil {
		log.Errorf("clusterID: %s, list api resources error:%v", clusterID, err)
		return resp, err
	}
	for _, apiGroup := range apiResources {
		for _, apiResource := range apiGroup.APIResources {
			version := ""
			group := ""
			groupVersions := strings.Split(apiGroup.GroupVersion, "/")
			if len(groupVersions) == 2 {
				group = groupVersions[0]
				version = groupVersions[1]
			} else if len(groupVersions) == 1 {
				version = groupVersions[0]
			} else {
				continue
			}
			gvk := schema.GroupVersionKind{
				Group:   group,
				Version: version,
				Kind:    apiResource.Kind,
			}
			resources, err := getter.ListUnstructuredResourceInCache(namespace, labels.Everything(), nil, gvk, kubeClient)
			if err != nil {
				log.Warnf("list resources %s %s error:%v", apiGroup.GroupVersion, apiResource.Kind, err)
				continue
			}
			for _, resource := range resources {
				resp = append(resp, &K8sResource{
					ResourceName:    resource.GetName(),
					ResourceKind:    resource.GetKind(),
					ResourceGroup:   group,
					ResourceVersion: version,
				})
			}
		}
	}
	return resp, nil
}

func ListK8sResOverview(args *FetchResourceArgs, log *zap.SugaredLogger) (*K8sResourceResp, error) {

	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       args.ProjectName,
		EnvName:    args.EnvName,
		Production: &args.Production,
	})

	if err != nil {
		return nil, e.ErrListK8sResources.AddErr(fmt.Errorf("failed to get product info, err: %s", err))
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(productInfo.ClusterID)
	if err != nil {
		return nil, e.ErrListK8sResources.AddErr(err)
	}

	inf, err := clientmanager.NewKubeClientManager().GetInformer(productInfo.ClusterID, productInfo.Namespace)
	if err != nil {
		return nil, e.ErrListGroups.AddDesc(err.Error())
	}

	page, pageSize := args.Page, args.PageSize
	clusterID, namespace := productInfo.ClusterID, productInfo.Namespace

	switch args.ResourceTypes {
	case "deployments":
		return ListDeployments(page, pageSize, namespace, kubeClient, inf)
	case "statefulsets":
		return ListStatefulSets(page, pageSize, namespace, kubeClient, inf)
	case "daemonsets":
		return ListDaemonSets(page, pageSize, namespace, kubeClient)
	case "jobs":
		return ListJobs(page, pageSize, namespace, kubeClient)
	case "cronjobs":
		return ListCronJobs(page, pageSize, productInfo.ClusterID, namespace, kubeClient, inf)
	case "services":
		return ListServices(page, pageSize, namespace, kubeClient, inf)
	case "ingresses":
		return ListIngressOverview(page, pageSize, clusterID, namespace, kubeClient, log)
	case "pvcs":
		return ListPVCs(page, pageSize, namespace, kubeClient)
	case "configmaps":
		return ListConfigMapOverview(page, pageSize, namespace, kubeClient)
	case "secrets":
		return ListK8sSecretOverview(page, pageSize, namespace, kubeClient)
	}
	return nil, e.ErrListK8sResources.AddDesc(fmt.Sprintf("unrecognized workload type: %s", args.ResourceTypes))
}

func GetK8sResourceYaml(args *FetchResourceArgs, log *zap.SugaredLogger) (string, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       args.ProjectName,
		EnvName:    args.EnvName,
		Production: &args.Production,
	})

	if err != nil {
		return "", e.ErrGetK8sResource.AddErr(fmt.Errorf("failed to get product info, err: %s", err))
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(productInfo.ClusterID)
	if err != nil {
		return "", e.ErrGetK8sResource.AddErr(fmt.Errorf("failed to init kube client with id: %s, err: %s", productInfo.ClusterID, err))
	}

	cls, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(productInfo.ClusterID)
	if err != nil {
		return "", e.ErrGetK8sResource.AddErr(fmt.Errorf("failed to init clientset with id: %s, err: %s", productInfo.ClusterID, err))
	}
	ns := productInfo.Namespace
	resName := args.Name

	switch strings.ToLower(args.Type) {
	case "deployment":
		return getK8sDeploymentYaml(ns, resName, kubeClient)
	case "statefulset":
		return getK8sStsYaml(ns, resName, kubeClient)
	case "daemonset":
		return getK8sDaemonSetYaml(ns, resName, kubeClient)
	case "job":
		return getK8sJobYaml(ns, resName, kubeClient)
	case "cronjob":
		return getK8sCronJobYaml(ns, resName, kubeClient, cls)
	case "pod":
		return getK8sPodYaml(ns, resName, kubeClient)
	case "configmap":
		return getK8sConfigMapYaml(ns, resName, kubeClient)
	case "secret":
		return getK8sSecretYaml(ns, resName, kubeClient)
	case "persistentvolumeclaim":
		return getK8sPVCYaml(ns, resName, kubeClient)
	case "service":
		return getK8sServiceYaml(ns, resName, kubeClient)
	case "ingress":
		return getK8sIngressYaml(ns, resName, kubeClient, cls)
	}
	return "", fmt.Errorf("unrecognized resource type: %s", args.Type)
}

func GetWorkloadDetail(args *FetchResourceArgs, workloadType, workloadName string, log *zap.SugaredLogger) (*WorkloadDetailResp, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       args.ProjectName,
		EnvName:    args.EnvName,
		Production: &args.Production,
	})

	if err != nil {
		return nil, e.ErrGetK8sResource.AddErr(fmt.Errorf("failed to get product info, err: %s", err))
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(productInfo.ClusterID)
	if err != nil {
		return nil, e.ErrGetK8sResource.AddErr(fmt.Errorf("failed to init kube client with id: %s, err: %s", productInfo.ClusterID, err))
	}

	cls, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(productInfo.ClusterID)
	if err != nil {
		return nil, err
	}

	workload, err := getWorkloadDetail(
		productInfo.Namespace,
		strings.ToLower(workloadType),
		workloadName,
		kubeClient, cls, log)
	if err != nil {
		err = e.ErrGetK8sResource.AddErr(fmt.Errorf("failed to get workload detail: %s/%s, err: %s", workloadType, workloadName, err))
		return nil, err
	}

	resp := &WorkloadDetailResp{
		Name:     workloadName,
		Type:     workload.WorkloadDetail.Type,
		Replicas: workload.WorkloadDetail.Replicas,
		Images:   workload.WorkloadDetail.Images,
		Pods:     workload.WorkloadDetail.Pods,
		Ingress:  workload.Ingresses,
		Services: workload.Services,
	}
	return resp, nil
}

func getWorkloadDetail(ns, resType, name string, kc client.Client, cs *kubernetes.Clientset, log *zap.SugaredLogger) (*WorkloadCommonData, error) {
	resp := &WorkloadCommonData{}
	var err error
	switch strings.ToLower(resType) {
	case "deployment":
		resp, err = GetDeployWorkloadData(ns, name, kc, cs, log)
	case "statefulset":
		resp, err = GetStsWorkloadData(ns, name, kc, cs, log)
	case "daemonset":
		resp, err = GetDsWorkloadData(ns, name, kc, cs, log)
	case "job":
		resp, err = GetJobData(ns, name, kc, cs, log)
	default:
		err = fmt.Errorf("unrecognized workload type: %s", resType)
	}
	return resp, err
}

func GetResourceDeployStatus(productName string, request *K8sDeployStatusCheckRequest, production bool, log *zap.SugaredLogger) ([]*ServiceDeployStatus, error) {
	clusterID, namespace := request.ClusterID, request.Namespace

	svcSet := sets.NewString()
	for _, svc := range request.Services {
		svcSet.Insert(svc.ServiceName)
	}
	ret := make([]*ServiceDeployStatus, 0)

	resourcesByType := make(map[string]map[string]*ResourceDeployStatus)
	addDeployStatus := func(deployStatus *ResourceDeployStatus) *ResourceDeployStatus {
		if _, ok := resourcesByType[deployStatus.Type]; !ok {
			resourcesByType[deployStatus.Type] = make(map[string]*ResourceDeployStatus)
		}
		if _, ok := resourcesByType[deployStatus.Type][deployStatus.Name]; !ok {
			resourcesByType[deployStatus.Type][deployStatus.Name] = deployStatus
		}
		return resourcesByType[deployStatus.Type][deployStatus.Name]
	}

	productServices, err := repository.ListMaxRevisionsServices(productName, production, false)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to find product services, err: %s", err))
	}

	fakeRenderMap := make(map[string]*template.ServiceRender)

	for _, sv := range request.Services {
		variableYaml, err := commontypes.RenderVariableKVToYaml(sv.VariableKVs, true)
		if err != nil {
			return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to convert render variable yaml, err: %s", err))
		}
		fakeRenderMap[sv.ServiceName] = &template.ServiceRender{
			ServiceName: sv.ServiceName,
			OverrideYaml: &template.CustomYaml{
				YamlContent:       variableYaml,
				RenderVariableKVs: sv.VariableKVs,
			},
		}
	}

	for _, svc := range productServices {

		if len(svcSet) > 0 && !svcSet.Has(svc.ServiceName) {
			continue
		}

		rederedYaml, err := kube.RenderServiceYaml(svc.Yaml, productName, svc.ServiceName, fakeRenderMap[svc.ServiceName])
		if err != nil {
			return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to render service yaml, serviceName：%s, err: %w", svc.ServiceName, err))
		}

		manifests := releaseutil.SplitManifests(rederedYaml)
		resources := make([]*ResourceDeployStatus, 0)
		for _, item := range manifests {
			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				// we should ignore the error since necessary vars may be missing when creating envs
				log.Errorf("failed to convert yaml to Unstructured when check resources, manifest is\n%s\n, error: %v", item, err)
				continue
			}
			rds := &ResourceDeployStatus{
				Type:   u.GetKind(),
				Name:   u.GetName(),
				GVK:    u.GroupVersionKind(),
				Status: StatusUnDeployed,
			}
			rds = addDeployStatus(rds)
			resources = append(resources, rds)
		}
		ret = append(ret, &ServiceDeployStatus{
			ServiceName: svc.ServiceName,
			Resources:   resources,
		})
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(err)
	}
	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		log.Errorf("failed to create kubernetes clientset for clusterID: %s, the error is: %s", clusterID, err)
		return nil, e.ErrGetResourceDeployInfo.AddErr(err)
	}

	err = setResourceDeployStatus(namespace, resourcesByType, kubeClient, clientset)
	return ret, err
}

func setResourceDeployStatus(namespace string, resourceMap map[string]map[string]*ResourceDeployStatus, kubeClient client.Client, clientset *kubernetes.Clientset) error {
	_, exist, _ := getter.GetNamespace(namespace, kubeClient)
	if !exist {
		return nil
	}

	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version, err: %s", err)
	}

	relatedGvks := make(map[schema.GroupVersionKind]schema.GroupVersionKind)
	for _, resList := range resourceMap {
		for _, res := range resList {
			gvk := kube.GetValidGVK(res.GVK, version)
			relatedGvks[gvk] = gvk
		}
	}

	for kind, gvk := range relatedGvks {
		u := &unstructured.UnstructuredList{}
		u.SetGroupVersionKind(gvk)
		err := getter.ListResourceInCache(namespace, nil, nil, u, kubeClient)
		if err != nil {
			log.Warnf("failed to get resources with gvk: %s, err: %s", gvk, err)
			continue
		}
		resources, ok := resourceMap[kind.Kind]
		if !ok {
			continue
		}
		for _, item := range u.Items {
			if deployStatus, ok := resources[item.GetName()]; ok && deployStatus.Status == StatusUnDeployed {
				deployStatus.Status = StatusDeployed
			}
		}
	}
	return nil
}

func GetReleaseDeployStatus(productName string, production bool, request *HelmDeployStatusCheckRequest) ([]*ServiceDeployStatus, error) {
	clusterID, namespace, envName := request.ClusterID, request.Namespace, request.EnvName
	_, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:              productName,
		EnvName:           envName,
		Production:        &production,
		IgnoreNotFoundErr: true,
	})
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to find product: %s/%s, err: %s", productName, envName, err))
	}
	productServices, err := repository.ListMaxRevisionsServices(productName, production, false)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to find product services, err: %s", err))
	}

	ret := make([]*ServiceDeployStatus, 0)
	svcSet := sets.NewString(request.Services...)

	releaseToServiceMap := make(map[string]*ResourceDeployStatus)
	for _, svcInfo := range productServices {
		if svcSet.Len() > 0 && !svcSet.Has(svcInfo.ServiceName) {
			continue
		}
		releaseName := util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, namespace, envName, svcInfo.ServiceName)
		deployStatus := &ResourceDeployStatus{
			Type:   "release",
			Name:   releaseName,
			Status: StatusUnDeployed,
		}
		resources := []*ResourceDeployStatus{deployStatus}
		releaseToServiceMap[releaseName] = deployStatus
		ret = append(ret, &ServiceDeployStatus{
			ServiceName: svcInfo.ServiceName,
			Resources:   resources,
		})
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(clusterID)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(err)
	}
	helmClient, err := helmtool.NewClientFromNamespace(clusterID, namespace)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(err)
	}

	err = setReleaseDeployStatus(namespace, releaseToServiceMap, kubeClient, helmClient)
	return ret, err
}

func setReleaseDeployStatus(namespace string, resourceMap map[string]*ResourceDeployStatus, kubeClient client.Client, helmClient *helmtool.HelmClient) error {
	_, exist, _ := getter.GetNamespace(namespace, kubeClient)
	if !exist {
		return nil
	}
	for releaseName, deployStatus := range resourceMap {
		release, err := helmClient.GetRelease(releaseName)
		if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
			log.Warnf("failed to get release with name: %s, err: %s", releaseName, err)
			continue
		}
		if release != nil {
			deployStatus.Status = StatusDeployed
		}
		customValues, err := helmClient.GetReleaseValues(releaseName, false)
		if err != nil {
			log.Warnf("failed to get release values with name: %s, err: %s", releaseName, err)
			continue
		}
		if len(customValues) == 0 {
			continue
		}
		overrideYaml, err := yaml.Marshal(customValues)
		if err != nil {
			log.Warnf("failed to marshal values map when fetching release deploy status, err: %s", err)
			continue
		}
		deployStatus.OverrideYaml = string(overrideYaml)
	}
	return nil
}

type GetReleaseInstanceDeployStatusResponse struct {
	ReleaseName  string `json:"release_name"`
	ChartName    string `json:"chart_name"`
	ChartVersion string `json:"chart_version"`
	Status       string `json:"status"`
	Values       string `json:"values"`
}

func GetReleaseInstanceDeployStatus(productName string, production bool, request *HelmDeployStatusCheckRequest) ([]*GetReleaseInstanceDeployStatusResponse, error) {
	clusterID, namespace, envName := request.ClusterID, request.Namespace, request.EnvName
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:              productName,
		EnvName:           envName,
		Production:        &production,
		IgnoreNotFoundErr: true,
	})
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to find product: %s/%s, err: %s", productName, envName, err))
	}
	productServiceMap, err := repository.GetMaxRevisionsServicesMap(productName, production)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(fmt.Errorf("failed to find product services, err: %s", err))
	}

	releaseToServiceMap := make(map[string]*ResourceDeployStatus)
	for _, prodSvc := range productInfo.GetServiceMap() {
		svcInfo, ok := productServiceMap[prodSvc.ServiceName]
		if !ok {
			continue
		}
		releaseName := util.GeneReleaseName(svcInfo.GetReleaseNaming(), productName, namespace, envName, prodSvc.ServiceName)
		deployStatus := &ResourceDeployStatus{
			Type:   "release",
			Name:   releaseName,
			Status: StatusUnDeployed,
		}
		releaseToServiceMap[releaseName] = deployStatus
	}

	chartSvcMap := productInfo.GetChartServiceMap()
	for _, chartSvc := range chartSvcMap {
		deployStatus := &ResourceDeployStatus{
			Type:   "release",
			Name:   chartSvc.ReleaseName,
			Status: StatusUnDeployed,
		}
		releaseToServiceMap[chartSvc.ReleaseName] = deployStatus
	}

	helmClient, err := helmtool.NewClientFromNamespace(clusterID, namespace)
	if err != nil {
		return nil, e.ErrGetResourceDeployInfo.AddErr(err)
	}

	releases, err := helmClient.ListReleasesByStateMask(action.ListDeployed | action.ListUninstalled | action.ListUninstalling | action.ListPendingInstall | action.ListPendingUpgrade | action.ListPendingRollback | action.ListSuperseded | action.ListFailed | action.ListUnknown)
	if err != nil {
		log.Warnf("failed to list releases with ns: %s, err: %s", namespace, err)
	}

	resp := make([]*GetReleaseInstanceDeployStatusResponse, 0)
	for _, release := range releases {
		if _, ok := releaseToServiceMap[release.Name]; ok {
			continue
		}

		values, err := helmClient.GetReleaseValues(release.Name, false)
		if err != nil {
			log.Warnf("failed to get release values with name: %s, err: %s", release.Name, err)
			continue
		}
		valuesYaml, err := yaml.Marshal(values)
		if err != nil {
			log.Warnf("failed to marshal values map when fetching release deploy status, err: %s", err)
			continue
		}

		resp = append(resp, &GetReleaseInstanceDeployStatusResponse{
			ReleaseName:  release.Name,
			ChartName:    release.Chart.Metadata.Name,
			ChartVersion: release.Chart.Metadata.Version,
			Status:       release.Info.Status.String(),
			Values:       string(valuesYaml),
		})
	}

	return resp, err
}

type ListPodsInfoRespone struct {
	Name       string   `json:"name"`
	Ready      string   `json:"ready"`
	Status     string   `json:"status"`
	Images     []string `json:"images"`
	CreateTime int64    `json:"create_time"`
}

func ListPodsInfo(projectName, envName string, production bool, log *zap.SugaredLogger) ([]*ListPodsInfoRespone, error) {
	res := make([]*ListPodsInfoRespone, 0)
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return nil, e.ErrListPod.AddErr(fmt.Errorf("failed to get product info, err: %s", err))
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(productInfo.ClusterID)
	if err != nil {
		return res, e.ErrListPod.AddErr(err)
	}

	pods, err := getter.ListPods(productInfo.Namespace, labels.Everything(), kubeClient)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] ListPods error: %v", productInfo.Namespace, err)
		log.Error(errMsg)
		return res, e.ErrListPod.AddDesc(errMsg)
	}

	for _, pod := range pods {
		resPod := wrapper.Pod(pod).Resource()
		images := []string{}
		readyTotal := 0
		ready := 0
		for _, c := range resPod.Containers {
			images = append(images, c.Image)
			readyTotal++
			if c.Ready {
				ready++
			}
		}
		elem := &ListPodsInfoRespone{
			Name:       resPod.Name,
			Ready:      fmt.Sprintf("%d/%d", ready, readyTotal),
			Status:     resPod.Status,
			Images:     images,
			CreateTime: resPod.CreateTime,
		}
		res = append(res, elem)
	}
	return res, nil
}

func GetPodDetailInfo(projectName, envName, podName string, production bool, log *zap.SugaredLogger) (*resource.Pod, error) {
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       projectName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		return nil, e.ErrGetPodDetail.AddErr(fmt.Errorf("failed to get product info, err: %s", err))
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(productInfo.ClusterID)
	if err != nil {
		return nil, e.ErrGetPodDetail.AddErr(err)
	}

	pod, found, err := getter.GetPod(productInfo.Namespace, podName, kubeClient)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] GetPod error: %v", productInfo.Namespace, err)
		log.Error(errMsg)
		return nil, e.ErrGetPodDetail.AddDesc(errMsg)
	}
	if !found {
		errMsg := fmt.Sprintf("[%s] can't find pod %s", productInfo.Namespace, podName)
		log.Error(errMsg)
		return nil, e.ErrGetPodDetail.AddDesc(errMsg)
	}

	res := wrapper.Pod(pod).Resource()
	return res, nil
}
