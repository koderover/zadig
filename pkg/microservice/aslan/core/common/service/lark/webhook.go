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

package lark

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/tidwall/gjson"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type CallbackData struct {
	UUID  string          `json:"uuid"`
	Event json.RawMessage `json:"event"`
	Token string          `json:"token"`
	Ts    string          `json:"ts"`
	Type  string          `json:"type"`
}

type ApprovalInstanceEvent struct {
	AppID               string `json:"app_id"`
	ApprovalCode        string `json:"approval_code"`
	InstanceCode        string `json:"instance_code"`
	InstanceOperateTime string `json:"instance_operate_time"`
	OperateTime         string `json:"operate_time"`
	Status              string `json:"status"`
	TenantKey           string `json:"tenant_key"`
	Type                string `json:"type"`
	UUID                string `json:"uuid"`
}

type ApprovalTaskEvent struct {
	AppID        string `json:"app_id"`
	OpenID       string `json:"open_id"`
	TenantKey    string `json:"tenant_key"`
	Type         string `json:"type"`
	ApprovalCode string `json:"approval_code"`
	InstanceCode string `json:"instance_code"`
	TaskID       string `json:"task_id"`
	UserID       string `json:"user_id"`
	Status       string `json:"status"`
	OperateTime  string `json:"operate_time"`
	CustomKey    string `json:"custom_key"`
	DefKey       string `json:"def_key"`
	Extra        string `json:"extra"`
}

type EventHandlerResponse struct {
	Challenge string `json:"challenge"`
}

func EventHandler(appID, sign, ts, nonce, body string) (*EventHandlerResponse, error) {
	log.Infof("LarkEventHandler: new request approval received")
	larkAppInfo, err := mongodb.NewIMAppColl().GetLarkByAppID(context.Background(), appID)
	if err != nil {
		log.Errorf("get lark app info failed: %v", err)
		return nil, errors.Wrap(err, "get approval by appID")
	}

	larkAppInfoID := larkAppInfo.ID.Hex()
	log.Infof("LarkEventHandler: new request approval ID %s", larkAppInfoID)

	key := larkAppInfo.EncryptKey
	raw := body
	if key != "" {
		raw, err = larkDecrypt(gjson.Get(body, "encrypt").String(), key)
		if err != nil {
			log.Errorf("decrypt body failed: %v", err)
			return nil, errors.Wrap(err, "decrypt body")
		}
	}

	// handle lark open platform webhook URL check request, which only need reply the challenge field.
	if sign == "" {
		log.Infof("LarkEventHandler: challenge request received, challenge: %s", gjson.Get(raw, "challenge").String())
		return &EventHandlerResponse{Challenge: gjson.Get(raw, "challenge").String()}, nil
	}

	if sign != larkCalculateSignature(ts, nonce, key, body) {
		log.Errorf("check sign failed")
		return nil, errors.New("check sign failed")
	}

	callback := &CallbackData{}
	err = json.Unmarshal([]byte(raw), callback)
	if err != nil {
		log.Errorf("unmarshal callback data failed: %v", err)
		return nil, errors.Wrap(err, "unmarshal")
	}

	if eventType := gjson.Get(string(callback.Event), "type").String(); eventType != "approval_task" {
		log.Infof("get unknown callback event type %s, ignored", eventType)
		return nil, nil
	}
	log.Debugf("event data: %s", string(callback.Event))
	event := ApprovalTaskEvent{}
	err = json.Unmarshal(callback.Event, &event)
	if err != nil {
		log.Errorf("unmarshal callback event failed: %v", err)
		return nil, errors.Wrap(err, "unmarshal")
	}
	log.Infof("LarkEventHandler: new request approval ID %s, request UUID %s, ts: %s", larkAppInfoID, callback.UUID, callback.Ts)
	manager := GetLarkApprovalInstanceManager(event.InstanceCode)
	if !manager.CheckAndUpdateUUID(callback.UUID) {
		log.Infof("check existed request uuid %s, ignored", callback.UUID)
		return nil, nil
	}
	t, err := strconv.ParseInt(event.OperateTime, 10, 64)
	if err != nil {
		log.Warnf("parse operate time %s failed: %v", event.OperateTime, err)
	}
	UpdateNodeUserApprovalResult(event.InstanceCode, event.DefKey, event.CustomKey, event.OpenID, &UserApprovalResult{
		Result:        event.Status,
		OperationTime: t / 1000,
	})
	log.Infof("update lark app info id: %s, instance code: %s, nodeKey: %s, userID: %s status: %s",
		larkAppInfoID, event.InstanceCode, event.CustomKey, event.OpenID, event.Status)
	return nil, nil
}

func larkDecrypt(encrypt string, key string) (string, error) {
	buf, err := base64.StdEncoding.DecodeString(encrypt)
	if err != nil {
		return "", fmt.Errorf("base64StdEncode Error[%v]", err)
	}
	if len(buf) < aes.BlockSize {
		return "", errors.New("cipher  too short")
	}
	keyBs := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(keyBs[:sha256.Size])
	if err != nil {
		return "", fmt.Errorf("AESNewCipher Error[%v]", err)
	}
	iv := buf[:aes.BlockSize]
	buf = buf[aes.BlockSize:]
	// CBC mode always works in whole blocks.
	if len(buf)%aes.BlockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(buf, buf)
	n := strings.Index(string(buf), "{")
	if n == -1 {
		n = 0
	}
	m := strings.LastIndex(string(buf), "}")
	if m == -1 {
		m = len(buf) - 1
	}
	return string(buf[n : m+1]), nil
}
func larkCalculateSignature(timestamp, nonce, encryptKey, bodystring string) string {
	var b strings.Builder
	b.WriteString(timestamp)
	b.WriteString(nonce)
	b.WriteString(encryptKey)
	b.WriteString(bodystring)
	bs := []byte(b.String())
	h := sha256.New()
	h.Write(bs)
	bs = h.Sum(nil)
	sig := fmt.Sprintf("%x", bs)
	return sig
}
