/*
Copyright 2023 The KodeRover Authors.

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
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	versionedclient "istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	"k8s.io/helm/pkg/releaseutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/kube/updater"
)

type SharedEnvHandler func(context.Context, *commonmodels.Product, string, client.Client, versionedclient.Interface) error

type ResourceApplyParam struct {
	ProductInfo         *commonmodels.Product
	ServiceName         string
	CurrentResourceYaml string
	UpdateResourceYaml  string
	Informer            informers.SharedInformerFactory
	KubeClient          client.Client
	IstioClient         versionedclient.Interface
	AddZadigLabel       bool
	InjectSecrets       bool
	SharedEnvHandler    SharedEnvHandler
}

func DeploymentSelectorLabelExists(resourceName, namespace string, informer informers.SharedInformerFactory, log *zap.SugaredLogger) bool {
	deployment, err := informer.Apps().V1().Deployments().Lister().Deployments(namespace).Get(resourceName)
	// default we assume the deployment is new so we don't need to add selector labels
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Errorf("Failed to find deployment in the namespace: %s, the error is: %s", namespace, err)
		}
		return false
	}
	// since the 2 predefined labels are always together, we just check for only one
	// if the match label exists, we return true. otherwise we return false
	if _, ok := deployment.Spec.Selector.MatchLabels["s-product"]; ok {
		return true
	}
	return false
}

func StatefulsetSelectorLabelExists(resourceName, namespace string, informer informers.SharedInformerFactory, log *zap.SugaredLogger) bool {
	sts, err := informer.Apps().V1().StatefulSets().Lister().StatefulSets(namespace).Get(resourceName)
	// default we assume the deployment is new so we don't need to add selector labels
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Errorf("Failed to find deployment in the namespace: %s, the error is: %s", namespace, err)
		}
		return false
	}
	// since the 2 predefined labels are always together, we just check for only one
	// if the match label exists, we return true. otherwise we return false
	if _, ok := sts.Spec.Selector.MatchLabels["s-product"]; ok {
		return true
	}
	return false
}

func GetPredefinedLabels(product, service string) map[string]string {
	ls := make(map[string]string)
	ls["s-product"] = product
	ls["s-service"] = service
	return ls
}

func GetPredefinedClusterLabels(product, service, envName string) map[string]string {
	labels := GetPredefinedLabels(product, service)
	labels[setting.EnvNameLabel] = envName
	return labels
}

func ApplyUpdatedAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[setting.UpdatedByLabel] = fmt.Sprintf("%d", time.Now().Unix())
	return annotations
}

func ApplySystemImagePullSecrets(podSpec *corev1.PodSpec) {
	for _, secret := range podSpec.ImagePullSecrets {
		if secret.Name == setting.DefaultImagePullSecret {
			return
		}
	}
	podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets,
		corev1.LocalObjectReference{
			Name: setting.DefaultImagePullSecret,
		})
}

func SetFieldValueIsNotExist(obj map[string]interface{}, value interface{}, fields ...string) map[string]interface{} {
	m := obj
	for _, field := range fields[:len(fields)-1] {
		if val, ok := m[field]; ok {
			if valMap, ok := val.(map[string]interface{}); ok {
				m = valMap
			} else {
				newVal := make(map[string]interface{})
				m[field] = newVal
				m = newVal
			}
		}
	}
	m[fields[len(fields)-1]] = value
	return obj
}

func removeOldResources(currentItems, items []*unstructured.Unstructured, namespace string, kubeClient client.Client, log *zap.SugaredLogger) error {
	itemsMap := make(map[string]*unstructured.Unstructured)
	errList := &multierror.Error{}
	for _, u := range items {
		itemsMap[fmt.Sprintf("%s/%s", u.GetKind(), u.GetName())] = u
	}

	oldItemsMap := make(map[string]*unstructured.Unstructured)
	for _, u := range currentItems {
		oldItemsMap[fmt.Sprintf("%s/%s", u.GetKind(), u.GetName())] = u
	}

	for name, item := range oldItemsMap {
		_, exists := itemsMap[name]
		item.SetNamespace(namespace)
		if exists {
			continue
		}
		if err := updater.DeleteUnstructured(item, kubeClient); err != nil {
			errList = multierror.Append(errList, errors.Wrapf(err, "failed to remove old item %s/%s from %s", item.GetName(), item.GetKind(), namespace))
			continue
		}
		log.Infof("succeed to remove old item %s/%s from %s", item.GetName(), item.GetKind(), namespace)
	}

	return errList.ErrorOrNil()
}

// CreateOrPatchResource create or patch resources defined in UpdateResourceYaml
// `CurrentResourceYaml` will be used to determine if some resources will be deleted
func CreateOrPatchResource(applyParam *ResourceApplyParam, log *zap.SugaredLogger) ([]*unstructured.Unstructured, error) {
	productInfo := applyParam.ProductInfo

	namespace, productName, envName := productInfo.Namespace, productInfo.ProductName, productInfo.EnvName
	informer := applyParam.Informer
	kubeClient := applyParam.KubeClient
	istioClient := applyParam.IstioClient

	errList := &multierror.Error{}
	manifests := releaseutil.SplitManifests(applyParam.UpdateResourceYaml)
	resources := make([]*unstructured.Unstructured, 0, len(manifests))
	for _, item := range manifests {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", item, err)
			errList = multierror.Append(errList, err)
			continue
		}
		resources = append(resources, u)
	}

	if errList.ErrorOrNil() != nil {
		return nil, errList.ErrorOrNil()
	}

	oldManifest := releaseutil.SplitManifests(applyParam.CurrentResourceYaml)
	curResources := make([]*unstructured.Unstructured, 0, len(oldManifest))
	for _, item := range oldManifest {
		u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
		if err != nil {
			log.Errorf("Failed to convert yaml to Unstructured, manifest is\n%s\n, error: %v", item, err)
			errList = multierror.Append(errList, err)
			continue
		}
		curResources = append(curResources, u)
	}
	if errList.ErrorOrNil() != nil {
		return nil, errList.ErrorOrNil()
	}

	err := removeOldResources(curResources, resources, namespace, applyParam.KubeClient, log)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to remove old resources")
	}

	labels := GetPredefinedLabels(productName, applyParam.ServiceName)
	clusterLabels := GetPredefinedClusterLabels(productName, applyParam.ServiceName, envName)
	if !applyParam.AddZadigLabel {
		labels = map[string]string{}
		clusterLabels = map[string]string{}
	}

	var res []*unstructured.Unstructured

	for _, u := range resources {
		switch u.GetKind() {
		case setting.Ingress:
			ls := MergeLabels(labels, u.GetLabels())

			u.SetNamespace(namespace)
			u.SetLabels(ls)

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}

		case setting.Service:
			u.SetNamespace(namespace)
			u.SetLabels(MergeLabels(labels, u.GetLabels()))

			if _, ok := u.GetLabels()["endpoints"]; !ok {
				selector, _, _ := unstructured.NestedStringMap(u.Object, "spec", "selector")
				err := unstructured.SetNestedStringMap(u.Object, MergeLabels(labels, selector), "spec", "selector")
				if err != nil {
					// should not have happened
					errList = multierror.Append(errList, errors.Wrapf(err, "failed to set nested string map for service: %v, err: %s", applyParam.ServiceName, err))
					log.Errorf("failed to set nested string map: %v", err)
					continue
				}
			}

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}

			if istioClient != nil && applyParam.SharedEnvHandler != nil {
				err = applyParam.SharedEnvHandler(context.TODO(), productInfo, u.GetName(), kubeClient, istioClient)
				if err != nil {
					log.Errorf("Failed to update Zadig service %s for env %s of product %s: %s", u.GetName(), productInfo.EnvName, productInfo.ProductName, err)
					errList = multierror.Append(errList, err)
					continue
				}
			}
		case setting.Deployment, setting.StatefulSet:
			// compatibility flag, We add a match label in spec.selector field pre 1.10.
			needSelectorLabel := false

			u.SetNamespace(namespace)
			u.SetLabels(MergeLabels(labels, u.GetLabels()))

			switch u.GetKind() {
			case setting.Deployment:
				needSelectorLabel = DeploymentSelectorLabelExists(u.GetName(), namespace, informer, log)
			case setting.StatefulSet:
				needSelectorLabel = StatefulsetSelectorLabelExists(u.GetName(), namespace, informer, log)
			}

			podLabels, _, err := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "labels")
			if err != nil {
				podLabels = nil
			}
			err = unstructured.SetNestedStringMap(u.Object, MergeLabels(labels, podLabels), "spec", "template", "metadata", "labels")
			if err != nil {
				log.Errorf("merge label failed err:%s", err)
				u.Object = SetFieldValueIsNotExist(u.Object, MergeLabels(labels, podLabels), "spec", "template", "metadata", "labels")
			}

			podAnnotations, _, err := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "annotations")
			if err != nil {
				podAnnotations = nil
			}
			err = unstructured.SetNestedStringMap(u.Object, ApplyUpdatedAnnotations(podAnnotations), "spec", "template", "metadata", "annotations")
			if err != nil {
				log.Errorf("merge annotation failed err:%s", err)
				u.Object = SetFieldValueIsNotExist(u.Object, ApplyUpdatedAnnotations(podAnnotations), "spec", "template", "metadata", "annotations")
			}

			if needSelectorLabel {
				// Inject selector: s-product and s-service
				selector, _, err := unstructured.NestedStringMap(u.Object, "spec", "selector", "matchLabels")
				if err != nil {
					selector = nil
				}

				err = unstructured.SetNestedStringMap(u.Object, MergeLabels(labels, selector), "spec", "selector", "matchLabels")
				if err != nil {
					log.Errorf("merge selector failed err:%s", err)
					u.Object = SetFieldValueIsNotExist(u.Object, MergeLabels(labels, selector), "spec", "selector", "matchLabels")
				}
			}

			jsonData, err := u.MarshalJSON()
			if err != nil {
				log.Errorf("Failed to marshal JSON, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}
			obj, err := serializer.NewDecoder().JSONToRuntimeObject(jsonData)
			if err != nil {
				log.Errorf("Failed to convert JSON to Object, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}

			switch res := obj.(type) {
			case *appsv1.Deployment:
				// Inject imagePullSecrets if qn-registry-secret is not set
				if applyParam.InjectSecrets {
					ApplySystemImagePullSecrets(&res.Spec.Template.Spec)
				}

				err = updater.CreateOrPatchDeployment(res, kubeClient)
				if err != nil {
					log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), res, err)
					errList = multierror.Append(errList, err)
					continue
				}
			case *appsv1.StatefulSet:
				// Inject imagePullSecrets if qn-registry-secret is not set
				if applyParam.InjectSecrets {
					ApplySystemImagePullSecrets(&res.Spec.Template.Spec)
				}

				err = updater.CreateOrPatchStatefulSet(res, kubeClient)
				if err != nil {
					log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), res, err)
					errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
					continue
				}
			default:
				errList = multierror.Append(errList, fmt.Errorf("object is not a appsv1.Deployment or appsv1.StatefulSet"))
				continue
			}

		case setting.Job:
			jsonData, err := u.MarshalJSON()
			if err != nil {
				log.Errorf("Failed to marshal JSON, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}
			obj, err := serializer.NewDecoder().JSONToJob(jsonData)
			if err != nil {
				log.Errorf("Failed to convert JSON to Job, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, err)
				continue
			}

			obj.Namespace = namespace
			obj.ObjectMeta.Labels = MergeLabels(labels, obj.ObjectMeta.Labels)
			obj.Spec.Template.ObjectMeta.Labels = MergeLabels(labels, obj.Spec.Template.ObjectMeta.Labels)

			// Inject imagePullSecrets if qn-registry-secret is not set
			if applyParam.InjectSecrets {
				ApplySystemImagePullSecrets(&obj.Spec.Template.Spec)
			}

			if err := updater.DeleteJobAndWait(namespace, obj.Name, kubeClient); err != nil {
				log.Errorf("Failed to delete Job, error: %v", err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}

			if err := updater.CreateJob(obj, kubeClient); err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), obj, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}

		case setting.CronJob:
			jsonData, err := u.MarshalJSON()
			if err != nil {
				log.Errorf("Failed to marshal JSON, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}
			obj, err := serializer.NewDecoder().JSONToCronJob(jsonData)
			if err != nil {
				log.Errorf("Failed to convert JSON to CronJob, manifest is\n%v\n, error: %v", u, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}

			obj.Namespace = namespace
			obj.ObjectMeta.Labels = MergeLabels(labels, obj.ObjectMeta.Labels)
			obj.Spec.JobTemplate.ObjectMeta.Labels = MergeLabels(labels, obj.Spec.JobTemplate.ObjectMeta.Labels)
			obj.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels = MergeLabels(labels, obj.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels)

			// Inject imagePullSecrets if qn-registry-secret is not set
			if applyParam.InjectSecrets {
				ApplySystemImagePullSecrets(&obj.Spec.JobTemplate.Spec.Template.Spec)
			}

			err = updater.CreateOrPatchCronJob(obj, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), obj, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}

		case setting.ClusterRole, setting.ClusterRoleBinding:
			u.SetLabels(MergeLabels(clusterLabels, u.GetLabels()))

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, errors.Wrapf(errList, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}
		default:
			u.SetNamespace(namespace)
			u.SetLabels(MergeLabels(labels, u.GetLabels()))

			err = updater.CreateOrPatchUnstructured(u, kubeClient)
			if err != nil {
				log.Errorf("Failed to create or update %s, manifest is\n%v\n, error: %v", u.GetKind(), u, err)
				errList = multierror.Append(errList, errors.Wrapf(err, "failed to create or update %s/%s", u.GetKind(), u.GetName()))
				continue
			}
		}

		res = append(res, u)
	}

	return res, errList.ErrorOrNil()
}
