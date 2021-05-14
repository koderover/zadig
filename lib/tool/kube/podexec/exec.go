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

package podexec

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/exec"

	krkubeclient "github.com/koderover/zadig/lib/tool/kube/client"
)

// ExecOptions passed to ExecWithOptions
type ExecOptions struct {
	Command       []string
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         io.Reader
	// If false, whitespace in std{err,out} will be removed.
	PreserveWhitespace bool
}

// ExecWithOptions executes a command in the specified container,
// returning stdout, stderr and error. `options` allowed for
// additional parameters to be passed.
func ExecWithOptions(options ExecOptions) (string, string, bool, error) {
	const tty = false

	req := krkubeclient.Clientset().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(options.PodName).
		Namespace(options.Namespace).
		SubResource("exec").
		Param("container", options.ContainerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: options.ContainerName,
		Command:   options.Command,
		Stdin:     options.Stdin != nil,
		Stdout:    true,
		Stderr:    true,
		TTY:       tty,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(krkubeclient.RESTConfig(), http.MethodPost, req.URL())
	if err != nil {
		return "", "", false, err
	}

	var stdout, stderr bytes.Buffer

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  options.Stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    tty,
	})

	if err != nil {
		if _, ok := err.(exec.ExitError); ok {
			return "", "", false, nil
		}
		return "", "", false, err
	}

	if options.PreserveWhitespace {
		return stdout.String(), stderr.String(), true, nil
	}

	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), true, nil
}
