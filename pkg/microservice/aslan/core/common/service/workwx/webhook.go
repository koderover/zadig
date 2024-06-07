/*
Copyright 2024 The KodeRover Authors.

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

package workwx

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
	"github.com/pkg/errors"
)

func workWXApprovalCacheKey(instanceID string) string {
	return fmt.Sprint("workwx_approval_cache_", instanceID)
}

func RemoveWorkWXApprovalManager(instanceID string) {
	cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Delete(workWXApprovalCacheKey(instanceID))
}

func EventHandler(id string, body []byte, signature, ts, nonce string) (interface{}, error) {
	log := log.SugaredLogger().With("func", "WorkWXEventHandler").With("ID", id)

	log.Info("New workwx event received")
	info, err := mongodb.NewIMAppColl().GetByID(context.Background(), id)
	if err != nil {
		log.Errorf("get workwx info error: %v", err)
		return nil, errors.Wrap(err, "get workwx info error")
	}

	msg := new(workwx.EncryptedWebhookMessage)

	err = xml.Unmarshal(body, msg)
	if err != nil {
		log.Errorf("failed to decode workwx webhook message, error: %s", err)
		return nil, fmt.Errorf("failed to decode workwx webhook message, error: %s", err)
	}

	signatureOK := workwx.CallbackValidate(signature, info.WorkWXToken, ts, nonce, msg.Encrypt)
	if !signatureOK {
		log.Errorf("cannon verify the signature: data corrupted")
		return "", fmt.Errorf("cannon verify the signature: data corrupted")
	}

	plaintext, _, err := workwx.DecodeEncryptedMessage(info.WorkWXAESKey, msg.Encrypt)
	if err != nil {
		log.Errorf("failed to decode workwx webhook message, error: %s", err)
		return nil, fmt.Errorf("failed to decode workwx webhook message, error: %s", err)
	}

	decodedMessage := new(workwx.DecodedWebhookMessage)
	err = xml.Unmarshal(plaintext, decodedMessage)
	if err != nil {
		log.Errorf("failed to decode workwx webhook message, error: %s", err)
		return nil, fmt.Errorf("failed to decode workwx webhook message, error: %s", err)
	}

	switch decodedMessage.Event {
	case workwx.EventTypeApprovalChange:
		// do something
		if decodedMessage.ApprovalInfo == nil {
			return "", fmt.Errorf("empty ApprovalInfo field")
		}

		err = setApprovalChange(decodedMessage.ApprovalInfo.ID, decodedMessage.ApprovalInfo)
		if err != nil {
			// do not return error since it is a webhook handler
			log.Errorf("failed to save the approval status change event into the redis, error: %s")
		}
	default:
		return "", fmt.Errorf("unsupported event type for now")
	}
	return string(plaintext), err
}

func setApprovalChange(instanceID string, change *workwx.ApprovalWebhookMessage) error {
	bytes, _ := json.Marshal(change)
	return cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Write(workWXApprovalCacheKey(instanceID), string(bytes), 0)
}

func GetWorkWXApprovalEvent(instanceID string) (*workwx.ApprovalWebhookMessage, error) {
	resp, err := cache.NewRedisCache(config.RedisCommonCacheTokenDB()).GetString(workWXApprovalCacheKey(instanceID))
	if err != nil {
		return nil, err
	}

	changeMessage := new(workwx.ApprovalWebhookMessage)
	err = json.Unmarshal([]byte(resp), changeMessage)
	if err != nil {
		return nil, err
	}
	return changeMessage, nil
}
