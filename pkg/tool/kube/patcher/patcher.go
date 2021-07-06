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

package patcher

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	krkubeclient "github.com/koderover/zadig/pkg/tool/kube/client"
	"github.com/koderover/zadig/pkg/tool/kube/util"
)

// GeneratePatchBytes generate the patchBytes inspired by the patcher in kubectl/pkg/cmd/apply/patcher.go
func GeneratePatchBytes(obj, modifiedObj runtime.Object) ([]byte, types.PatchType, error) {
	// Serialize the current configuration of the object from the server.
	current, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, "", fmt.Errorf("serializing current configuration from:\n%v\nis failed, err: %v", obj, err)
	}

	modified, err := util.GetModifiedConfiguration(modifiedObj)
	if err != nil {
		return nil, "", fmt.Errorf("get modified configuration is failed, err: %v", err)
	}

	gvk, err := util.GetGroupVersionKind(obj)
	if err != nil {
		return nil, "", fmt.Errorf("retrieving gvk is failed, err: %v", err)
	}

	// Retrieve the original configuration of the object from the annotation.
	original, err := util.GetOriginalConfiguration(obj)
	if err != nil {
		return nil, "", fmt.Errorf("retrieving original configuration from:\n%v\nis failed, err: %v", obj, err)
	}

	var patchType types.PatchType
	var patch []byte
	var lookupPatchMeta strategicpatch.LookupPatchMeta
	createPatchErrFormat := "creating patch with:\noriginal:\n%s\nmodified:\n%s\ncurrent:\n%s\nis failed, err: %v"

	// Create the versioned struct from the type defined in the restmapping
	// (which is the API version we'll be submitting the patch to)
	versionedObject, err := krkubeclient.Scheme().New(*gvk)
	switch {
	case runtime.IsNotRegisteredError(err):
		// fall back to generic JSON merge patch
		patchType = types.MergePatchType
		preconditions := []mergepatch.PreconditionFunc{mergepatch.RequireKeyUnchanged("apiVersion"),
			mergepatch.RequireKeyUnchanged("kind"), mergepatch.RequireMetadataKeyUnchanged("name")}
		patch, err = jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current, preconditions...)
		if err != nil {
			if mergepatch.IsPreconditionFailed(err) {
				return nil, "", fmt.Errorf("%s", "At least one of apiVersion, kind and name was changed")
			}
			return nil, "", fmt.Errorf(createPatchErrFormat, original, modified, current, err)
		}
	case err != nil:
		return nil, "", fmt.Errorf("getting instance of versioned object for %v is failed, err: %v", gvk, err)
	default:
		// Compute a three way strategic merge patch to send to server.
		patchType = types.StrategicMergePatchType
		lookupPatchMeta, err = strategicpatch.NewPatchMetaFromStruct(versionedObject)
		if err != nil {
			return nil, "", fmt.Errorf(createPatchErrFormat, original, modified, current, err)
		}

		patch, err = strategicpatch.CreateThreeWayMergePatch(original, modified, current, lookupPatchMeta, true)
		if err != nil {
			return nil, "", fmt.Errorf(createPatchErrFormat, original, modified, current, err)
		}
	}

	if string(patch) == "{}" {
		return patch, "", nil
	}

	return patch, patchType, err
}
