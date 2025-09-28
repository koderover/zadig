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

package jobcontroller

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crClient "sigs.k8s.io/controller-runtime/pkg/client"

	zadigconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	vmmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/vm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	vmmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/workflowcontroller/stepcontroller"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/multicluster/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/dockerhost"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	DindServer              = "dind"
	KoderoverAgentNamespace = "koderover-agent"
)

type FreestyleJobCtl struct {
	job         *commonmodels.JobTask
	workflowCtx *commonmodels.WorkflowTaskCtx
	logger      *zap.SugaredLogger
	kubeclient  crClient.Client
	informer    informers.SharedInformerFactory
	apiServer   crClient.Reader
	paths       *string
	jobTaskSpec *commonmodels.JobTaskFreestyleSpec
	ack         func()
	// File handling related fields
	filesPVCNames map[string]string // mountPath -> PVCName mapping
	hasFileTypes  bool
}

func NewFreestyleJobCtl(job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, ack func(), logger *zap.SugaredLogger) *FreestyleJobCtl {
	paths := ""
	jobTaskSpec := &commonmodels.JobTaskFreestyleSpec{}
	if err := commonmodels.IToi(job.Spec, jobTaskSpec); err != nil {
		logger.Error(err)
	}
	job.Spec = jobTaskSpec
	return &FreestyleJobCtl{
		job:           job,
		workflowCtx:   workflowCtx,
		logger:        logger,
		ack:           ack,
		paths:         &paths,
		jobTaskSpec:   jobTaskSpec,
		filesPVCNames: make(map[string]string),
	}
}

func (c *FreestyleJobCtl) Clean(ctx context.Context) {}

func (c *FreestyleJobCtl) Run(ctx context.Context) {
	if err := c.prepare(ctx); err != nil {
		return
	}

	// check the job is k8s job or vm job
	if c.job.Infrastructure == setting.JobVMInfrastructure {
		var vmJobID string
		var err error
		if vmJobID, err = c.runVMJob(ctx); err != nil {
			return
		}
		c.vmJobWait(ctx, vmJobID)
		c.vmComplete(ctx, vmJobID)
	} else {
		if err := c.run(ctx); err != nil {
			return
		}
		c.wait(ctx)
		c.complete(ctx)
	}
}

func (c *FreestyleJobCtl) prepare(ctx context.Context) error {
	for _, env := range c.jobTaskSpec.Properties.Envs {
		if strings.HasPrefix(env.Value, "{{.job") && strings.HasSuffix(env.Value, "}}") {
			env.Value = ""
		}
	}
	// set default timeout
	if c.jobTaskSpec.Properties.Timeout <= 0 {
		c.jobTaskSpec.Properties.Timeout = 600
	}
	// set default resource
	if c.jobTaskSpec.Properties.ResourceRequest == setting.Request("") {
		c.jobTaskSpec.Properties.ResourceRequest = setting.MinRequest
	}
	// set default resource
	if c.jobTaskSpec.Properties.ClusterID == "" {
		c.jobTaskSpec.Properties.ClusterID = setting.LocalClusterID
	}

	// Check if there are file type environment variables
	if err := c.checkAndPrepareFileTypes(ctx); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}

	// init step configration.
	if err := stepcontroller.PrepareSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Key, c.jobTaskSpec.Steps, c.logger); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}
	c.ack()
	return nil
}

func (c *FreestyleJobCtl) run(ctx context.Context) error {
	// get kube client
	hubServerAddr := zadigconfig.HubServerServiceAddress()
	if c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		c.jobTaskSpec.Properties.Namespace = zadigconfig.Namespace()
	} else {
		c.jobTaskSpec.Properties.Namespace = setting.AttachedClusterNamespace
	}

	crClient, _, apiServer, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
	if err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}
	c.kubeclient = crClient
	c.apiServer = apiServer

	// decide which docker host to use.
	// TODO: do not use code in warpdrive moudule, should move to a public place
	dockerhosts := dockerhost.NewDockerHosts(hubServerAddr, c.logger)
	c.jobTaskSpec.Properties.DockerHost = dockerhosts.GetBestHost(dockerhost.ClusterID(c.jobTaskSpec.Properties.ClusterID), fmt.Sprintf("%v", c.workflowCtx.TaskID))

	// not local cluster
	var (
		replaceDindServer = "." + DindServer
		dockerHost        = ""
	)

	if c.jobTaskSpec.Properties.ClusterID != "" && c.jobTaskSpec.Properties.ClusterID != setting.LocalClusterID {
		if strings.Contains(c.jobTaskSpec.Properties.DockerHost, config.Namespace()) {
			// replace namespace only
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, config.Namespace(), KoderoverAgentNamespace, 1)
		} else {
			// add namespace
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+KoderoverAgentNamespace, 1)
		}
	} else if c.jobTaskSpec.Properties.ClusterID == "" || c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		if !strings.Contains(c.jobTaskSpec.Properties.DockerHost, config.Namespace()) {
			// add namespace
			dockerHost = strings.Replace(c.jobTaskSpec.Properties.DockerHost, replaceDindServer, replaceDindServer+"."+config.Namespace(), 1)
		}
	}

	c.jobTaskSpec.Properties.DockerHost = dockerHost

	jobCtxBytes, err := yaml.Marshal(BuildJobExecutorContext(c.jobTaskSpec, c.job, c.workflowCtx, c.logger))
	if err != nil {
		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}
	if err := ensureDeleteConfigMap(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		logError(c.job, err.Error(), c.logger)
		return err
	}

	if err := createJobConfigMap(
		c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, jobLabel, string(jobCtxBytes), c.kubeclient); err != nil {
		msg := fmt.Sprintf("createJobConfigMap error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	c.logger.Infof("succeed to create cm for job %s", c.job.K8sJobName)

	if len(c.jobTaskSpec.Properties.Storages) > 0 {
		for i, storage := range c.jobTaskSpec.Properties.Storages {
			if storage.ProvisionType == types.DynamicProvision {
				err = service.CreateDynamicPVC(c.jobTaskSpec.Properties.ClusterID, getStoragePVCName(c.job.K8sJobName, i), storage, c.logger)
				if err != nil {
					msg := fmt.Sprintf("create dynamic PVC error: %v", err)
					logError(c.job, msg, c.logger)
					return errors.New(msg)

				}

				c.logger.Infof("succeed to create dynamic PVC for job %s", c.job.K8sJobName)
			}
		}
	}

	jobImage := getBaseImage(c.jobTaskSpec.Properties.BuildOS, c.jobTaskSpec.Properties.ImageFrom)

	c.jobTaskSpec.Properties.Registries = getMatchedRegistries(jobImage, c.jobTaskSpec.Properties.Registries)
	//Resource request default value is LOW
	customAnnotation := make(map[string]string)
	customLabel := make(map[string]string)

	for _, lb := range c.jobTaskSpec.Properties.CustomLabels {
		customLabel[lb.Key] = lb.Value.(string)
	}
	for _, annotate := range c.jobTaskSpec.Properties.CustomAnnotations {
		customAnnotation[annotate.Key] = annotate.Value.(string)
	}

	job, err := buildJobWithFiles(c.job.JobType, jobImage, c.job.K8sJobName, c.jobTaskSpec.Properties.ClusterID, c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ResourceRequest, c.jobTaskSpec.Properties.ResReqSpec, c.job, c.jobTaskSpec, c.workflowCtx, customLabel, customAnnotation, c.filesPVCNames, c.hasFileTypes)
	if err != nil {
		msg := fmt.Sprintf("create job context error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	job.Namespace = c.jobTaskSpec.Properties.Namespace

	if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
		msg := fmt.Sprintf("delete job error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if err := createOrUpdateRegistrySecrets(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.Registries, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create secret error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	if err := updater.CreateJob(job, c.kubeclient); err != nil {
		msg := fmt.Sprintf("create job error: %v", err)
		logError(c.job, msg, c.logger)
		return errors.New(msg)
	}

	// set informer when job and cm have been created
	informer, err := clientmanager.NewKubeClientManager().GetInformer(c.jobTaskSpec.Properties.ClusterID, c.jobTaskSpec.Properties.Namespace)
	if err != nil {
		return errors.Wrap(err, "get informer")
	}
	c.informer = informer
	c.logger.Infof("succeed to create job %s", c.job.K8sJobName)
	return nil
}

func (c *FreestyleJobCtl) runVMJob(ctx context.Context) (string, error) {
	jobCtxBytes, err := yaml.Marshal(BuildJobExecutorContext(c.jobTaskSpec, c.job, c.workflowCtx, c.logger))
	if err != nil {

		msg := fmt.Sprintf("cannot Jobexcutor.Context data: %v", err)
		logError(c.job, msg, c.logger)
		return "", errors.New(msg)
	}
	jobInfo := new(commonmodels.TaskJobInfo)
	if err := commonmodels.IToi(c.job.JobInfo, jobInfo); err != nil {
		return "", fmt.Errorf("convert job info to task job info error: %v", err)
	}

	vmJob := new(vmmodels.VMJob)
	if c.workflowCtx != nil {
		vmJob.ProjectName = c.workflowCtx.ProjectName
		vmJob.WorkflowName = c.workflowCtx.WorkflowName
		vmJob.TaskID = c.workflowCtx.TaskID
		vmJob.JobName = c.job.Name
		vmJob.JobDisplayName = c.job.DisplayName
		vmJob.JobKey = c.job.Key
		vmJob.JobType = c.job.JobType
		vmJob.JobOriginName = jobInfo.JobName
	}

	vmJob.JobCtx = string(jobCtxBytes)
	vmJob.VMLabels = c.job.VMLabels
	vmJob.Status = setting.VMJobStatusCreated

	if err := vmmongodb.NewVMJobColl().Create(vmJob); err != nil {
		msg := fmt.Sprintf("create vm job error: %v", err)
		logError(c.job, msg, c.logger)
		return "", errors.New(msg)
	}
	return vmJob.ID.Hex(), nil
}

func (c *FreestyleJobCtl) checkAndPrepareFileTypes(ctx context.Context) error {
	// the file will be downloaded by agent in VM type job
	if c.job.Infrastructure == setting.JobVMInfrastructure {
		return nil
	}

	// Analyze file environment variables and group by mount paths
	mountPaths, err := c.analyzeFileMountPaths()
	if err != nil {
		return err
	}
	if len(mountPaths) == 0 {
		return nil
	}

	c.hasFileTypes = true

	// Create PVCs for each unique mount path
	if err := c.createFilesPVCs(ctx, mountPaths); err != nil {
		return fmt.Errorf("failed to create files PVCs: %v", err)
	}

	// Create helper pod and copy files
	if err := c.copyFilesToPVCs(ctx, mountPaths); err != nil {
		return fmt.Errorf("failed to copy files to PVCs: %v", err)
	}

	return nil
}

// analyzeFileMountPaths analyzes file environment variables and returns optimized mount paths
// - FilePath is where the file should be placed (target file path)
// - Mount path is the directory that should be mounted (parent directory of the file)
// - If file path is absolute, mount its parent directory
// - If file path is relative, calculate final path relative to /workspace and mount parent
// - Special handling for root directory mounting with security considerations
// Returns a map of mountPath -> list of file environment variables for that path
func (c *FreestyleJobCtl) analyzeFileMountPaths() (map[string][]*commonmodels.KeyVal, error) {
	// First, collect all file paths and determine their mount points
	allPaths := make(map[string][]*commonmodels.KeyVal)

	for _, env := range c.jobTaskSpec.Properties.Envs {
		if env.Type == commonmodels.FileType && env.FileID != "" {
			filePath := strings.TrimSpace(env.FilePath)
			if filePath == "" {
				return nil, fmt.Errorf("file env %s has empty path", env.Key)
			}

			// Security validation: prevent dangerous paths
			if strings.Contains(filePath, "..") {
				c.logger.Warnf("File path contains '..' which may be unsafe: %s", filePath)
			}

			// Validate file path doesn't contain null bytes or other dangerous characters
			if strings.ContainsAny(filePath, "\x00\n\r") {
				return nil, fmt.Errorf("file env %s contains invalid characters in path", env.Key)
			}

			var finalFilePath string
			if path.IsAbs(filePath) {
				finalFilePath = filePath
			} else {
				// relative path -> under /workspace
				finalFilePath = path.Join("/workspace", filePath)
			}

			// Clean the path to resolve '.' and '..'
			finalFilePath = path.Clean(finalFilePath)
			// Ensure absolute after clean
			if !strings.HasPrefix(finalFilePath, "/") {
				finalFilePath = "/" + finalFilePath
			}

			// For relative inputs, prevent escaping '/workspace' via '..'
			if !path.IsAbs(filePath) {
				if finalFilePath != "/workspace" && !strings.HasPrefix(finalFilePath, "/workspace/") {
					return nil, fmt.Errorf("relative path for %s escapes /workspace: %s", env.Key, finalFilePath)
				}
			}

			// Determine mount path (parent directory of the file)
			mountPath := path.Dir(finalFilePath)

			// Security check: Avoid mounting root directory if possible
			if mountPath == "/" {
				c.logger.Warnf("File %s requires mounting root directory (/), this may have security implications", env.Key)
				// For root directory files, we'll use a special mount at /zadig-root-files
				mountPath = "/zadig-root-files"
				// Store the original target path for later reference
				env.FilePath = "/zadig-root-files:" + path.Base(finalFilePath) + ":/" + path.Base(finalFilePath)
			} else {
				// Store the relative path from mount point to target file
				relativePath := path.Base(finalFilePath)
				env.FilePath = mountPath + ":" + relativePath // Use colon separator
			}

			allPaths[mountPath] = append(allPaths[mountPath], env)
		}
	}

	if len(allPaths) == 0 {
		return allPaths, nil
	}

	// Check for reasonable limits to prevent resource exhaustion
	if len(allPaths) > 50 {
		return nil, fmt.Errorf("too many unique mount paths (%d), maximum allowed is 50", len(allPaths))
	}

	// Optimize by finding parent-child relationships
	optimizedPaths := c.optimizeMountPaths(allPaths)

	// Final validation of optimized paths
	if len(optimizedPaths) > 20 {
		c.logger.Warnf("Large number of mount paths (%d) after optimization, this may impact performance", len(optimizedPaths))
	}

	c.logger.Infof("Original mount paths: %v, Optimized mount paths: %v", func() []string {
		paths := make([]string, 0, len(allPaths))
		for path := range allPaths {
			paths = append(paths, path)
		}
		return paths
	}(), func() []string {
		paths := make([]string, 0, len(optimizedPaths))
		for path := range optimizedPaths {
			paths = append(paths, path)
		}
		return paths
	}())

	return optimizedPaths, nil
}

// optimizeMountPaths optimizes mount paths by consolidating child paths under parent paths
func (c *FreestyleJobCtl) optimizeMountPaths(allPaths map[string][]*commonmodels.KeyVal) map[string][]*commonmodels.KeyVal {
	// Convert paths to a slice and sort them by length (shortest first)
	paths := make([]string, 0, len(allPaths))
	for path := range allPaths {
		paths = append(paths, path)
	}

	// Sort paths by length so we process parents before children
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) < len(paths[j])
	})

	optimizedPaths := make(map[string][]*commonmodels.KeyVal)
	pathMapping := make(map[string]string) // originalPath -> mountPath mapping

	for _, currentPath := range paths {
		parentPath := c.findParentMountPath(currentPath, optimizedPaths)

		if parentPath != "" {
			// This path is a child of an existing mount path
			optimizedPaths[parentPath] = append(optimizedPaths[parentPath], allPaths[currentPath]...)
			pathMapping[currentPath] = parentPath
			c.logger.Infof("Consolidating %s under parent mount %s", currentPath, parentPath)
		} else {
			// This path becomes a new mount point
			optimizedPaths[currentPath] = allPaths[currentPath]
			pathMapping[currentPath] = currentPath
		}
	}

	// Update the file path mapping for later use in file copying
	c.createFilePathMapping(pathMapping, optimizedPaths)

	return optimizedPaths
}

// findParentMountPath finds if there's already a parent path that can contain this path
func (c *FreestyleJobCtl) findParentMountPath(path string, existingPaths map[string][]*commonmodels.KeyVal) string {
	for existingPath := range existingPaths {
		if c.isChildPath(path, existingPath) {
			return existingPath
		}
	}
	return ""
}

// isChildPath checks if childPath is a subdirectory of parentPath
func (c *FreestyleJobCtl) isChildPath(childPath, parentPath string) bool {
	// Ensure both paths end with / for proper comparison
	if !strings.HasSuffix(parentPath, "/") {
		parentPath += "/"
	}
	if !strings.HasSuffix(childPath, "/") {
		childPath += "/"
	}

	return strings.HasPrefix(childPath, parentPath) && childPath != parentPath
}

// createFilePathMapping creates a mapping from original file paths to actual mount paths
// This is needed because files might be mounted at parent paths but need to be copied to child paths
func (c *FreestyleJobCtl) createFilePathMapping(pathMapping map[string]string, optimizedPaths map[string][]*commonmodels.KeyVal) {
	// Store the mapping for use during file copying
	for originalPath, mountPath := range pathMapping {
		if originalPath != mountPath {
			// Update the file entries to know their relative path within the mount
			for _, files := range optimizedPaths {
				for _, file := range files {
					if file.FilePath == originalPath {
						// Store the relative path from mount point to actual target
						relativePath := strings.TrimPrefix(originalPath, mountPath)
						relativePath = strings.TrimPrefix(relativePath, "/")
						if relativePath != "" {
							// We'll use this during file copying
							file.FilePath = mountPath + ":" + relativePath // Use colon as separator
						} else {
							file.FilePath = mountPath
						}
					}
				}
			}
		}
	}
}

// createFilesPVCs creates PVCs for each unique mount path
func (c *FreestyleJobCtl) createFilesPVCs(ctx context.Context, mountPathFiles map[string][]*commonmodels.KeyVal) error {
	hubServerAddr := zadigconfig.HubServerServiceAddress()
	crClient, _, _, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
	if err != nil {
		return err
	}

	var namespace string
	if c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		namespace = zadigconfig.Namespace()
	} else {
		namespace = setting.AttachedClusterNamespace
	}

	// Create a PVC for each unique mount path
	for mountPath := range mountPathFiles {
		pvcName := c.generatePVCName(mountPath)
		c.filesPVCNames[mountPath] = pvcName

		// Check if PVC already exists
		pvc := &corev1.PersistentVolumeClaim{}
		err = crClient.Get(ctx, client.ObjectKey{
			Name:      pvcName,
			Namespace: namespace,
		}, pvc)
		if err == nil {
			c.logger.Infof("PVC %s already exists for mount path %s", pvcName, mountPath)
			continue
		} else if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check PVC %s for mount path %s: %v", pvcName, mountPath, err)
		}

		// Create PVC
		filesystemVolume := corev1.PersistentVolumeFilesystem
		storageQuantity := resource.MustParse("10Gi") // Default 10GB for files

		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
				Labels: map[string]string{
					"zadig-files": "true",
					"job-name":    util.TruncateName(c.job.K8sJobName, 63),
					"mount-path":  c.sanitizeLabelValue(mountPath),
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeMode:  &filesystemVolume,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageQuantity,
					},
				},
			},
		}

		err = crClient.Create(ctx, pvc)
		if err != nil {
			return fmt.Errorf("failed to create PVC %s for mount path %s: %v", pvcName, mountPath, err)
		}

		c.logger.Infof("Created PVC %s for mount path %s", pvcName, mountPath)
	}

	return nil
}

// generatePVCName generates a unique PVC name for a mount path
func (c *FreestyleJobCtl) generatePVCName(mountPath string) string {
	// Create a safe identifier from the mount path
	pathIdentifier := strings.ReplaceAll(mountPath, "/", "-")
	// Replace underscores and any invalid characters with dashes to satisfy DNS-1123 label rules
	// Allowed: [a-z0-9]([-a-z0-9]*[a-z0-9])?
	// Normalize to lowercase first
	pathIdentifier = strings.ToLower(pathIdentifier)
	// Replace underscores with dashes
	pathIdentifier = strings.ReplaceAll(pathIdentifier, "_", "-")
	// Replace any character not in [a-z0-9-] with '-'
	clean := make([]rune, 0, len(pathIdentifier))
	for _, r := range pathIdentifier {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean = append(clean, r)
		} else {
			clean = append(clean, '-')
		}
	}
	pathIdentifier = string(clean)
	// Trim leading/trailing '-'
	pathIdentifier = strings.Trim(pathIdentifier, "-")
	if pathIdentifier == "" {
		pathIdentifier = "root"
	}

	baseName := fmt.Sprintf("zadig-files-%s-%s", c.job.K8sJobName, pathIdentifier)
	return util.TruncateName(baseName, 63)
}

// sanitizeLabelValue sanitizes a string to be a valid Kubernetes label value
func (c *FreestyleJobCtl) sanitizeLabelValue(value string) string {
	sanitized := strings.ToLower(value)
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	// Replace any character not in [a-z0-9-] with '-'
	clean := make([]rune, 0, len(sanitized))
	for _, r := range sanitized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			clean = append(clean, r)
		} else {
			clean = append(clean, '-')
		}
	}
	sanitized = string(clean)
	sanitized = strings.Trim(sanitized, "-")
	if sanitized == "" {
		sanitized = "root"
	}
	return util.TruncateName(sanitized, 63)
}

func (c *FreestyleJobCtl) copyFilesToPVCs(ctx context.Context, mountPathFiles map[string][]*commonmodels.KeyVal) error {
	hubServerAddr := zadigconfig.HubServerServiceAddress()
	crClient, _, _, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
	if err != nil {
		return err
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(c.jobTaskSpec.Properties.ClusterID)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes clientset: %v", err)
	}

	var namespace string
	if c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		namespace = zadigconfig.Namespace()
	} else {
		namespace = setting.AttachedClusterNamespace
	}

	helperPodBaseName := fmt.Sprintf("zadig-file-helper-%s", c.job.K8sJobName)
	helperPodName := util.TruncateName(helperPodBaseName, 63)
	if err := c.createHelperPodWithMultiplePVCs(ctx, crClient, namespace, helperPodName); err != nil {
		return fmt.Errorf("failed to create helper pod: %v", err)
	}

	// Ensure the helper pod is cleaned up regardless of success or failure
	defer func() {
		if err := c.cleanupHelperPod(context.Background(), crClient, namespace, helperPodName); err != nil {
			c.logger.Errorf("Failed to cleanup helper pod: %v", err)
		}
	}()

	if err := c.waitForPodReady(ctx, kubeClient, namespace, helperPodName); err != nil {
		return fmt.Errorf("failed to wait for helper pod: %v", err)
	}

	// Copy files to their respective mount paths
	for mountPath, files := range mountPathFiles {
		for _, env := range files {
			targetPath := c.parseTargetPath(env.FilePath, mountPath)

			// Check if this is a root directory file that needs special handling
			rootTargetPath := c.getRootTargetPath(env.FilePath)
			if rootTargetPath != "" && mountPath == "/zadig-root-files" {
				// Copy to the special mount first
				if err := c.copyFileToHelper(ctx, kubeClient, namespace, helperPodName, env.FileID, env.Key, targetPath); err != nil {
					c.logger.Errorf("Failed to copy file %s to temp path %s: %v", env.Key, targetPath, err)
					return fmt.Errorf("failed to copy file %s to temp path %s: %v", env.Key, targetPath, err)
				}

				// Now move from temp path to actual root path
				if err := c.moveFileInHelper(ctx, kubeClient, namespace, helperPodName, targetPath, rootTargetPath); err != nil {
					c.logger.Errorf("Failed to move file %s from %s to %s: %v", env.Key, targetPath, rootTargetPath, err)
					return fmt.Errorf("failed to move file %s from %s to %s: %v", env.Key, targetPath, rootTargetPath, err)
				}
			} else {
				// Regular file copying
				if err := c.copyFileToHelper(ctx, kubeClient, namespace, helperPodName, env.FileID, env.Key, targetPath); err != nil {
					c.logger.Errorf("Failed to copy file %s to %s: %v", env.Key, targetPath, err)
					return fmt.Errorf("failed to copy file %s to %s: %v", env.Key, targetPath, err)
				}
			}
		}
	}

	return nil
}

// parseTargetPath parses the target path from the file path, handling different formats
// Format: "mountPath:relativePath" for regular files, "mountPath:filename:targetPath" for root files
func (c *FreestyleJobCtl) parseTargetPath(filePath, mountPath string) string {
	// Handle colon-separated formats
	if strings.Contains(filePath, ":") {
		parts := strings.Split(filePath, ":")

		// Handle three-part format for root files: "mountPath:filename:targetPath"
		if len(parts) == 3 && parts[0] == mountPath && mountPath == "/zadig-root-files" {
			// Return the mount path + filename for initial copying
			return mountPath + "/" + parts[1]
		}

		// Handle two-part format: "mountPath:relativePath"
		if len(parts) == 2 && parts[0] == mountPath {
			relativePath := parts[1]
			if relativePath != "" {
				return mountPath + "/" + relativePath
			}
		}
	}

	return mountPath
}

// getRootTargetPath extracts the final target path for root directory files
func (c *FreestyleJobCtl) getRootTargetPath(filePath string) string {
	if strings.Contains(filePath, ":") {
		parts := strings.Split(filePath, ":")
		// Handle three-part format for root files: "mountPath:filename:targetPath"
		if len(parts) == 3 && parts[0] == "/zadig-root-files" {
			return parts[2] // Return the final target path
		}
	}
	return ""
}

// createHelperPodWithMultiplePVCs creates a helper pod that mounts all required PVCs
func (c *FreestyleJobCtl) createHelperPodWithMultiplePVCs(ctx context.Context, client crClient.Client, namespace, podName string) error {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	// Create volumes and volume mounts for each PVC
	for mountPath, pvcName := range c.filesPVCNames {
		volumeName := c.generateVolumeName(mountPath)

		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"zadig-file-helper": "true",
				"job-name":          util.TruncateName(c.job.K8sJobName, 63),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:         "file-helper",
					Image:        "busybox:latest",
					Command:      []string{"sleep", "3600"}, // Sleep for 1 hour
					VolumeMounts: volumeMounts,
				},
			},
			Volumes: volumes,
		},
	}

	err := client.Create(ctx, pod)
	if err != nil {
		return fmt.Errorf("failed to create helper pod: %v", err)
	}

	c.logger.Infof("Created helper pod %s with %d volume mounts in namespace %s", podName, len(volumeMounts), namespace)
	return nil
}

// generateVolumeName generates a safe volume name from mount path
func (c *FreestyleJobCtl) generateVolumeName(mountPath string) string {
	volumeName := strings.ReplaceAll(mountPath, "/", "-")
	volumeName = strings.Trim(volumeName, "-")
	if volumeName == "" {
		volumeName = "root"
	}
	return "files-" + volumeName
}

func (c *FreestyleJobCtl) waitForPodReady(ctx context.Context, client *kubernetes.Clientset, namespace, podName string) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod %s to be ready", podName)
		case <-ticker.C:
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				c.logger.Warnf("Failed to get pod %s: %v", podName, err)
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				c.logger.Infof("Helper pod %s is ready", podName)
				return nil
			}
			c.logger.Infof("Waiting for pod %s to be ready, current phase: %s", podName, pod.Status.Phase)
		}
	}
}

func (c *FreestyleJobCtl) copyFileToHelper(ctx context.Context, client *kubernetes.Clientset, namespace, podName, fileID, envKey, mountPath string) error {
	temporaryFile, err := mongodb.NewTemporaryFileColl().GetByID(fileID)
	if err != nil {
		return fmt.Errorf("failed to get temporary file: %v", err)
	}
	if temporaryFile == nil {
		return fmt.Errorf("temporary file not found")
	}
	if temporaryFile.Status != commonmodels.TemporaryFileStatusCompleted {
		return fmt.Errorf("file not ready, status: %s", temporaryFile.Status)
	}

	s3Storage, err := mongodb.NewS3StorageColl().Find(temporaryFile.StorageID)
	if err != nil {
		return fmt.Errorf("failed to get default s3 storage: %v", err)
	}

	s3Client, err := s3tool.NewClient(s3Storage.Endpoint, s3Storage.Ak, s3Storage.Sk, s3Storage.Region, s3Storage.Insecure, s3Storage.Provider)
	if err != nil {
		return fmt.Errorf("failed to create s3 client: %v", err)
	}

	localTempFile := fmt.Sprintf("/tmp/%s_%s", envKey, temporaryFile.FileName)
	if err := s3Client.Download(s3Storage.Bucket, temporaryFile.FilePath, localTempFile); err != nil {
		return fmt.Errorf("failed to download file from s3: %v", err)
	}
	defer func() {
		if err := os.Remove(localTempFile); err != nil {
			c.logger.Warnf("Failed to cleanup temp file %s: %v", localTempFile, err)
		}
	}()

	retryCount := 3
	for i := 0; i < retryCount; i++ {
		if err := c.copyFileToCluster(ctx, client, namespace, podName, localTempFile, mountPath); err != nil {
			c.logger.Warnf("Failed to copy file (attempt %d/%d): %v", i+1, retryCount, err)
			if i < retryCount-1 {
				// Exponential backoff
				time.Sleep(time.Duration(i+1) * 5 * time.Second)
				continue
			}
			return fmt.Errorf("failed to copy file after %d attempts: %v", retryCount, err)
		}
		return nil
	}

	return fmt.Errorf("failed to copy file %s after %d attempts", temporaryFile.FileName, retryCount)
}

// createDirectoryInPod creates a directory in the helper pod
func (c *FreestyleJobCtl) createDirectoryInPod(ctx context.Context, client *kubernetes.Clientset, namespace, podName, dirPath string) error {
	mkdirReq := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	mkdirReq.VersionedParams(&corev1.PodExecOptions{
		Container: "file-helper",
		Command:   []string{"mkdir", "-p", dirPath},
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	mkdirExecutor, err := clientmanager.NewKubeClientManager().GetSPDYExecutor(c.jobTaskSpec.Properties.ClusterID, mkdirReq.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor for mkdir: %v", err)
	}

	var mkStdout, mkStderr bytes.Buffer
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- mkdirExecutor.Stream(remotecommand.StreamOptions{Stdout: &mkStdout, Stderr: &mkStderr})
	}()
	select {
	case err := <-doneCh:
		if err != nil {
			return fmt.Errorf("failed to create directory %s in pod: %v, stderr: %s", dirPath, err, mkStderr.String())
		}
	case <-time.After(2 * time.Minute):
		return fmt.Errorf("mkdir exec timed out: pod=%s dir=%s", podName, dirPath)
	}

	return nil
}

func (c *FreestyleJobCtl) copyFileToCluster(ctx context.Context, client *kubernetes.Clientset, namespace, podName, localFilePath, mountPath string) error {
	// Create target directory first
	if err := c.createDirectoryInPod(ctx, client, namespace, podName, mountPath); err != nil {
		return err
	}

	// Step 2: prepare tar stream from the local file
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		f, err := os.Open(localFilePath)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("open local file failed: %v", err))
			return
		}
		defer f.Close()

		tw := tar.NewWriter(pw)
		defer tw.Close()

		fi, err := f.Stat()
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("stat local file failed: %v", err))
			return
		}

		hdr := &tar.Header{
			Name: filepath.Base(localFilePath),
			Mode: 0644,
			Size: fi.Size(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("write tar header failed: %v", err))
			return
		}
		const logEveryBytes = 8 * 1024 * 1024 // 8MB
		buf := make([]byte, 2*1024*1024)      // 2MB chunks
		var written int64
		var lastLog int64
		for {
			n, rerr := f.Read(buf)
			if n > 0 {
				if _, werr := tw.Write(buf[:n]); werr != nil {
					_ = pw.CloseWithError(fmt.Errorf("write tar body failed: %v", werr))
					return
				}
				written += int64(n)
				if written-lastLog >= logEveryBytes {
					lastLog = written
				}
			}
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				_ = pw.CloseWithError(fmt.Errorf("read local file failed: %v", rerr))
				return
			}
		}
	}()

	// Step 3: untar stream in the helper pod at the mount path
	untarReq := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	// Use tar directly to avoid relying on shell; order of args is compatible with busybox tar
	untarCmd := []string{"tar", "-x", "-C", mountPath, "-f", "-"}
	untarReq.VersionedParams(&corev1.PodExecOptions{
		Container: "file-helper",
		Command:   untarCmd,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	untarExecutor, err := clientmanager.NewKubeClientManager().GetSPDYExecutor(c.jobTaskSpec.Properties.ClusterID, untarReq.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor for untar: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := untarExecutor.Stream(remotecommand.StreamOptions{Stdin: pr, Stdout: &stdout, Stderr: &stderr}); err != nil {
		return fmt.Errorf("failed to untar in pod: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// moveFileInHelper moves a file from source path to destination path within the helper pod
func (c *FreestyleJobCtl) moveFileInHelper(ctx context.Context, client *kubernetes.Clientset, namespace, podName, sourcePath, destPath string) error {
	// Validate paths
	if sourcePath == "" || destPath == "" {
		return fmt.Errorf("source path and destination path cannot be empty")
	}

	// Prevent moving to dangerous locations
	if destPath == "/" || destPath == "/bin" || destPath == "/usr" || destPath == "/etc" {
		c.logger.Warnf("Attempting to move file to potentially dangerous location: %s", destPath)
	}

	// Create destination directory first using shared function
	destDir := path.Dir(destPath)
	if err := c.createDirectoryInPod(ctx, client, namespace, podName, destDir); err != nil {
		return err
	}

	// Move the file
	moveReq := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	moveReq.VersionedParams(&corev1.PodExecOptions{
		Container: "file-helper",
		Command:   []string{"mv", sourcePath, destPath},
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	moveExecutor, err := clientmanager.NewKubeClientManager().GetSPDYExecutor(c.jobTaskSpec.Properties.ClusterID, moveReq.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor for move: %v", err)
	}

	var mvStdout, mvStderr bytes.Buffer
	doneCh2 := make(chan error, 1)
	go func() {
		doneCh2 <- moveExecutor.Stream(remotecommand.StreamOptions{Stdout: &mvStdout, Stderr: &mvStderr})
	}()
	select {
	case err := <-doneCh2:
		if err != nil {
			return fmt.Errorf("failed to move file in pod: %v, stderr: %s", err, mvStderr.String())
		}
	case <-time.After(1 * time.Minute):
		return fmt.Errorf("move exec timed out: pod=%s from=%s to=%s", podName, sourcePath, destPath)
	}

	c.logger.Infof("Successfully moved file from %s to %s in helper pod", sourcePath, destPath)
	return nil
}

func (c *FreestyleJobCtl) cleanupHelperPod(ctx context.Context, client crClient.Client, namespace, podName string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
	}

	err := client.Delete(ctx, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete helper pod: %v", err)
	}

	c.logger.Infof("Cleaned up helper pod %s", podName)
	return nil
}

func (c *FreestyleJobCtl) wait(ctx context.Context) {
	var err error
	taskTimeout := time.After(time.Duration(c.jobTaskSpec.Properties.Timeout) * time.Minute)
	c.job.Status, err = waitJobStart(ctx, c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, c.kubeclient, c.apiServer, taskTimeout, c.logger)
	if err != nil {
		c.job.Error = err.Error()
	}
	if c.job.Status == config.StatusRunning {
		c.ack()
	} else {
		return
	}
	c.job.Status, c.job.Error = waitJobEndByCheckingConfigMap(ctx, taskTimeout, c.jobTaskSpec.Properties.Namespace, c.job.K8sJobName, true, c.informer, c.job, c.ack, c.logger)
}

func (c *FreestyleJobCtl) vmJobWait(ctx context.Context, jobID string) {
	var err error
	timeout := time.After(time.Duration(c.jobTaskSpec.Properties.Timeout) * time.Minute)

	// check job whether start
	c.job.Status, err = waitVMJobStart(ctx, jobID, timeout, c.job, c.logger)
	if err != nil {
		c.job.Error = err.Error()
	}
	if c.job.Status == config.StatusRunning {
		c.ack()
	} else {
		return
	}

	c.job.Status, c.job.Error = waitVMJobEndByCheckStatus(ctx, jobID, timeout, c.job, c.ack, c.logger)

	switch c.job.Status {
	case config.StatusCancelled:
		err := vmmongodb.NewVMJobColl().UpdateStatus(jobID, string(config.StatusCancelled))
		if err != nil {
			c.logger.Errorf("update vm job status error: %v", err)
			c.job.Error = fmt.Errorf("update vm job status %s error: %v", string(config.ReleasePlanStatusCancel), err).Error()
		}
	case config.StatusTimeout:
		err := vmmongodb.NewVMJobColl().UpdateStatus(jobID, string(config.StatusTimeout))
		if err != nil {
			c.logger.Errorf("update vm job status error: %v", err)
			c.job.Error = fmt.Errorf("update vm job status %s error: %v", string(config.StatusTimeout), err).Error()
		}
	}
}

func (c *FreestyleJobCtl) complete(ctx context.Context) {
	jobLabel := &JobLabel{
		JobType: string(c.job.JobType),
		JobName: c.job.K8sJobName,
	}

	// 清理用户取消和超时的任务
	defer func() {
		go func() {
			if len(c.jobTaskSpec.Properties.Storages) > 0 {
				for _, storage := range c.jobTaskSpec.Properties.Storages {
					if storage.IsTemporary {
						if err := ensureDeletePVC(storage.PVC, c.jobTaskSpec.Properties.Namespace, storage, c.kubeclient); err != nil {
							c.logger.Error(err)
						}
					}
				}
			}
			// Cleanup files PVCs if they were created
			if c.hasFileTypes && len(c.filesPVCNames) > 0 {
				if err := c.cleanupFilesPVCs(); err != nil {
					c.logger.Errorf("Failed to cleanup files PVCs: %v", err)
				}
			}
			if err := ensureDeleteJob(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
			if err := ensureDeleteConfigMap(c.jobTaskSpec.Properties.Namespace, jobLabel, c.kubeclient); err != nil {
				c.logger.Error(err)
			}
		}()
	}()

	// get job outputs info from pod terminate message.
	if err := getJobOutputFromConfigMap(c.jobTaskSpec.Properties.Namespace, c.job.Name, c.job, c.workflowCtx, c.informer); err != nil {
		c.logger.Error(err)
		c.job.Status, c.job.Error = config.StatusFailed, errors.Wrap(err, "get job outputs").Error()
	}

	if err := saveContainerLog(c.jobTaskSpec.Properties.Namespace, c.jobTaskSpec.Properties.ClusterID, c.workflowCtx.WorkflowName, c.job.Name, c.workflowCtx.TaskID, jobLabel, c.kubeclient); err != nil {
		c.logger.Error(err)
		if c.job.Error == "" {
			c.job.Error = err.Error()
		}
		return
	}
	if err := stepcontroller.SummarizeSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Key, c.jobTaskSpec.Steps, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}

func (c *FreestyleJobCtl) vmComplete(ctx context.Context, jobID string) {
	defer func() {
		go func() {
			if err := vmmongodb.NewVMJobColl().DeleteByID(jobID, string(c.job.Status)); err != nil {
				c.logger.Error(fmt.Errorf("delete vm job error: %v", err))
			}
		}()
	}()

	// get job outputs info from job db
	if err := getVMJobOutputFromJobDB(jobID, c.job.Name, c.job, c.workflowCtx); err != nil {
		c.logger.Error(fmt.Errorf("get job outputs from job db error: %v", err))
		c.job.Status, c.job.Error = config.StatusFailed, fmt.Errorf("get job outputs from job db error: %v", err).Error()
	}

	// summarize steps
	if err := stepcontroller.SummarizeSteps(ctx, c.workflowCtx, &c.jobTaskSpec.Properties.Paths, c.job.Key, c.jobTaskSpec.Steps, c.logger); err != nil {
		c.logger.Error(err)
		c.job.Error = err.Error()
		return
	}
}

func getVMJobOutputFromJobDB(jobID, jobName string, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx) error {
	vmJob, err := vmmongodb.NewVMJobColl().FindByID(jobID)
	if err != nil {
		return err
	}
	if vmJob == nil {
		return errors.New("vm job not found")
	}
	outputs := vmJob.Outputs
	writeOutputs(outputs, job.Key, workflowCtx)

	return nil
}

func BuildJobExecutorContext(jobTaskSpec *commonmodels.JobTaskFreestyleSpec, job *commonmodels.JobTask, workflowCtx *commonmodels.WorkflowTaskCtx, logger *zap.SugaredLogger) *JobContext {
	var envVars, secretEnvVars []string
	var files []*JobFileInfo

	for _, env := range jobTaskSpec.Properties.Envs {
		// Handle file type environment variables separately for VM jobs
		if env.Type == commonmodels.FileType && env.FileID != "" && job.Infrastructure == setting.JobVMInfrastructure {
			fileInfo := &JobFileInfo{
				EnvKey:   env.Key,
				FileID:   env.FileID,
				FilePath: env.FilePath,
			}

			// Try to get the file name for better organization
			if commonmodels.GetFileNameByID != nil {
				if filename, err := commonmodels.GetFileNameByID(env.FileID); err == nil && filename != "" {
					fileInfo.FileName = filename
				} else {
					// Fallback: use env key as filename if we can't resolve it
					logger.Warnf("Failed to resolve filename for file ID %s (env: %s): %v", env.FileID, env.Key, err)
					fileInfo.FileName = env.Key
				}
			} else {
				// If resolver not available, use env key as filename
				fileInfo.FileName = env.Key
			}

			files = append(files, fileInfo)
			continue
		}

		// Handle regular environment variables
		if env.IsCredential {
			secretEnvVars = append(secretEnvVars, strings.Join([]string{env.Key, env.GetValue()}, "="))
			continue
		}
		envVars = append(envVars, strings.Join([]string{env.Key, env.GetValue()}, "="))
	}

	outputs := []string{}
	for _, output := range job.Outputs {
		outputs = append(outputs, output.Name)
	}

	jobContext := &JobContext{
		Name:          job.Name,
		Key:           job.Key,
		OriginName:    job.OriginName,
		DisplayName:   job.DisplayName,
		Envs:          envVars,
		SecretEnvs:    secretEnvVars,
		WorkflowName:  workflowCtx.WorkflowName,
		Workspace:     workflowCtx.Workspace,
		TaskID:        workflowCtx.TaskID,
		Outputs:       outputs,
		Paths:         jobTaskSpec.Properties.Paths,
		Steps:         jobTaskSpec.Steps,
		ConfigMapName: job.K8sJobName,
		Files:         files,
	}

	if job.Infrastructure == setting.JobVMInfrastructure {
		jobContext.Cache = &JobCacheConfig{
			CacheEnable:  jobTaskSpec.Properties.CacheEnable,
			CacheDirType: jobTaskSpec.Properties.CacheDirType,
			CacheUserDir: jobTaskSpec.Properties.CacheUserDir,
		}
	}

	return jobContext
}

func (c *FreestyleJobCtl) SaveInfo(ctx context.Context) error {
	// save delivery artifact for archive step
	if c.job.Status == config.StatusPassed {
		for _, stepTask := range c.jobTaskSpec.Steps {
			if stepTask.StepType == config.StepArchive {
				yamlString, err := yaml.Marshal(stepTask.Spec)
				if err != nil {
					return fmt.Errorf("marshal archive spec error: %v", err)
				}
				archiveSpec := &step.StepArchiveSpec{}
				if err := yaml.Unmarshal(yamlString, &archiveSpec); err != nil {
					return fmt.Errorf("unmarshal archive spec error: %v", err)
				}

				for _, upload := range archiveSpec.UploadDetail {
					if !upload.IsFileArchive {
						continue
					}
					deliveryArtifact := new(commonmodels.DeliveryArtifact)
					deliveryArtifact.CreatedBy = c.workflowCtx.WorkflowTaskCreatorUsername
					deliveryArtifact.CreatedTime = time.Now().Unix()
					deliveryArtifact.Source = string(config.WorkflowTypeV4)
					deliveryArtifact.Name = upload.ServiceModule + "_" + upload.ServiceName
					// TODO(Ray) file类型的交付物名称存放在Image和ImageTag字段是不规范的，优化时需要考虑历史数据的兼容问题。
					deliveryArtifact.Image = upload.Name
					deliveryArtifact.ImageTag = upload.Name
					deliveryArtifact.Type = string(config.File)
					deliveryArtifact.PackageFileLocation = upload.PackageFileLocation
					deliveryArtifact.PackageStorageURI = archiveSpec.S3.Endpoint + "/" + archiveSpec.S3.Bucket
					err := mongodb.NewDeliveryArtifactColl().Insert(deliveryArtifact)
					if err != nil {
						return fmt.Errorf("archiveCtl AfterRun: insert delivery artifact error: %v", err)
					}

					deliveryActivity := new(commonmodels.DeliveryActivity)
					deliveryActivity.Type = setting.BuildType
					deliveryActivity.ArtifactID = deliveryArtifact.ID
					deliveryActivity.JobTaskName = upload.JobTaskName
					deliveryActivity.URL = fmt.Sprintf("/v1/projects/detail/%s/pipelines/custom/%s/%d?display_name=%s", c.workflowCtx.ProjectName, c.workflowCtx.WorkflowName, c.workflowCtx.TaskID, url.QueryEscape(c.workflowCtx.WorkflowDisplayName))
					commits := make([]*commonmodels.ActivityCommit, 0)
					for _, repo := range archiveSpec.Repos {
						deliveryCommit := new(commonmodels.ActivityCommit)
						deliveryCommit.Address = repo.Address
						deliveryCommit.Source = repo.Source
						deliveryCommit.RepoOwner = repo.RepoOwner
						deliveryCommit.RepoName = repo.RepoName
						deliveryCommit.Branch = repo.Branch
						deliveryCommit.Tag = repo.Tag
						deliveryCommit.PR = repo.PR
						deliveryCommit.PRs = repo.PRs
						deliveryCommit.CommitID = repo.CommitID
						deliveryCommit.CommitMessage = repo.CommitMessage
						deliveryCommit.AuthorName = repo.AuthorName

						commits = append(commits, deliveryCommit)
					}
					deliveryActivity.Commits = commits

					deliveryActivity.CreatedBy = c.workflowCtx.WorkflowTaskCreatorUsername
					deliveryActivity.CreatedTime = time.Now().Unix()
					deliveryActivity.StartTime = c.workflowCtx.StartTime.Unix()
					deliveryActivity.EndTime = time.Now().Unix()

					err = mongodb.NewDeliveryActivityColl().Insert(deliveryActivity)
					if err != nil {
						return fmt.Errorf("archiveCtl AfterRun: build deliveryActivityColl insert err:%v", err)
					}
				}

				break
			}
		}
	}

	jobInfo := &commonmodels.JobInfo{
		Type:                c.job.JobType,
		WorkflowName:        c.workflowCtx.WorkflowName,
		WorkflowDisplayName: c.workflowCtx.WorkflowDisplayName,
		TaskID:              c.workflowCtx.TaskID,
		ProductName:         c.workflowCtx.ProjectName,
		StartTime:           c.job.StartTime,
		EndTime:             c.job.EndTime,
		Duration:            c.job.EndTime - c.job.StartTime,
		Status:              string(c.job.Status),
	}

	if c.job.JobType == string(config.JobZadigVMDeploy) {
		jobInfo.ServiceName = c.jobTaskSpec.Properties.ServiceName
		jobInfo.ServiceModule = c.jobTaskSpec.Properties.ServiceName
	}

	return mongodb.NewJobInfoColl().Create(context.TODO(), jobInfo)
}

// cleanupFilesPVCs removes all files PVCs
func (c *FreestyleJobCtl) cleanupFilesPVCs() error {
	// Get kubernetes client
	hubServerAddr := zadigconfig.HubServerServiceAddress()
	crClient, _, _, err := GetK8sClients(hubServerAddr, c.jobTaskSpec.Properties.ClusterID)
	if err != nil {
		return err
	}

	var namespace string
	if c.jobTaskSpec.Properties.ClusterID == setting.LocalClusterID {
		namespace = zadigconfig.Namespace()
	} else {
		namespace = setting.AttachedClusterNamespace
	}

	var errors []string
	for mountPath, pvcName := range c.filesPVCNames {
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: namespace,
			},
		}

		err = crClient.Delete(context.Background(), pvc)
		if err != nil && !apierrors.IsNotFound(err) {
			errors = append(errors, fmt.Sprintf("failed to delete PVC %s for mount path %s: %v", pvcName, mountPath, err))
		} else {
			c.logger.Infof("Cleaned up files PVC %s for mount path %s", pvcName, mountPath)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}

	return nil
}
