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
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/koderover/zadig/v2/pkg/microservice/reaper/config"
	"github.com/koderover/zadig/v2/pkg/microservice/reaper/internal/s3"
	"github.com/koderover/zadig/v2/pkg/tool/httpclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

func (r *Reaper) runIntallationScripts() error {
	var (
		openProxy                   bool
		proxyScript, disProxyScript string
	)

	if r.Ctx.Proxy.EnableApplicationProxy {
		openProxy = true
		proxyScript = fmt.Sprintf("\nexport http_proxy=%s\nexport https_proxy=%s\n", r.Ctx.Proxy.GetProxyURL(), r.Ctx.Proxy.GetProxyURL())
		disProxyScript = "\nunset http_proxy https_proxy"
	}

	for i, install := range r.Ctx.Installs {
		var tmpPath string
		scripts := []string{}
		scripts = append(scripts, "set -ex")

		// 获取用户指定环境变量
		r.Ctx.Envs = append(r.Ctx.Envs, install.Envs.Environs()...)

		// 添加用户指定执行路径到PATH
		if install.BinPath != "" {
			r.Ctx.Paths = fmt.Sprintf("%s:%s", r.Ctx.Paths, install.BinPath)
		}

		if openProxy {
			scripts = append(scripts, proxyScript)
		}

		// 如果应用有配置下载路径
		if install.Download != "" {
			// 执行脚本之前检查缓存
			var store *s3.S3
			var err error
			store = &s3.S3{
				Ak:       r.Ctx.StorageAK,
				Sk:       r.Ctx.StorageSK,
				Endpoint: r.Ctx.StorageEndpoint,
				Bucket:   r.Ctx.StorageBucket,
				Insecure: true,
				Provider: r.Ctx.StorageProvider,
				Region:   r.Ctx.StorageRegion,
			}
			store.Subfolder = fmt.Sprintf("%s/%s-v%s", config.ConstructCachePath, install.Name, install.Version)

			filepath := strings.Split(install.Download, "/")
			fileName := filepath[len(filepath)-1]
			tmpPath = path.Join(os.TempDir(), fileName)
			s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, int8(store.Provider))
			if err == nil {
				objectKey := store.GetObjectPath(fileName)
				err = s3client.Download(
					store.Bucket,
					objectKey,
					tmpPath,
				)

				// 缓存不存在
				if err != nil {
					err := httpclient.Download(install.Download, tmpPath)
					if err != nil {
						return err
					}
					s3client.Upload(
						store.Bucket,
						tmpPath,
						objectKey,
					)
					log.Infof("Package loaded from url: %s", install.Download)
				}
			} else {
				err := httpclient.Download(install.Download, tmpPath)
				if err != nil {
					return err
				}
			}
		}

		for j, command := range install.Scripts {
			realCommand := strings.ReplaceAll(command, config.FilepathParam, tmpPath)
			install.Scripts[j] = realCommand
		}

		scripts = append(scripts, install.Scripts...)

		if openProxy {
			scripts = append(scripts, disProxyScript)
		}

		file := filepath.Join(os.TempDir(), fmt.Sprintf("install_script_%d.sh", i))
		if err := ioutil.WriteFile(file, []byte(strings.Join(scripts, "\n")), 0700); err != nil {
			return fmt.Errorf("write script file error: %v", err)
		}

		cmd := exec.Command("/bin/bash", file)
		cmd.Dir = r.ActiveWorkspace
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = r.getUserEnvs()

		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reaper) createReadme(file string) error {
	if r.Ctx.Archive == nil || len(r.Ctx.Repos) == 0 {
		return nil
	}

	scripts := []string{}
	scripts = append(scripts, "set -e")

	scripts = append(scripts, "echo DATE: `date +%Y-%m-%d-%H-%M-%S` > "+file)
	scripts = append(scripts, fmt.Sprintf("echo PKG_FILE: %s >> %s", r.Ctx.Archive.File, file))
	scripts = append(scripts, fmt.Sprintf("echo BUILD_URL: %s >> %s", config.BuildURL(), file))
	scripts = append(scripts, fmt.Sprintf("echo GO_VERSION: `which go >/dev/null && go version` >> %s", file))
	scripts = append(scripts, fmt.Sprintf("echo >> %s", file))
	scripts = append(scripts, fmt.Sprintf("echo GIT-COMMIT: >> %s", file))

	for _, repo := range r.Ctx.Repos {
		if repo == nil || len(repo.Name) == 0 {
			continue
		}

		workDir := filepath.Join(r.ActiveWorkspace, repo.Name)
		if len(repo.CheckoutPath) != 0 {
			workDir = filepath.Join(r.ActiveWorkspace, repo.CheckoutPath)
		}

		scripts = append(scripts, fmt.Sprintf("cd %s", workDir))
		scripts = append(scripts,
			fmt.Sprintf("echo %s: %s https://github.com/%s/%s/commit/`git rev-parse HEAD` >> %s",
				repo.Name,
				repo.Branch,
				repo.Owner,
				repo.Name,
				file,
			))
	}
	scripts = append(scripts, fmt.Sprintf("echo >> %s", file))

	sfile := filepath.Join(os.TempDir(), ReadmeScriptFile)
	if err := ioutil.WriteFile(sfile, []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return fmt.Errorf("write script file error: %v", err)
	}

	cmd := exec.Command("/bin/bash", sfile)
	cmd.Dir = r.ActiveWorkspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *Reaper) runScripts() error {
	if len(r.Ctx.Scripts) == 0 {
		return nil
	}
	scripts := r.prepareScriptsEnv()
	// avoid non-blocking IO for stdout to workaround "stdout: write error"
	for _, script := range r.Ctx.Scripts {
		scripts = append(scripts, script)
	}

	userScriptFile := "user_script.sh"
	if err := ioutil.WriteFile(filepath.Join(os.TempDir(), userScriptFile), []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return fmt.Errorf("write script file error: %v", err)
	}

	cmd := exec.Command("/bin/bash", filepath.Join(os.TempDir(), userScriptFile))
	cmd.Dir = r.ActiveWorkspace
	cmd.Env = r.getUserEnvs()

	fileName := filepath.Join(os.TempDir(), "user_script.log")
	//如果文件不存在就创建文件，避免后面使用变量出错
	util.WriteFile(fileName, []byte{}, 0700)

	needPersistentLog := len(r.Ctx.PostScripts) > 0

	var wg sync.WaitGroup

	cmdStdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		r.handleCmdOutput(cmdStdoutReader, needPersistentLog, fileName)
	}()

	cmdStdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		r.handleCmdOutput(cmdStdErrReader, needPersistentLog, fileName)
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}

func (r *Reaper) runSonarScanner() error {
	// first we write the sonar scanner config to the config file
	for _, repo := range r.Ctx.Repos {
		repoPath := repo.Name
		if repo.CheckoutPath != "" {
			repoPath = repo.CheckoutPath
		}
		log.Infof("Writing sonar-project.properties for repo: %s under relative path %s", repo.Name, repoPath)
		repoConfigPath := filepath.Join("/workspace", repoPath, "sonar-project.properties")
		// renders the scanned repository branch information to the user configuration
		r.Ctx.SonarParameter = strings.ReplaceAll(r.Ctx.SonarParameter, "$BRANCH", repo.Branch)
		configContent := fmt.Sprintf("sonar.login=%s\nsonar.host.url=%s\n%s", r.Ctx.SonarLogin, r.Ctx.SonarServer, r.Ctx.SonarParameter)
		err := os.WriteFile(repoConfigPath, []byte(configContent), fs.ModeAppend)
		if err != nil {
			log.Errorf("failed to write sonar-project.properties for repo: %s, the error is: %s", repo.Name, err)
			return err
		}
	}
	scanPath := r.Ctx.Repos[0].Name
	if r.Ctx.Repos[0].CheckoutPath != "" {
		scanPath = r.Ctx.Repos[0].CheckoutPath
	}
	// then we simply run the sonar-scanner command to commence the scan
	cmd := exec.Command("sonar-scanner")
	// since currently only one codehost is supported, we will just use the first repository
	cmd.Dir = filepath.Join("/workspace", scanPath)
	fileName := filepath.Join(os.TempDir(), "sonar.log")
	util.WriteFile(fileName, []byte{}, 0700)
	var wg sync.WaitGroup

	cmdStdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		r.handleCmdOutput(cmdStdoutReader, false, fileName)
	}()

	cmdStdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		r.handleCmdOutput(cmdStdErrReader, false, fileName)
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}

func (r *Reaper) prepareScriptsEnv() []string {
	scripts := []string{}
	if r.Ctx.ScannerFlag && r.Ctx.ScannerType == types.ScanningTypeSonar {
		return scripts
	}
	scripts = append(scripts, "eval $(ssh-agent -s) > /dev/null")
	// $HOME/.ssh/id_rsa 为 github 私钥
	scripts = append(scripts, fmt.Sprintf("ssh-add %s/.ssh/id_rsa.github &> /dev/null", config.Home()))
	scripts = append(scripts, fmt.Sprintf("rm %s/.ssh/id_rsa.github &> /dev/null", config.Home()))
	// $HOME/.ssh/gitlab 为 gitlab 私钥
	scripts = append(scripts, fmt.Sprintf("ssh-add %s/.ssh/id_rsa.gitlab &> /dev/null", config.Home()))
	scripts = append(scripts, fmt.Sprintf("rm %s/.ssh/id_rsa.gitlab &> /dev/null", config.Home()))

	return scripts
}

func (r *Reaper) RunPostScripts() error {
	if len(r.Ctx.PostScripts) == 0 {
		return nil
	}
	scripts := make([]string, 0, len(r.Ctx.PostScripts)+1)
	scripts = append(scripts, "echo \"----------------------以下是Shell脚本执行日志----------------------\"\n")
	scripts = append(scripts, r.Ctx.PostScripts...)

	postScriptFile := "post_script.sh"
	if err := ioutil.WriteFile(filepath.Join(os.TempDir(), postScriptFile), []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return fmt.Errorf("write post script file error: %v", err)
	}

	cmd := exec.Command("/bin/bash", filepath.Join(os.TempDir(), postScriptFile))
	cmd.Dir = r.ActiveWorkspace
	cmd.Env = r.getUserEnvs()

	cmdOutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	outScanner := bufio.NewScanner(cmdOutReader)
	go func() {
		for outScanner.Scan() {
			fmt.Printf("%s\n", r.maskSecretEnvs(outScanner.Text()))
		}
	}()

	cmdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	errScanner := bufio.NewScanner(cmdErrReader)
	go func() {
		for errScanner.Scan() {
			fmt.Printf("%s\n", r.maskSecretEnvs(errScanner.Text()))
		}
	}()

	return cmd.Run()
}

// @note run pm deploy scripts
func (r *Reaper) RunPMDeployScripts() error {
	if len(r.Ctx.PMDeployScripts) == 0 {
		return nil
	}

	scripts := r.Ctx.PMDeployScripts
	pmDeployScriptFile := "pm_deploy_script.sh"
	if err := ioutil.WriteFile(filepath.Join(os.TempDir(), pmDeployScriptFile), []byte(strings.Join(scripts, "\n")), 0700); err != nil {
		return fmt.Errorf("write script file error: %v", err)
	}

	cmd := exec.Command("/bin/bash", filepath.Join(os.TempDir(), pmDeployScriptFile))
	cmd.Dir = r.ActiveWorkspace

	cmd.Env = r.getUserEnvs()
	// ssh连接参数
	for _, ssh := range r.Ctx.SSHs {
		decodeBytes, err := base64.StdEncoding.DecodeString(ssh.PrivateKey)
		if err != nil {
			return fmt.Errorf("decode private_key failed, error: %v", err)
		}
		if err = ioutil.WriteFile(filepath.Join(os.TempDir(), ssh.Name+"_PK"), decodeBytes, 0600); err != nil {
			return fmt.Errorf("write private_key file error: %v", err)
		}

		cmd.Env = append(cmd.Env, fmt.Sprintf("%s_PK=%s", ssh.Name, filepath.Join(os.TempDir(), ssh.Name+"_PK")))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s_IP=%s", ssh.Name, ssh.IP))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s_PORT=%d", ssh.Name, ssh.Port))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s_USERNAME=%s", ssh.Name, ssh.UserName))

		r.Ctx.SecretEnvs = append(r.Ctx.SecretEnvs, fmt.Sprintf("%s_PK=%s", ssh.Name, filepath.Join(os.TempDir(), ssh.Name+"_PK")))
	}

	cmdOutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	outScanner := bufio.NewScanner(cmdOutReader)
	go func() {
		for outScanner.Scan() {
			fmt.Printf("%s\n", r.maskSecretEnvs(outScanner.Text()))
		}
	}()

	cmdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	errScanner := bufio.NewScanner(cmdErrReader)
	go func() {
		for errScanner.Scan() {
			fmt.Printf("%s\n", r.maskSecretEnvs(errScanner.Text()))
		}
	}()

	return cmd.Run()
}

func (r *Reaper) downloadArtifactFile() error {
	var err error
	var store *s3.S3
	if store, err = s3.UnmarshalNewS3StorageFromEncrypted(r.Ctx.ArtifactInfo.URL, r.Ctx.AesKey); err != nil {
		log.Errorf("Archive failed to create s3 storage %s", r.Ctx.ArtifactInfo.URL)
		return err
	}
	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%d/%s", store.Subfolder, r.Ctx.ArtifactInfo.WorkflowName, r.Ctx.ArtifactInfo.TaskID, "file")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%d/%s", r.Ctx.ArtifactInfo.WorkflowName, r.Ctx.ArtifactInfo.TaskID, "file")
	}

	s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, int8(store.Provider))
	if err != nil {
		log.Errorf("s3 create client err:%s", err)
		return err
	}
	files, err := s3client.ListFiles(store.Bucket, store.GetObjectPath(r.Ctx.ArtifactInfo.FileName), false)
	if err != nil {
		log.Errorf("s3 list file err:%s", err)
		return err
	}
	if len(files) > 0 {
		if err = s3client.Download(store.Bucket, files[0], fmt.Sprintf("%s/%s", r.ActiveWorkspace, r.Ctx.ArtifactInfo.FileName)); err != nil {
			log.Errorf("s3 download file err:%s", err)
			return err
		}
	}
	return nil
}

func (r *Reaper) handleCmdOutput(pipe io.ReadCloser, needPersistentLog bool, logFile string) {
	reader := bufio.NewReader(pipe)

	for {
		lineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Errorf("Failed to read log when processing cmd output: %s", err)
			break
		}

		fmt.Printf("%s", r.maskSecretEnvs(string(lineBytes)))

		if needPersistentLog {
			err := util.WriteFile(logFile, lineBytes, 0700)
			if err != nil {
				log.Warnf("Failed to write file when processing cmd output: %s", err)
			}
		}
	}
}
