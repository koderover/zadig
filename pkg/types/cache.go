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

package types

import (
	corev1 "k8s.io/api/core/v1"
)

type MediumType string

const (
	ObjectMedium MediumType = "object"
	NFSMedium    MediumType = "nfs"
)

type ProvisionType string

const (
	DynamicProvision ProvisionType = "dynamic"
	StaticProvision  ProvisionType = "static"
)

// Note:
// In the current design, when the object storage is used as the cache medium, the default object storage
// will be used as the cache medium, but in the near future users may be allowed to select different object storage
// as the cache medium according to the cluster.
// So it is temporarily reserved in `v1.10.0`, but do not use.
type ObjectProperties struct {
	ID string `json:"id" bson:"id"`
}

type NFSProperties struct {
	ProvisionType    ProvisionType                     `json:"provision_type"      bson:"provision_type"         yaml:"provision_type"`
	StorageClass     string                            `json:"storage_class"       bson:"storage_class"          yaml:"storage_class"`
	StorageSizeInGiB int64                             `json:"storage_size_in_gib" bson:"storage_size_in_gib"    yaml:"storage_size_in_gib"`
	PVC              string                            `json:"pvc"                 bson:"pvc"                    yaml:"pvc"`
	AccessMode       corev1.PersistentVolumeAccessMode `json:"access_mode"         bson:"access_mode"            yaml:"access_mode"`
	Subpath          string                            `json:"sub_path"            bson:"sub_path"               yaml:"sub_path"`
	MountPath        string                            `json:"mount_path"          bson:"mount_path"             yaml:"mount_path"`
	IsTemporary      bool                              `json:"is_temporary"        bson:"is_temporary"           yaml:"is_temporary"`
}

type Cache struct {
	MediumType       MediumType       `json:"medium_type"       bson:"medium_type"`
	ObjectProperties ObjectProperties `json:"object_properties" bson:"object_properties"`
	NFSProperties    NFSProperties    `json:"nfs_properties"    bson:"nfs_properties"`
}

type CacheDirType string

const (
	WorkspaceCacheDir   CacheDirType = "workspace"
	UserDefinedCacheDir CacheDirType = "user_defined"
)

type StorageClassType string

const (
	StorageClassAll StorageClassType = "all"
)

type ScriptType string

const (
	ScriptTypeShell      ScriptType = "shell"
	ScriptTypeBatchFile  ScriptType = "batch_file"
	ScriptTypePowerShell ScriptType = "powershell"
)
