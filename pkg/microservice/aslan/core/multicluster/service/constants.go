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

package service

import (
	"strings"
)

// StorageProvisioner is a storage type
type StorageProvisioner string

const (
	// AliCloudNASProvisioner is the provisioner of NFS storage in AliCloud.
	AliCloudNASProvisioner StorageProvisioner = "nasplugin.csi.alibabacloud.com"

	// TencentCloudCFSProvisioner is the provisioner of NFS storage in TencentCloud.
	TencentCloudCFSProvisioner StorageProvisioner = "com.tencent.cloud.csi.cfs"

	// HuaweiCloudSFSProvisioner is the provisioner of SFS storage in HuaweiCloud.
	HuaweiCloudSFSProvisioner StorageProvisioner = "sfsturbo.csi.everest.io"

	// HuaweiCloudNASProvisioner is the provisioner of NAS storage in HuaweiCloud.
	HuaweiCloudNASProvisioner StorageProvisioner = "nas.csi.everest.io"

	// AWSEFSProvisioner is the provisioner of EFS storage in AWS.
	AWSEFSProvisioner StorageProvisioner = "efs.csi.aws.com"
)

// Note: For the convenience of users, we filter the storage types for known cloud vendors to prevent users from mistakenly selecting storage
// types that do not support the ``ReadWriteMany`` policy.
// If the user builds their own storage, it is not checked because there is no clear information to judge.
func (s StorageProvisioner) IsNFS() bool {
	if strings.Contains(string(s), "alibabacloud.com") && s != AliCloudNASProvisioner {
		return false
	}
	if strings.Contains(string(s), "com.tencent.cloud") && s != TencentCloudCFSProvisioner {
		return false
	}
	if strings.Contains(string(s), "everest.io") && !(s == HuaweiCloudSFSProvisioner || s == HuaweiCloudNASProvisioner) {
		return false
	}
	if strings.Contains(string(s), "aws.com") && s != AWSEFSProvisioner {
		return false
	}

	return true
}

// ZadigMinioSVC is the service name of minio.
const ZadigMinioSVC = "zadig-minio"

// IstiodDeployment is the Deployment name of istiod.
const IstiodDeployment = "istiod"
