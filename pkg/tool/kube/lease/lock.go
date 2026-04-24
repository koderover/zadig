/*
Copyright 2026 The KodeRover Authors.

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

package lease

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	namespaceFilePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultRetryDelay = 500 * time.Millisecond
	defaultDuration   = 30 * time.Second
)

type Lock struct {
	client       kubernetes.Interface
	namespace    string
	name         string
	holder       string
	duration     time.Duration
	acquiredOnce bool
}

func NewLock(name string, duration time.Duration) (*Lock, error) {
	if name == "" {
		return nil, fmt.Errorf("lease name is empty")
	}
	if duration <= 0 {
		duration = defaultDuration
	}

	namespace, err := currentNamespace()
	if err != nil {
		return nil, err
	}
	holder, err := currentHolderIdentity()
	if err != nil {
		return nil, err
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("get in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build kubernetes client: %w", err)
	}

	return &Lock{
		client:    clientset,
		namespace: namespace,
		name:      name,
		holder:    holder,
		duration:  duration,
	}, nil
}

func (l *Lock) Acquire(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(defaultRetryDelay)
	defer ticker.Stop()

	for {
		acquired, err := l.tryAcquire(ctx)
		if err != nil {
			return err
		}
		if acquired {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (l *Lock) Release(ctx context.Context) error {
	if !l.acquiredOnce {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	current, err := l.client.CoordinationV1().Leases(l.namespace).Get(ctx, l.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if current.Spec.HolderIdentity == nil || *current.Spec.HolderIdentity != l.holder {
		return nil
	}

	current = current.DeepCopy()
	current.Spec.HolderIdentity = nil
	current.Spec.AcquireTime = nil
	current.Spec.RenewTime = nil
	current.Spec.LeaseDurationSeconds = nil
	_, err = l.client.CoordinationV1().Leases(l.namespace).Update(ctx, current, metav1.UpdateOptions{})
	if apierrors.IsConflict(err) {
		return nil
	}
	return err
}

func (l *Lock) tryAcquire(ctx context.Context) (bool, error) {
	now := metav1.NewMicroTime(time.Now())
	durationSeconds := int32(l.duration / time.Second)

	current, err := l.client.CoordinationV1().Leases(l.namespace).Get(ctx, l.name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}

		lease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      l.name,
				Namespace: l.namespace,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       pointerTo(l.holder),
				LeaseDurationSeconds: pointerTo(durationSeconds),
				AcquireTime:          &now,
				RenewTime:            &now,
			},
		}
		_, err = l.client.CoordinationV1().Leases(l.namespace).Create(ctx, lease, metav1.CreateOptions{})
		if err == nil {
			l.acquiredOnce = true
			return true, nil
		}
		if apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err) {
			return false, nil
		}
		return false, err
	}

	if !leaseAvailable(current, time.Now()) && (current.Spec.HolderIdentity == nil || *current.Spec.HolderIdentity != l.holder) {
		return false, nil
	}

	updated := current.DeepCopy()
	if updated.Spec.HolderIdentity == nil || *updated.Spec.HolderIdentity != l.holder {
		transitions := int32(0)
		if updated.Spec.LeaseTransitions != nil {
			transitions = *updated.Spec.LeaseTransitions
		}
		updated.Spec.LeaseTransitions = pointerTo(transitions + 1)
		updated.Spec.AcquireTime = &now
	}
	updated.Spec.HolderIdentity = pointerTo(l.holder)
	updated.Spec.LeaseDurationSeconds = pointerTo(durationSeconds)
	updated.Spec.RenewTime = &now
	_, err = l.client.CoordinationV1().Leases(l.namespace).Update(ctx, updated, metav1.UpdateOptions{})
	if err == nil {
		l.acquiredOnce = true
		return true, nil
	}
	if apierrors.IsConflict(err) {
		return false, nil
	}
	return false, err
}

func leaseAvailable(lease *coordinationv1.Lease, now time.Time) bool {
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == "" {
		return true
	}
	if lease.Spec.RenewTime == nil || lease.Spec.LeaseDurationSeconds == nil {
		return true
	}
	expireAt := lease.Spec.RenewTime.Time.Add(time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second)
	return now.After(expireAt)
}

func currentNamespace() (string, error) {
	data, err := os.ReadFile(namespaceFilePath)
	if err != nil {
		return "", fmt.Errorf("read namespace file: %w", err)
	}
	namespace := strings.TrimSpace(string(data))
	if namespace == "" {
		return "", fmt.Errorf("namespace is empty")
	}
	return namespace, nil
}

func currentHolderIdentity() (string, error) {
	holder, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("get hostname: %w", err)
	}
	holder = strings.TrimSpace(holder)
	if holder == "" {
		return "", fmt.Errorf("hostname is empty")
	}
	log.Debugf("shared cache lease holder identity: %s", holder)
	return holder, nil
}

func pointerTo[T any](v T) *T {
	return &v
}
