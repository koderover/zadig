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

package meego

type GeneralWebhookRequest struct {
	Header  *WebhookHeader         `json:"header"`
	Payload *GeneralWebhookPayload `json:"payload"`
}

type WebhookHeader struct {
	Operator  string `json:"operator"`
	EventType string `json:"event_type"`
	Token     string `json:"token"`
	UUID      string `json:"uuid"`
}

type GeneralWebhookPayload struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	ProjectKey        string `json:"project_key"`
	ProjectSimpleName string `json:"project_simple_name"`
	WorkItemTypeKey   string `json:"work_item_type_key"`
}
