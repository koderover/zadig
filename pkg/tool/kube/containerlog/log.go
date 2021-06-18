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

package containerlog

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func GetContainerLogs(namespace, podName, containerName string, follow bool, tailLines int64, out io.Writer, clientset kubernetes.Interface) error {
	readCloser, err := GetContainerLogStream(context.TODO(), namespace, podName, containerName, follow, tailLines, clientset)
	if err != nil {
		return err
	}

	defer func() {
		_ = readCloser.Close()
	}()

	_, err = io.Copy(out, readCloser)
	return err
}

func GetContainerLogStream(ctx context.Context, namespace, podName, containerName string, follow bool, tailLines int64, clientset kubernetes.Interface) (io.ReadCloser, error) {
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
	}

	if tailLines > 0 {
		logOptions.TailLines = &tailLines
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	return req.Stream(ctx)
}
