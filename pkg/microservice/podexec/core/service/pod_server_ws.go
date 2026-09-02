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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	auditservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/terminalaudit"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func ServeWs(c *gin.Context) {
	ctx, err := internalhandler.NewContextWithAuthorization(c)
	defer func() { internalhandler.JSONResponse(c, ctx) }()

	if err != nil {
		ctx.RespErr = fmt.Errorf("authorization Info Generation failed: err %s", err)
		ctx.UnAuthorized = true
		return
	}

	podName := c.Param("podName")
	containerName := c.Param("containerName")

	if podName == "" {
		ctx.RespErr = e.ErrInvalidParam.AddDesc("containerName can't be empty,please check!")
		return
	}
	log.Infof("exec containerName: %s, pod: %s", containerName, podName)

	productName := c.Query("projectName")
	envName := c.Param("envName")
	production := strings.HasPrefix(c.FullPath(), "/api/podexec/production/")
	productInfo, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:       productName,
		EnvName:    envName,
		Production: &production,
	})
	if err != nil {
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("failed to find product %s/%s, err: %s", productName, envName, err))
		return
	}
	namespace, clusterID := productInfo.Namespace, productInfo.ClusterID

	pty, err := NewTerminalSession(c.Writer, c.Request, nil)
	if err != nil {
		log.Errorf("get pty failed: %v", err)
		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("get pty failed: %v", err))
		return
	}
	initialCols, initialRows := readTerminalSizeFromQuery(c)
	finalStatus := commonmodels.TerminalSessionStatusFinished
	var audit *auditservice.AuditSession
	defer func() {
		_ = pty.Close()
		if audit == nil {
			return
		}
		if err := audit.Close(finalStatus); err != nil {
			log.Errorf("close terminal audit recorder failed: %v", err)
		}
	}()

	kubeCli, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(clusterID)
	if err != nil {
		msg := fmt.Sprintf("get kubecli err :%v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))

		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("get kubecli err :%v", err))
		return
	}

	pod, err := getValidatedPod(kubeCli, namespace, podName, containerName)
	if err != nil {
		msg := fmt.Sprintf("Validate pod error! err: %v", err)
		log.Errorf(msg)
		_, _ = pty.Write([]byte(msg))

		ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("Validate pod error! err: %v", err))
		return
	}
	secrets, secretErr := collectContainerSecretValues(c.Request.Context(), kubeCli, pod, namespace, containerName)
	if secretErr != nil {
		log.Warnf("collect pod secret values for terminal audit failed, continuing without audit: %v", secretErr)
	} else {
		meta := &auditservice.SessionMeta{
			SessionType:   commonmodels.TerminalSessionTypePodExec,
			Protocol:      "k8s-exec",
			UserID:        ctx.UserID,
			Username:      ctx.UserName,
			Account:       ctx.Account,
			ProjectName:   productName,
			EnvName:       envName,
			ServiceName:   resolvePodServiceName(kubeCli, productInfo, pod),
			TargetName:    fmt.Sprintf("%s/%s", podName, containerName),
			RemoteAddr:    pod.Status.PodIP,
			ClusterID:     clusterID,
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: containerName,
			ClientIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			InitialCols:   initialCols,
			InitialRows:   initialRows,
			Secrets:       secrets,
		}
		session, auditErr := auditservice.NewAuditSession(meta, func() {
			_ = pty.Close()
		})
		if auditErr != nil {
			log.Errorf("create podexec terminal audit recorder failed, continuing without audit: %v", auditErr)
		} else {
			audit = session
			log.Infof("created podexec terminal audit session, sessionID=%s project=%s env=%s pod=%s container=%s", audit.SessionID, productName, envName, podName, containerName)
			pty.attachAudit(audit.SessionID, audit)
		}
	}

	log.Infof("start pod exec stream, sessionID=%s clusterID=%s namespace=%s pod=%s container=%s", pty.sessionID, clusterID, namespace, podName, containerName)
	err = ExecPod(clusterID, []string{"/bin/sh"}, pty, namespace, podName, containerName)
	log.Infof("finish pod exec stream, sessionID=%s err=%v", pty.sessionID, err)
	if err == nil || isExpectedTerminalClose(err) {
		return
	}
	finalStatus = commonmodels.TerminalSessionStatusFailed
	msg := fmt.Sprintf("Exec to pod error! err: %v", err)
	log.Errorf(msg)
	_, _ = pty.Write([]byte(msg))

	ctx.RespErr = e.ErrInternalError.AddDesc(fmt.Sprintf("Exec to pod error! err: %v", err))
	return
}

func resolvePodServiceName(kubeCli kubernetes.Interface, productInfo *commonmodels.Product, pod *corev1.Pod) string {
	if serviceName := strings.TrimSpace(pod.Labels[setting.ServiceLabel]); serviceName != "" {
		return serviceName
	}

	kind, name := podWorkloadReference(kubeCli, pod)
	if kind == "" || name == "" {
		return ""
	}
	for _, service := range productInfo.GetSvcList() {
		if service == nil {
			continue
		}
		for _, resource := range service.Resources {
			if resource != nil && resource.Kind == kind && resource.Name == name {
				return service.ServiceName
			}
		}
	}
	return ""
}

func podWorkloadReference(kubeCli kubernetes.Interface, pod *corev1.Pod) (string, string) {
	var owner *metav1.OwnerReference
	for i := range pod.OwnerReferences {
		if ownerRef := &pod.OwnerReferences[i]; ownerRef.Controller != nil && *ownerRef.Controller {
			owner = ownerRef
			break
		}
	}
	if owner == nil && len(pod.OwnerReferences) > 0 {
		owner = &pod.OwnerReferences[0]
	}
	if owner == nil {
		return "", ""
	}
	if owner.Kind != "ReplicaSet" {
		return owner.Kind, owner.Name
	}

	replicaset, err := kubeCli.AppsV1().ReplicaSets(pod.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
	if err != nil {
		return owner.Kind, owner.Name
	}
	for i := range replicaset.OwnerReferences {
		if ownerRef := &replicaset.OwnerReferences[i]; ownerRef.Controller != nil && *ownerRef.Controller {
			return ownerRef.Kind, ownerRef.Name
		}
	}
	return owner.Kind, owner.Name
}

func readTerminalSizeFromQuery(c *gin.Context) (int, int) {
	cols, _ := strconv.Atoi(c.Query("cols"))
	rows, _ := strconv.Atoi(c.Query("rows"))
	return cols, rows
}

func isExpectedTerminalClose(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	return closeErr.Code == websocket.CloseNormalClosure || closeErr.Code == websocket.CloseGoingAway
}

func collectContainerSecretValues(ctx context.Context, kubeCli kubernetes.Interface, pod *corev1.Pod, namespace, containerName string) ([]string, error) {
	var envFrom []corev1.EnvFromSource
	var envs []corev1.EnvVar
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			envFrom, envs = pod.Spec.Containers[i].EnvFrom, pod.Spec.Containers[i].Env
			break
		}
	}
	if envFrom == nil && envs == nil {
		for i := range pod.Spec.EphemeralContainers {
			if pod.Spec.EphemeralContainers[i].Name == containerName {
				envFrom, envs = pod.Spec.EphemeralContainers[i].EnvFrom, pod.Spec.EphemeralContainers[i].Env
				break
			}
		}
	}

	type secretRequest struct {
		optional bool
		all      bool
		keys     map[string]struct{}
	}
	requests := make(map[string]*secretRequest)
	for _, source := range envFrom {
		if source.SecretRef == nil || source.SecretRef.Name == "" {
			continue
		}
		name := source.SecretRef.Name
		optional := source.SecretRef.Optional != nil && *source.SecretRef.Optional
		request, ok := requests[name]
		if !ok {
			request = &secretRequest{optional: optional}
			requests[name] = request
		} else if !optional {
			request.optional = false
		}
		request.all = true
	}
	for _, envVar := range envs {
		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := envVar.ValueFrom.SecretKeyRef
		if ref.Name == "" || ref.Key == "" {
			continue
		}
		optional := ref.Optional != nil && *ref.Optional
		request, ok := requests[ref.Name]
		if !ok {
			request = &secretRequest{optional: optional, keys: make(map[string]struct{})}
			requests[ref.Name] = request
		} else if !optional {
			request.optional = false
		}
		if request.keys == nil {
			request.keys = make(map[string]struct{})
		}
		request.keys[ref.Key] = struct{}{}
	}

	secretValues := make([]string, 0)
	for name, request := range requests {
		secret, err := kubeCli.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if request.optional && apierrors.IsNotFound(err) {
				continue
			}
			return nil, fmt.Errorf("get secret %s: %w", name, err)
		}
		if request.all {
			for _, value := range secret.Data {
				if len(value) > 0 {
					secretValues = append(secretValues, string(value))
				}
			}
			continue
		}
		for key := range request.keys {
			if value := secret.Data[key]; len(value) > 0 {
				secretValues = append(secretValues, string(value))
			}
		}
	}
	return secretValues, nil
}
