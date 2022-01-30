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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

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
	cm              CacheManager
	dogFeed         bool
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

	// 初始化容器Envs, 格式为: "key=value".
	// ctx.Envs = os.Environ()
	// 初始化容器Path
	ctx.Paths = config.Path()

	reaper := &Reaper{
		Ctx: ctx,
		cm:  NewTarCacheManager(ctx.StorageURI, ctx.PipelineName, ctx.ServiceName, ctx.AesKey),
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
	log.Infof("Succeed to cache %s", cacheDir)

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

	log.Info("wait for docker daemon to start ...")
	for i := 0; i < 15; i++ {
		if err := dockerInfo().Run(); err == nil {
			break
		}
		time.Sleep(time.Second * 1)
	}

	if r.Ctx.DockerRegistry != nil {
		if r.Ctx.DockerRegistry.UserName != "" {
			log.Infof("login docker registry %s", r.Ctx.DockerRegistry.Host)
			cmd := dockerLogin(r.Ctx.DockerRegistry.UserName, r.Ctx.DockerRegistry.Password, r.Ctx.DockerRegistry.Host)
			var out bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &out
			if err := cmd.Run(); err != nil {
				log.Errorf("docker login failed with error: %s\n%s", err, out.String())
				return fmt.Errorf("docker login failed with error: %s", err)
			}
		}
	}

	if r.Ctx.CacheEnable && r.Ctx.Cache.MediumType == types.ObjectMedium {
		log.Info("extracting workspace ...")
		if err := r.DecompressCache(); err != nil {
			// If the workflow runs for the first time, there may be no cache.
			log.Infof("no previous cache is found: %s", err)
		} else {
			log.Info("succeed to extract workspace")
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
		log.Infof("clean test result path %s", r.Ctx.GinkgoTest.ResultPath)
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
	if r.Ctx.DockerBuildCtx != nil {
		err := r.prepareDockerfile()
		if err != nil {
			return err
		}
		if r.Ctx.Proxy != nil {
			r.setProxy(r.Ctx.DockerBuildCtx, r.Ctx.Proxy)
		}

		envs := r.getUserEnvs()
		for _, c := range r.dockerCommands() {
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Dir = r.ActiveWorkspace
			c.Env = envs
			if err := c.Run(); err != nil {
				return err
			}
		}
	}

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

func (r *Reaper) Exec() error {
	if err := r.runIntallationScripts(); err != nil {
		return err
	}

	if err := r.runGitCmds(); err != nil {
		return err
	}

	if err := r.createReadme(ReadmeFile); err != nil {
		log.Warningf("create readme file error: %v", err)
	}

	if err := r.runScripts(); err != nil {
		return err
	}

	return r.runDockerBuild()
}

func (r *Reaper) AfterExec(upStreamErr error) error {
	if r.Ctx.GinkgoTest != nil && r.Ctx.GinkgoTest.ResultPath != "" {
		resultPath := r.Ctx.GinkgoTest.ResultPath
		if !strings.HasPrefix(resultPath, "/") {
			resultPath = filepath.Join(r.ActiveWorkspace, resultPath)
		}
		if r.Ctx.TestType == "" {
			r.Ctx.TestType = setting.FunctionTest
		}
		if r.Ctx.TestType == setting.FunctionTest {
			log.Info("Merge test result.")
			if err := mergeGinkgoTestResults(
				r.Ctx.Archive.File,
				resultPath,
				r.Ctx.Archive.Dir,
				r.StartTime,
			); err != nil {
				log.Errorf("Failed to merge results of ginkgo test: %s", err)
				return err
			}
		} else if r.Ctx.TestType == setting.PerformanceTest {
			log.Info("Archive performance test result.")
			if err := JmeterTestResults(
				r.Ctx.Archive.File,
				resultPath,
				r.Ctx.Archive.Dir,
			); err != nil {
				log.Errorf("Failed to archive results of performance test: %s", err)
				return err
			}
		}

		if len(r.Ctx.GinkgoTest.ArtifactPaths) > 0 {
			if err := artifactsUpload(r.Ctx, r.ActiveWorkspace, r.Ctx.GinkgoTest.ArtifactPaths); err != nil {
				log.Errorf("Failed to upload artifacts: %s", err)
				return err
			}
		}

		if err := r.archiveTestFiles(); err != nil {
			log.Errorf("Failed to archive test files: %s", err)
			return err
		}

		if err := r.archiveHTMLTestReportFile(); err != nil {
			log.Errorf("Failed to archive html test report: %s", err)
			return err
		}
	}

	if upStreamErr != nil {
		return nil
	}

	if r.Ctx.ArtifactInfo == nil {
		if err := r.archiveS3Files(); err != nil {
			log.Errorf("Failed to archive S3 files: %s", err)
			return err
		}
		if err := r.RunPostScripts(); err != nil {
			log.Errorf("Failed to run postscripts: %s", err)
			return err
		}
	} else {
		if err := r.downloadArtifactFile(); err != nil {
			log.Errorf("Failed to download artifact files: %s", err)
			return err
		}
	}

	if r.Ctx.ArtifactPath != "" {
		if err := artifactsUpload(r.Ctx, r.ActiveWorkspace, []string{r.Ctx.ArtifactPath}, "buildv3"); err != nil {
			log.Errorf("Failed to upload artifacts: %s", err)
			return err
		}
	}

	if err := r.RunPMDeployScripts(); err != nil {
		log.Errorf("Failed to run deploy scripts on physical machine: %s", err)
		return err
	}

	// Upload workspace cache if the user turns on caching and uses object storage.
	// Note: Whether the cache is uploaded successfully or not cannot hinder the progress of the overall process,
	//       so only exceptions are printed here and the process is not interrupted.
	if r.Ctx.CacheEnable && r.Ctx.Cache.MediumType == types.ObjectMedium {
		if err := r.CompressCache(r.Ctx.StorageURI); err != nil {
			log.Warnf("Failed to run compress cache: %s", err)
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
