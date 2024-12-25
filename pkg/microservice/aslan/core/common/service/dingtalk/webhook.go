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

package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/tidwall/gjson"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	EventTaskChange     = "bpms_task_change"
	EventInstanceChange = "bpms_instance_change"
)

type UserApprovalResult struct {
	Result        string
	OperationTime int64
	Remark        string
}

func dingtalkApprovalCacheKey(instanceID string) string {
	return fmt.Sprint("dingtalk_approval_cache_", instanceID)
}

func RemoveDingTalkApprovalManager(instanceID string) {
	cache.NewRedisCache(config.RedisCommonCacheTokenDB()).Delete(dingtalkApprovalCacheKey(instanceID))
}

func GetAllUserApprovalResults(instanceID string) map[string]*UserApprovalResult {

	resp, err := cache.NewRedisCache(config.RedisCommonCacheTokenDB()).HGetAllString(dingtalkApprovalCacheKey(instanceID))
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Errorf("get dingtalk approval cache error: %v", err)
		return nil
	}
	re := make(map[string]*UserApprovalResult)
	for k, v := range resp {
		var info UserApprovalResult
		if err := json.Unmarshal([]byte(v), &info); err != nil {
			log.Errorf("unmarshal dingtalk approval cache error: %v", err)
			continue
		}
		re[k] = &info
	}

	return re
}

func SetUserApprovalResult(instanceID, userID, result, remark string, time int64) {

	bytes, _ := json.Marshal(&UserApprovalResult{
		Result:        result,
		OperationTime: time / 1000,
		Remark:        remark,
	})
	cache.NewRedisCache(config.RedisCommonCacheTokenDB()).HWrite(dingtalkApprovalCacheKey(instanceID), userID, string(bytes), 0)
}

type EventInstanceChangeData struct {
	EventType         string `json:"EventType"`
	ProcessInstanceID string `json:"processInstanceId"`
	FinishTime        int64  `json:"finishTime"`
	CorpID            string `json:"corpId"`
	Title             string `json:"title"`
	Type              string `json:"type"`
	URL               string `json:"url"`
	Result            string `json:"result"`
	CreateTime        int64  `json:"createTime"`
	StaffID           string `json:"staffId"`
	ProcessCode       string `json:"processCode"`
}

type EventTaskChangeData struct {
	EventType         string `json:"EventType"`
	ProcessInstanceID string `json:"processInstanceId"`
	FinishTime        int64  `json:"finishTime"`
	CorpID            string `json:"corpId"`
	Title             string `json:"title"`
	Type              string `json:"type"`
	Result            string `json:"result"`
	Remark            string `json:"remark"`
	CreateTime        int64  `json:"createTime"`
	StaffID           string `json:"staffId"`
	ProcessCode       string `json:"processCode"`
}

type EventResponse struct {
	MsgSignature string `json:"msg_signature"`
	TimeStamp    string `json:"timeStamp"`
	Nonce        string `json:"nonce"`
	Encrypt      string `json:"encrypt"`
}

func EventHandler(appKey string, body []byte, signature, ts, nonce string) (*EventResponse, error) {
	log := log.SugaredLogger().With("func", "DingTalkEventHandler").With("appKey", appKey)

	log.Info("New dingtalk event received")
	info, err := mongodb.NewIMAppColl().GetDingTalkByAppKey(context.Background(), appKey)
	if err != nil {
		log.Errorf("get dingtalk info error: %v", err)
		return nil, errors.Wrap(err, "get dingtalk info error")
	}

	d, err := NewDingTalkCrypto(info.DingTalkToken, info.DingTalkAesKey, info.DingTalkAppKey)
	if err != nil {
		log.Errorf("new dingtalk crypto error: %v", err)
		return nil, errors.Wrap(err, "new dingtalk crypto error")
	}

	data, err := d.GetDecryptMsg(signature, ts, nonce, gjson.Get(string(body), "encrypt").String())
	if err != nil {
		log.Errorf("get decrypt msg error: %v", err)
		return nil, errors.Wrap(err, "get decrypt msg error")
	}
	eventType := gjson.Get(data, "EventType").String()
	log.Infof("receive dingtalk event type: %s instanceID: %s", eventType,
		gjson.Get(data, "processInstanceId").String())

	switch eventType {
	case EventTaskChange:
		var event EventTaskChangeData
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			log.Errorf("unmarshal event data error: %v", err)
			return nil, errors.Wrap(err, "unmarshal event data error")
		}
		if event.Type != "finish" {
			break
		}
		SetUserApprovalResult(event.ProcessInstanceID, event.StaffID, event.Result, event.Remark, event.FinishTime)
		log.Infof("dingtalk event type: %s instanceID: %s userID: %s result: %s remark: %s",
			eventType, event.ProcessInstanceID, event.StaffID, event.Result, event.Remark)
	}

	msg, err := d.GetEncryptMsg("success")
	if err != nil {
		log.Errorf("get encrypt msg error: %v", err)
		return nil, errors.Wrap(err, "get encrypt msg error")
	}
	return &EventResponse{
		MsgSignature: msg["msg_signature"],
		TimeStamp:    msg["timeStamp"],
		Nonce:        msg["nonce"],
		Encrypt:      msg["encrypt"],
	}, nil
}
