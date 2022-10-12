package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	jenkins "github.com/bndr/gojenkins"
	"github.com/spf13/viper"

	"github.com/koderover/zadig/pkg/tool/log"
)

// envs
const (
	JenkinsAddress = "JENKINS_ADDRESS"
	Username       = "USERNAME"
	Password       = "PASSWORD"
	PipelineName   = "PIPELINE_NAME"
	Parameters     = "PARAMETERS"
)

func main() {
	log.Init(&log.Config{
		Level:       "info",
		Development: false,
		MaxSize:     5,
	})
	viper.AutomaticEnv()

	addr := viper.GetString(JenkinsAddress)
	pipelineName := viper.GetString(PipelineName)
	username := viper.GetString(Username)
	password := viper.GetString(Password)
	parameters := viper.GetString(Parameters)

	log.Infof("executing jenkins pipeline: %s on server %s", pipelineName, addr)

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: transport}
	jenkinsClient, err := jenkins.CreateJenkins(client, addr, username, password).Init(context.TODO())

	if err != nil {
		log.Infof("failed to create jenkins client for server: %s, the error is: %s", addr, err)
		os.Exit(1)
	}

	job, err := jenkinsClient.GetJob(context.TODO(), pipelineName)
	if err != nil {
		log.Infof("failed to get jenkins job, error is: %s", err)
		os.Exit(1)
	}

	params := make(map[string]string)

	// decode the given parameters
	parameterList := strings.Split(parameters, "\n")
	for _, param := range parameterList {
		kv := strings.Split(param, "=")
		// if there are nothing in this line, or no value is provided, we simply skip this
		if len(kv) <= 1 {
			continue
		}
		value := strings.Join(kv[1:], "=")
		params[kv[0]] = value
	}

	var offset int64 = 0

	queueid, err := job.InvokeSimple(context.TODO(), params)

	build, err := jenkinsClient.GetBuildFromQueueID(context.TODO(), queueid)
	if err != nil {
		log.Infof("failed to get build info from jenkins, error is: %s", err)
		os.Exit(1)
	}

	for build.IsRunning(context.TODO()) {
		time.Sleep(5000 * time.Millisecond)
		build.Poll(context.TODO())
		consoleOutput, err := build.GetConsoleOutputFromIndex(context.TODO(), offset)
		if err != nil {
			log.Warnf("[Jenkins Plugin] failed to get logs from jenkins job, error: %s", err)
		}
		fmt.Printf("%s", consoleOutput.Content)
		offset += consoleOutput.Offset
	}

	if !build.IsGood(context.TODO()) {
		log.Infof("jenkins job failed")
		os.Exit(1)
	} else {
		log.Infof("jenkins job completed")
	}
}
