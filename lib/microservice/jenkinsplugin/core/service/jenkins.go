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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bndr/gojenkins"
	"gopkg.in/yaml.v3"

	"github.com/koderover/zadig/lib/microservice/jenkinsplugin/config"
)

// JenkinsPlugin ...
type JenkinsPlugin struct {
	config *Config
}

// Config ...
type Config struct {
	JobType            string              `yaml:"job_type"`
	OnSetup            string              `yaml:"setup,omitempty"`
	JenkinsIntegration *JenkinsIntegration `yaml:"jenkins_integration"`
	JenkinsBuild       *JenkinsBuild
}

type JenkinsIntegration struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type JenkinsBuild struct {
	JobName           string               `json:"job_name"`
	JenkinsBuildParam []*JenkinsBuildParam `json:"jenkins_build_param"`
}

type JenkinsBuildParam struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func NewJenkinsPlugin() (*JenkinsPlugin, error) {
	configContent, err := ioutil.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, err
	}

	var config *Config
	if err := yaml.Unmarshal(configContent, &config); err != nil {
		return nil, err
	}

	jenkinsPlugin := &JenkinsPlugin{
		config: config,
	}
	return jenkinsPlugin, nil
}

// BeforeExec ...
func (p *JenkinsPlugin) getJenkinsClient(ctx context.Context) (*gojenkins.Jenkins, error) {
	jenkinsIntegration := p.config.JenkinsIntegration
	jenkinsClient, err := gojenkins.CreateJenkins(nil, jenkinsIntegration.Url, jenkinsIntegration.Username, jenkinsIntegration.Password).Init(ctx)
	if err != nil {
		Errorf("create jenkins client err:%v", err)
		return nil, err
	}
	return jenkinsClient, nil
}

// Exec ...
func (p *JenkinsPlugin) Exec(ctx context.Context) error {
	jenkinsClient, err := p.getJenkinsClient(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	for _, param := range p.config.JenkinsBuild.JenkinsBuildParam {
		params[param.Name] = fmt.Sprintf("%v", param.Value)
	}
	queueID, err := jenkinsClient.BuildJob(ctx, p.config.JenkinsBuild.JobName, params)
	if err != nil {
		Errorf("create jenkins job build err:%v", err)
		return err
	}
	err = p.afterExec(ctx, jenkinsClient, queueID)
	if err != nil {
		return err
	}
	return nil
}

// AfterExec ...
func (p *JenkinsPlugin) afterExec(ctx context.Context, jenkinsClient *gojenkins.Jenkins, queueID int64) error {
	job, err := jenkinsClient.GetJob(ctx, p.config.JenkinsBuild.JobName)
	if err != nil {
		Errorf("get jenkins job err:%v", err)
		return err
	}
	task, err := jenkinsClient.GetQueueItem(ctx, queueID)
	if err != nil {
		Errorf("get jenkins queue item err:%v", err)
		return err
	}

	var buildID int64 = 0
	// 最多等待30秒
	for i := 0; i < 30; i++ {
		buildID = task.Raw.Executable.Number
		if buildID > 0 {
			break
		}

		time.Sleep(time.Second)
		_, err = task.Poll(ctx)
		if err != nil {
			Errorf("get jenkins queue poll task err:%v", err)
			return err
		}
	}
	Infof("Jenkins buildUrl: %s%d", task.Raw.Task.URL, buildID)
	Infof("Jenkins buildNumber: %d", buildID)
	jobBuild, err := getBuild(ctx, buildID, jenkinsClient, job)
	if err != nil {
		Errorf("get jenkins build detail err:%v", err)
		return err
	}
	p.outputLog(ctx, jobBuild)
	return nil
}

func (p *JenkinsPlugin) outputLog(ctx context.Context, build *gojenkins.Build) {
	var stringBuffer bytes.Buffer
	for {
		output, err := build.GetConsoleOutputFromIndex(ctx, 0)
		if err != nil {
			Errorf("output log err:%v", err)
			break
		}
		time.Sleep(time.Second)
		oldContent := stringBuffer.String()
		content := strings.Replace(output.Content, oldContent, "", -1)
		Info(content)
		stringBuffer.WriteString(content)
		if !output.HasMoreText {
			break
		}
	}
}

func getBuild(ctx context.Context, buildID int64, jenkins *gojenkins.Jenkins, job *gojenkins.Job) (*gojenkins.Build, error) {
	// There is old domain name in this oldURL
	oldURL, err := url.Parse(job.Raw.URL)
	if err != nil {
		Errorf("url parse err:%v", err)
		return nil, err
	}
	newURL, err := url.Parse(job.Jenkins.Server)
	if err != nil {
		Errorf("url parse err:%v", err)
		return nil, err
	}
	oldURL.Host = newURL.Host
	jobURL := oldURL.String()
	build := gojenkins.Build{Jenkins: jenkins, Job: job, Raw: new(gojenkins.BuildResponse), Depth: 1, Base: jobURL + "/" + strconv.FormatInt(buildID, 10)}
	build.Jenkins.Requester.Base = ""
	status, err := build.Poll(ctx)
	if err != nil {
		Errorf("build poll err:%v", err)
		return nil, err
	}
	if status == 200 {
		return &build, nil
	}
	return nil, nil
}
