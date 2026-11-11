/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package apollo

import (
	"context"
	"errors"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	apollotool "github.com/koderover/zadig/v2/pkg/tool/apollo"
)

// MergeNamespace reads the current configuration and prepares validated
// updates without writing to Apollo or changing the saved plan.
func MergeNamespace(ctx context.Context, client *apollotool.Client, namespace *models.ApolloNamespace) (*models.ApolloNamespace, error) {
	if namespace == nil {
		return nil, errors.New("Apollo namespace is nil")
	}
	current, err := client.GetNamespace(namespace.AppID, namespace.Env, namespace.ClusterID, namespace.Namespace)
	if err != nil {
		return nil, fmt.Errorf("get current namespace failed: %w", err)
	}
	if current == nil {
		return nil, errors.New("current namespace not found")
	}
	format := current.Format
	if format == "" {
		format = namespace.Type
	}
	original := make(map[string]string, len(namespace.OriginalConfig))
	for _, kv := range namespace.OriginalConfig {
		if kv == nil {
			return nil, errors.New("original config item is nil; refresh the original configuration and edit again")
		}
		key := apollotool.NormalizeItemKey(format, kv.Key)
		if _, exists := original[key]; exists {
			return nil, fmt.Errorf("duplicate original config key %s", key)
		}
		original[key] = kv.Val
	}
	latest := make(map[string]string, len(current.Items))
	for _, item := range current.Items {
		if item == nil {
			return nil, errors.New("current config item is nil")
		}
		latest[apollotool.NormalizeItemKey(format, item.Key)] = item.Value
	}

	// Merge and validate every item before writing anything to Apollo.
	updates := make([]*models.ApolloKV, 0, len(namespace.KeyValList))
	seen := make(map[string]bool, len(namespace.KeyValList))
	for _, kv := range namespace.KeyValList {
		if kv == nil {
			return nil, errors.New("update item failed: config item is nil")
		}
		key := apollotool.NormalizeItemKey(format, kv.Key)
		if seen[key] {
			return nil, fmt.Errorf("duplicate config key %s", key)
		}
		seen[key] = true
		base, hadOriginal := original[key]
		value, exists := latest[key]
		if hadOriginal && kv.Val == base {
			continue // Preserve concurrent changes, including deleted items.
		}
		if len(original) == 0 && value != "" && kv.Val != value {
			return nil, fmt.Errorf("config key %s is missing its original configuration; refresh the original configuration and edit again", key)
		}
		if hadOriginal && !exists {
			return nil, fmt.Errorf("config key %s was deleted in Apollo; refresh the original configuration and edit again", key)
		}
		merged, err := apollotool.MergeConfig(ctx, format, base, kv.Val, value)
		if err != nil {
			return nil, fmt.Errorf("merge config key %s failed: %w", key, err)
		}
		if !exists || merged != value {
			updates = append(updates, &models.ApolloKV{Key: key, Val: merged})
		}
	}
	mergedNamespace := *namespace
	mergedNamespace.Type = format
	mergedNamespace.KeyValList = updates
	return &mergedNamespace, nil
}
