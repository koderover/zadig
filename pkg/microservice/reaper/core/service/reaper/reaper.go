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

package reaper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/tool/sonar"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/pkg/microservice/reaper/config"
	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util/fs"
)

const (
	ReadmeScriptFile = "readme_script.sh"
	ReadmeFile       = "/tmp/README"
)

type Reaper struct {
	Ctx             *meta.Context
	StartTime       time.Time
	ActiveWorkspace string
	UserEnvs        map[string]string
	Type            types.ReaperType

	cm CacheManager
}

func NewReaper() (*Reaper, error) {
	context, err := ioutil.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, fmt.Errorf("read job config file error: %v", err)
	}

	var ctx *meta.Context
	if err := yaml.Unmarshal(context, &ctx); err != nil {
		return nil, fmt.Errorf("cannot unmarshal job data: %v", err)
	}

	ctx.Paths = config.Path()

	reaper := &Reaper{
		Ctx: ctx,
		cm:  NewTarCacheManager(ctx.StorageURI, ctx.PipelineName, ctx.ServiceName, ctx.AesKey),
	}

	if ctx.TestType != "" {
		reaper.Type = types.TestReaperType
	} else if ctx.ScannerFlag {
		reaper.Type = types.ScanningReaperType
	} else {
		reaper.Type = types.BuildReaperType
	}

	workspace := "/workspace"
	if reaper.Ctx.ClassicBuild {
		workspace = reaper.Ctx.Workspace
	}
	err = reaper.EnsureActiveWorkspace(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure active workspace `%s`: %s", workspace, err)
	}

	userEnvs := reaper.getUserEnvs()
	reaper.UserEnvs = make(map[string]string, len(userEnvs))
	for _, env := range userEnvs {
		items := strings.Split(env, "=")
		if len(items) != 2 {
			continue
		}

		reaper.UserEnvs[items[0]] = items[1]
	}

	return reaper, nil
}

func (r *Reaper) GetCacheFile() string {
	return filepath.Join(r.Ctx.Workspace, "reaper.tar.gz")
}

func (r *Reaper) CompressCache(storageURI string) error {
	cacheDir := r.ActiveWorkspace
	if r.Ctx.CacheDirType == types.UserDefinedCacheDir {
		// Note: Product supports using environment variables, so we need to parsing the directory path here.
		cacheDir = r.renderUserEnv(r.Ctx.CacheUserDir)
	}

	log.Infof("Data in `%s` will be cached.", cacheDir)
	if err := r.cm.Archive(cacheDir, r.GetCacheFile()); err != nil {
		return fmt.Errorf("failed to cache %s: %s", cacheDir, err)
	}
	log.Infof("Succeed to cache %s.", cacheDir)

	// remove workspace
	err := os.RemoveAll(r.ActiveWorkspace)
	if err != nil {
		log.Errorf("RemoveAll err:%v", err)
		return err
	}
	return nil
}

func (r *Reaper) DecompressCache() error {
	cacheDir := r.ActiveWorkspace
	if r.Ctx.CacheDirType == types.UserDefinedCacheDir {
		// Note: Product supports using environment variables, so we need to parsing the directory path here.
		cacheDir = r.renderUserEnv(r.Ctx.CacheUserDir)
	}

	err := r.EnsureDir(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to ensure cache dir `%s`: %s", cacheDir, err)
	}

	log.Infof("Cache will be decompressed to %s.", cacheDir)
	err = r.cm.Unarchive(r.GetCacheFile(), cacheDir)
	if err != nil && strings.Contains(err.Error(), "decompression OK") {
		// could met decompression OK, trailing garbage ignored
		err = nil
	}

	return err
}

func (r *Reaper) EnsureActiveWorkspace(workspace string) error {
	if workspace == "" {
		tempWorkspace, err := ioutil.TempDir(os.TempDir(), "reaper")
		if err != nil {
			return fmt.Errorf("create workspace error: %v", err)
		}
		r.ActiveWorkspace = tempWorkspace
		return os.Chdir(r.ActiveWorkspace)
	}

	err := os.MkdirAll(workspace, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}
	r.ActiveWorkspace = workspace

	return os.Chdir(r.ActiveWorkspace)
}

func (r *Reaper) EnsureDir(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

func (r *Reaper) BeforeExec() error {
	r.StartTime = time.Now()

	// the execution preparation are not required, so we skip it if the reaper type is scanning
	if r.Type != types.ScanningReaperType && r.Type != types.TestReaperType {
		log.Infof("Checking Docker Connectivity.")
		startTimeCheckDocker := time.Now()
		for i := 0; i < 15; i++ {
			if err := dockerInfo().Run(); err == nil {
				break
			}
			time.Sleep(time.Second * 1)
		}
		log.Infof("Check ended. Duration: %.2f seconds.", time.Since(startTimeCheckDocker).Seconds())

		if r.Ctx.DockerRegistry != nil {
			if r.Ctx.DockerRegistry.UserName != "" {
				log.Infof("Logining Docker Registry: %s.", r.Ctx.DockerRegistry.Host)
				startTimeDockerLogin := time.Now()
				cmd := dockerLogin(r.Ctx.DockerRegistry.UserName, r.Ctx.DockerRegistry.Password, r.Ctx.DockerRegistry.Host)
				var out bytes.Buffer
				cmd.Stdout = &out
				cmd.Stderr = &out
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to login docker registry: %s %s", err, out.String())
				}

				log.Infof("Login ended. Duration: %.2f seconds.", time.Since(startTimeDockerLogin).Seconds())
			}
		}
	}

	if r.Ctx.CacheEnable && r.Ctx.Cache.MediumType == types.ObjectMedium {
		log.Info("Pulling Cache.")
		startTimePullCache := time.Now()
		if err := r.DecompressCache(); err != nil {
			// If the workflow runs for the first time, there may be no cache.
			log.Infof("Failed to pull cache: %s. Duration: %.2f seconds.", err, time.Since(startTimePullCache).Seconds())
		} else {
			log.Infof("Succeed to pull cache. Duration: %.2f seconds.", time.Since(startTimePullCache).Seconds())
		}
	}

	if err := os.MkdirAll(path.Join(os.Getenv("HOME"), "/.ssh"), os.ModePerm); err != nil {
		return fmt.Errorf("create ssh folder error: %v", err)
	}

	if r.Ctx.Archive != nil && len(r.Ctx.Archive.Dir) > 0 {
		if err := os.MkdirAll(r.Ctx.Archive.Dir, os.ModePerm); err != nil {
			return fmt.Errorf("create DistDir error: %v", err)
		}
	}

	if r.Ctx.Git != nil {
		if err := r.Ctx.Git.WriteGithubSSHFile(); err != nil {
			return fmt.Errorf("write github ssh file error: %v", err)
		}

		if err := r.Ctx.Git.WriteGitlabSSHFile(); err != nil {
			return fmt.Errorf("write gitlab ssh file error: %v", err)
		}

		if err := r.Ctx.Git.WriteKnownHostFile(); err != nil {
			return fmt.Errorf("write known_host file error: %v", err)
		}

		if err := r.Ctx.Git.WriteSSHConfigFile(r.Ctx.Proxy); err != nil {
			return fmt.Errorf("write ssh config error: %v", err)
		}
	}

	if r.Ctx.GinkgoTest != nil && len(r.Ctx.GinkgoTest.ResultPath) > 0 {
		r.Ctx.GinkgoTest.ResultPath = filepath.Join(r.ActiveWorkspace, r.Ctx.GinkgoTest.ResultPath)
		if err := os.RemoveAll(r.Ctx.GinkgoTest.ResultPath); err != nil {
			log.Warning(err.Error())
		}

		if err := os.MkdirAll(r.Ctx.GinkgoTest.ResultPath, os.ModePerm); err != nil {
			return fmt.Errorf("create test result path error: %v", err)
		}
	}

	return nil
}

func dockerBuildCmd(dockerfile, fullImage, ctx, buildArgs string, ignoreCache bool) *exec.Cmd {
	args := []string{"-c"}
	dockerCommand := "docker build --rm=true"
	if ignoreCache {
		dockerCommand += " --no-cache"
	}

	if buildArgs != "" {
		for _, val := range strings.Fields(buildArgs) {
			if val != "" {
				dockerCommand = dockerCommand + " " + val
			}
		}

	}
	dockerCommand = dockerCommand + " -t " + fullImage + " -f " + dockerfile + " " + ctx
	args = append(args, dockerCommand)
	return exec.Command("sh", args...)
}

func (r *Reaper) setProxy(ctx *meta.DockerBuildCtx, cfg *meta.Proxy) {
	if cfg.EnableRepoProxy && cfg.Type == "http" {
		if !strings.Contains(strings.ToLower(ctx.BuildArgs), "--build-arg http_proxy=") {
			ctx.BuildArgs = fmt.Sprintf("%s --build-arg http_proxy=%s", ctx.BuildArgs, cfg.GetProxyURL())
		}
		if !strings.Contains(strings.ToLower(ctx.BuildArgs), "--build-arg https_proxy=") {
			ctx.BuildArgs = fmt.Sprintf("%s --build-arg https_proxy=%s", ctx.BuildArgs, cfg.GetProxyURL())
		}
	}
}

func (r *Reaper) dockerCommands() []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)
	cmds = append(
		cmds,
		dockerBuildCmd(
			r.Ctx.DockerBuildCtx.GetDockerFile(),
			r.Ctx.DockerBuildCtx.ImageName,
			r.Ctx.DockerBuildCtx.WorkDir,
			r.Ctx.DockerBuildCtx.BuildArgs,
			r.Ctx.IgnoreCache,
		),
		dockerPush(r.Ctx.DockerBuildCtx.ImageName),
	)
	return cmds
}

func (r *Reaper) runDockerBuild() error {
	if r.Ctx.DockerBuildCtx == nil {
		return nil
	}

	log.Info("Preparing Dockerfile.")
	startTimePrepareDockerfile := time.Now()
	err := r.prepareDockerfile()
	if err != nil {
		return fmt.Errorf("failed to prepare dockerfile: %s", err)
	}
	log.Infof("Preparation ended. Duration: %.2f seconds.", time.Since(startTimePrepareDockerfile).Seconds())

	if r.Ctx.Proxy != nil {
		r.setProxy(r.Ctx.DockerBuildCtx, r.Ctx.Proxy)
	}

	log.Info("Runing Docker Build.")
	startTimeDockerBuild := time.Now()
	envs := r.getUserEnvs()
	for _, c := range r.dockerCommands() {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Dir = r.ActiveWorkspace
		c.Env = envs
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run docker build: %s", err)
		}
	}
	log.Infof("Docker build ended. Duration: %.2f seconds.", time.Since(startTimeDockerBuild).Seconds())

	return nil
}

func (r *Reaper) prepareDockerfile() error {
	if r.Ctx.DockerBuildCtx.Source == setting.DockerfileSourceTemplate {
		reader := strings.NewReader(r.Ctx.DockerBuildCtx.DockerTemplateContent)
		readCloser := io.NopCloser(reader)
		path := fmt.Sprintf("/%s", setting.ZadigDockerfilePath)
		err := fs.SaveFile(readCloser, path)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reaper) Exec() (err error) {
	log.Info("Installing Dependency Packages.")
	startTimeInstallDeps := time.Now()
	if err = r.runIntallationScripts(); err != nil {
		err = fmt.Errorf("failed to install dependency packages: %s", err)
		return
	}
	log.Infof("Install ended. Duration: %.2f seconds.", time.Since(startTimeInstallDeps).Seconds())

	log.Info("Cloning Repository.")
	startTimeCloneRepo := time.Now()
	if err = r.runGitCmds(); err != nil {
		err = fmt.Errorf("failed to clone repository: %s", err)
		return
	}
	log.Infof("Clone ended. Duration: %.2f seconds.", time.Since(startTimeCloneRepo).Seconds())

	if err = r.createReadme(ReadmeFile); err != nil {
		log.Warningf("Failed to create README file: %s", err)
	}
	// collect test result regardless of excution result
	defer func() {
		collectErr := r.CollectTestResults()
		if collectErr != nil {
			err = collectErr
		}
	}()
	log.Info("Executing User Build Script.")
	startTimeRunBuildScript := time.Now()
	if err = r.runScripts(); err != nil {
		err = fmt.Errorf("failed to execute user build script: %s", err)
		return
	}
	log.Infof("Execution ended. Duration: %.2f seconds.", time.Since(startTimeRunBuildScript).Seconds())

	// for sonar type we write the sonar parameter into config file and go with sonar-scanner command
	log.Info("Executing SonarQube Scanning process.")
	startTimeRunSonar := time.Now()
	if err = r.runSonarScanner(); err != nil {
		err = fmt.Errorf("failed to execute sonar scanning process, the error is: %s", err)
		return
	}
	log.Infof("Sonar scan ended. Duration %.2f seconds.", time.Since(startTimeRunSonar).Seconds())

	err = r.runDockerBuild()
	return
}

func (r *Reaper) CollectTestResults() error {
	if r.Ctx.GinkgoTest == nil {
		return nil
	}
	resultPath := r.Ctx.GinkgoTest.ResultPath
	if resultPath != "" && !strings.HasPrefix(resultPath, "/") {
		resultPath = filepath.Join(r.ActiveWorkspace, resultPath)
	}

	if r.Ctx.TestType == "" {
		r.Ctx.TestType = setting.FunctionTest
	}

	switch r.Ctx.TestType {
	case setting.FunctionTest:
		err := mergeGinkgoTestResults(r.Ctx.Archive.File, resultPath, r.Ctx.Archive.Dir, r.StartTime)
		if err != nil {
			return fmt.Errorf("failed to merge test result: %s", err)
		}
	case setting.PerformanceTest:
		err := JmeterTestResults(r.Ctx.Archive.File, resultPath, r.Ctx.Archive.Dir)
		if err != nil {
			return fmt.Errorf("failed to archive performance test result: %s", err)
		}
	}

	if len(r.Ctx.GinkgoTest.ArtifactPaths) > 0 {
		if err := artifactsUpload(r.Ctx, r.ActiveWorkspace, r.Ctx.GinkgoTest.ArtifactPaths); err != nil {
			return fmt.Errorf("failed to upload artifacts: %s", err)
		}
	}

	if err := r.archiveTestFiles(); err != nil {
		return fmt.Errorf("failed to archive test files: %s", err)
	}

	if err := r.archiveHTMLTestReportFile(); err != nil {
		return fmt.Errorf("failed to archive HTML test report: %s", err)
	}
	return nil
}

func (r *Reaper) AfterExec() error {
	if r.Ctx.ScannerFlag && r.Ctx.ScannerType == types.ScanningTypeSonar && r.Ctx.SonarCheckQualityGate {
		log.Info("Start check Sonar scanning quality gate status.")
		client := sonar.NewSonarClient(r.Ctx.SonarServer, r.Ctx.SonarLogin)
		sonarWorkDir := sonar.GetSonarWorkDir(r.Ctx.SonarParameter)
		if sonarWorkDir == "" {
			sonarWorkDir = ".scannerwork"
		}
		if !filepath.IsAbs(sonarWorkDir) {
			sonarWorkDir = filepath.Join("/workspace", r.Ctx.Repos[0].Name, sonarWorkDir)
		}
		taskReportDir := filepath.Join(sonarWorkDir, "report-task.txt")
		bytes, err := ioutil.ReadFile(taskReportDir)
		if err != nil {
			log.Errorf("read sonar task report file: %s error :%v", taskReportDir, err)
			return err
		}
		taskReportContent := string(bytes)
		ceTaskID := sonar.GetSonarCETaskID(taskReportContent)
		if ceTaskID == "" {
			log.Error("can not get sonar ce task ID")
			return errors.New("can not get sonar ce task ID")
		}
		analysisID, err := client.WaitForCETaskTobeDone(ceTaskID, time.Minute*10)
		if err != nil {
			log.Error(err)
			return err
		}
		gateInfo, err := client.GetQualityGateInfo(analysisID)
		if err != nil {
			log.Error(err)
			return err
		}
		log.Infof("Sonar quality gate status: %s", gateInfo.ProjectStatus.Status)
		sonar.PrintSonarConditionTables(gateInfo.ProjectStatus.Conditions)
		if gateInfo.ProjectStatus.Status != sonar.QualityGateOK && gateInfo.ProjectStatus.Status != sonar.QualityGateNone {
			return fmt.Errorf("sonar quality gate status was: %s", gateInfo.ProjectStatus.Status)
		}
	}

	if r.Ctx.ArtifactInfo == nil {
		if err := r.archiveS3Files(); err != nil {
			return fmt.Errorf("failed to archive S3 files: %s", err)
		}

		if err := r.RunPostScripts(); err != nil {
			return fmt.Errorf("failed to run postscripts: %s", err)
		}
	} else {
		if err := r.downloadArtifactFile(); err != nil {
			return fmt.Errorf("failed to download artifact files: %s", err)
		}
	}

	if r.Ctx.UploadEnabled {
		forcedPathStyle := true
		if r.Ctx.UploadStorageInfo.Provider == setting.ProviderSourceAli {
			forcedPathStyle = false
		}
		client, err := s3.NewClient(r.Ctx.UploadStorageInfo.Endpoint, r.Ctx.UploadStorageInfo.AK, r.Ctx.UploadStorageInfo.SK, r.Ctx.UploadStorageInfo.Insecure, forcedPathStyle)
		if err != nil {
			return fmt.Errorf("failed to create s3 client to upload file, err: %s", err)
		}
		for _, upload := range r.Ctx.UploadInfo {
			info, err := os.Stat(upload.AbsFilePath)
			if err != nil {
				return fmt.Errorf("failed to upload file path [%s] to destination [%s], the error is: %s", upload.AbsFilePath, upload.DestinationPath, err)
			}
			// if the given path is a directory
			if info.IsDir() {
				err := client.UploadDir(r.Ctx.UploadStorageInfo.Bucket, upload.AbsFilePath, upload.DestinationPath)
				if err != nil {
					log.Errorf("Failed to upload dir [%s] to path [%s] on s3, the error is: %s", upload.AbsFilePath, upload.DestinationPath, err)
					return err
				}
			} else {
				key := filepath.Join(upload.DestinationPath, info.Name())
				err := client.Upload(r.Ctx.UploadStorageInfo.Bucket, upload.AbsFilePath, key)
				if err != nil {
					log.Errorf("Failed to upload [%s] to key [%s] on s3, the error is: %s", upload.AbsFilePath, key, err)
					return err
				}
			}
		}
	}

	if r.Ctx.ArtifactPath != "" {
		if err := artifactsUpload(r.Ctx, r.ActiveWorkspace, []string{r.Ctx.ArtifactPath}, "buildv3"); err != nil {
			return fmt.Errorf("failed to upload artifacts: %s", err)
		}
	}

	if err := r.RunPMDeployScripts(); err != nil {
		return fmt.Errorf("failed to run deploy scripts on physical machine: %s", err)
	}

	// Upload workspace cache if the user turns on caching and uses object storage.
	// Note: Whether the cache is uploaded successfully or not cannot hinder the progress of the overall process,
	//       so only exceptions are printed here and the process is not interrupted.
	if r.Ctx.CacheEnable && r.Ctx.Cache.MediumType == types.ObjectMedium {
		log.Info("Uploading Build Cache.")
		startTimeUploadBuildCache := time.Now()
		if err := r.CompressCache(r.Ctx.StorageURI); err != nil {
			log.Warnf("Failed to upload build cache: %s. Duration: %.2f seconds.", err, time.Since(startTimeUploadBuildCache).Seconds())
		} else {
			log.Infof("Upload ended. Duration: %.2f seconds.", time.Since(startTimeUploadBuildCache).Seconds())
		}
	}

	return nil
}

func (r *Reaper) maskSecret(secrets []string, message string) string {
	out := message

	for _, val := range secrets {
		if len(val) == 0 {
			continue
		}
		out = strings.Replace(out, val, "********", -1)
	}
	return out
}

const (
	secretEnvMask = "********"
)

func (r *Reaper) maskSecretEnvs(message string) string {
	out := message

	for _, val := range r.Ctx.SecretEnvs {
		if len(val) == 0 {
			continue
		}
		sl := strings.Split(val, "=")

		if len(sl) != 2 {
			continue
		}

		if len(sl[0]) == 0 || len(sl[1]) == 0 {
			// invalid key value pair received
			continue
		}
		out = strings.Replace(out, strings.Join(sl[1:], "="), secretEnvMask, -1)
	}
	return out
}

func (r *Reaper) getUserEnvs() []string {
	envs := []string{
		"CI=true",
		"ZADIG=true",
		fmt.Sprintf("HOME=%s", config.Home()),
		fmt.Sprintf("WORKSPACE=%s", r.ActiveWorkspace),
		// TODO: readme文件可以使用别的方式代替
		fmt.Sprintf("README=%s", ReadmeFile),
	}

	r.Ctx.Paths = strings.Replace(r.Ctx.Paths, "$HOME", config.Home(), -1)
	envs = append(envs, fmt.Sprintf("PATH=%s", r.Ctx.Paths))
	envs = append(envs, fmt.Sprintf("DOCKER_HOST=%s", config.DockerHost()))
	envs = append(envs, r.Ctx.Envs...)
	envs = append(envs, r.Ctx.SecretEnvs...)

	return envs
}

func (r *Reaper) renderUserEnv(raw string) string {
	mapper := func(env string) string {
		return r.UserEnvs[env]
	}

	return os.Expand(raw, mapper)
}
