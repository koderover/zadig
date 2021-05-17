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

package serializer

import (
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
)

type decoder struct {
	typedDeserializer      runtime.Decoder
	unstructuredSerializer runtime.Serializer
}

var once sync.Once
var d *decoder

func NewDecoder() *decoder {
	once.Do(func() {
		d = &decoder{
			typedDeserializer:      serializer.NewCodecFactory(krkubeclient.Scheme()).UniversalDeserializer(),
			unstructuredSerializer: yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme),
		}
	})

	return d
}

func (d *decoder) YamlToUnstructured(manifest []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	decoded, _, err := d.unstructuredSerializer.Decode(manifest, nil, obj)

	u, ok := decoded.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object is not an Unstructured")
	}

	return u, err
}

func (d *decoder) YamlToRuntimeObject(manifest []byte) (runtime.Object, error) {
	obj, _, err := d.typedDeserializer.Decode(manifest, nil, nil)

	return obj, err
}

func (d *decoder) JsonToRuntimeObject(manifest []byte) (runtime.Object, error) {
	return d.YamlToRuntimeObject(manifest)
}

func (d *decoder) YamlToDeployment(manifest []byte) (*appsv1.Deployment, error) {
	obj, err := d.YamlToRuntimeObject(manifest)
	if err != nil {
		return nil, err
	}

	res, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("object is not a Deployment")
	}

	return res, err
}

func (d *decoder) YamlToStatefulSet(manifest []byte) (*appsv1.StatefulSet, error) {
	obj, err := d.YamlToRuntimeObject(manifest)
	if err != nil {
		return nil, err
	}

	res, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return nil, fmt.Errorf("object is not a StatefulSet")
	}

	return res, err
}

func (d *decoder) YamlToIngress(manifest []byte) (*extensionsv1beta1.Ingress, error) {
	obj, err := d.YamlToRuntimeObject(manifest)
	if err != nil {
		return nil, err
	}

	res, ok := obj.(*extensionsv1beta1.Ingress)
	if !ok {
		return nil, fmt.Errorf("object is not a Ingress")
	}

	return res, err
}

func (d *decoder) YamlToJob(manifest []byte) (*batchv1.Job, error) {
	obj, err := d.YamlToRuntimeObject(manifest)
	if err != nil {
		return nil, err
	}

	res, ok := obj.(*batchv1.Job)
	if !ok {
		return nil, fmt.Errorf("object is not a Job")
	}

	return res, err
}

func (d *decoder) YamlToCronJob(manifest []byte) (*batchv1beta1.CronJob, error) {
	obj, err := d.YamlToRuntimeObject(manifest)
	if err != nil {
		return nil, err
	}

	res, ok := obj.(*batchv1beta1.CronJob)
	if !ok {
		return nil, fmt.Errorf("object is not a CronJob")
	}

	return res, err
}

func (d *decoder) JsonToDeployment(manifest []byte) (*appsv1.Deployment, error) {
	return d.YamlToDeployment(manifest)
}

func (d *decoder) JsonToStatefulSet(manifest []byte) (*appsv1.StatefulSet, error) {
	return d.YamlToStatefulSet(manifest)
}

func (d *decoder) JsonToJob(manifest []byte) (*batchv1.Job, error) {
	return d.YamlToJob(manifest)
}

func (d *decoder) JsonToCronJob(manifest []byte) (*batchv1beta1.CronJob, error) {
	return d.YamlToCronJob(manifest)
}
