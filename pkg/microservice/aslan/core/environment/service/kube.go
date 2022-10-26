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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	commonconfig "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/microservice/podexec/core/service"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/shared/kube/resource"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
	"github.com/koderover/zadig/pkg/tool/kube/util"
	"github.com/koderover/zadig/pkg/tool/log"
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

func ListPodEvents(envName, productName, podName string, log *zap.SugaredLogger) ([]*resource.Event, error) {
	res := make([]*resource.Event, 0)
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
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

// ListAvailableNamespaces lists available namespaces created by non-koderover
func ListAvailableNamespaces(clusterID, listType string, log *zap.SugaredLogger) ([]*resource.Namespace, error) {
	resp := make([]*resource.Namespace, 0)
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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
	if listType == setting.ListNamespaceTypeCreate {
		nsList, err := commonrepo.NewProductColl().ListExistedNamespace()
		if err != nil {
			log.Errorf("Failed to list existed namespace from the env List, error: %s", err)
			return nil, err
		}
		filterK8sNamespaces.Insert(nsList...)
	}

	filter := func(namespace *corev1.Namespace) bool {
		if listType == setting.ListNamespaceTypeALL {
			return true
		}
		if value, IsExist := namespace.Labels[setting.EnvCreatedBy]; IsExist {
			if value == setting.EnvCreator {
				return false
			}
		}
		if filterK8sNamespaces.Has(namespace.Name) {
			return false
		}
		return true
	}

	for _, namespace := range namespaces {
		if filter(namespace) {
			resp = append(resp, wrapper.Namespace(namespace).Resource())
		}
	}
	return resp, nil
}

func ListServicePods(productName, envName string, serviceName string, log *zap.SugaredLogger) ([]*resource.Pod, error) {
	res := make([]*resource.Pod, 0)

	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return res, e.ErrListServicePod.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
	if err != nil {
		return res, e.ErrListServicePod.AddErr(err)
	}

	selector := labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceName}.AsSelector()
	pods, err := getter.ListPods(product.Namespace, selector, kubeClient)
	if err != nil {
		errMsg := fmt.Sprintf("[%s] ListServicePods %s error: %v", product.Namespace, selector, err)
		log.Error(errMsg)
		return res, e.ErrListServicePod.AddDesc(errMsg)
	}

	for _, pod := range pods {
		res = append(res, wrapper.Pod(pod).Resource())
	}
	return res, nil
}

func DeletePod(envName, productName, podName string, log *zap.SugaredLogger) error {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return e.ErrDeletePod.AddErr(err)
	}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
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

func execPodCopy(kubeClient kubernetes.Interface, cfg *rest.Config, cmd []string, filePath, targetDir, namespace, podName, containerName string) (string, error) {
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

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
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

func DownloadFile(envName, productName, podName, container, path string, log *zap.SugaredLogger) ([]byte, string, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return nil, "", err
	}

	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), product.ClusterID)
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

	kubeCli, cfg, err := service.NewKubeOutClusterClient(product.ClusterID)
	if err != nil {
		return nil, "", e.ErrGetPodFile.AddDesc(fmt.Sprintf("get kubecli err :%v", err))
	}

	localPath, err := execPodCopy(kubeCli, cfg, []string{"tar", "cf", "-", path}, path, podFileTmpPath(envName, productName, podName, container), product.Namespace, podName, container)
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
	t, _ := util.ParseTime(as[setting.LastUpdateTimeAnnotation])
	return &serviceInfo{
		Name:           ls[setting.ServiceLabel],
		ModifiedBy:     as[setting.ModifiedByAnnotation],
		LastUpdateTime: t,
	}
}

func ListAvailableNodes(clusterID string, log *zap.SugaredLogger) (*NodeResp, error) {
	resp := new(NodeResp)
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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

func ListCustomWorkload(clusterID, namespace string, log *zap.SugaredLogger) ([]string, error) {
	resp := make([]string, 0)
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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
			resp = append(resp, strings.Join([]string{setting.Deployment, deployment.Name, container.Name}, "/"))
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
			resp = append(resp, strings.Join([]string{setting.StatefulSet, statefulset.Name, container.Name}, "/"))
		}
	}
	return resp, nil
}

// list serivce and matched deployment containers for canary and blue-green deployment.
func ListCanaryDeploymentServiceInfo(clusterID, namespace string, log *zap.SugaredLogger) ([]*ServiceMatchedDeploymentContainers, error) {
	resp := []*ServiceMatchedDeploymentContainers{}
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
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
	kubeClient, err := kubeclient.GetKubeClient(config.HubServerAddress(), clusterID)
	if err != nil {
		log.Errorf("get kubeclient error: %v, clusterID: %s", err, clusterID)
		return resp, err
	}
	discoveryCli, err := kubeclient.GetDiscoveryClient(config.HubServerAddress(), clusterID)
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
