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

package nsqcli

import (
	"context"
	"fmt"
)

// EnsureNsqdTopic is to create a nsqd topic
func (c *Client) EnsureNsqdTopic(topic string) error {
	var createURL string
	if len(c.nsqdAddr) > 0 {
		createURL = fmt.Sprintf("http://%s/topic/create?topic=%s", c.nsqdAddr, topic)
	} else {
		return fmt.Errorf("no nsqd host found")
	}
	return c.Conn.Call(context.Background(), "", "POST", createURL)
}

// EnsureNsqdTopics is to create a list of topic
func (c *Client) EnsureNsqdTopics(topics []string) error {
	for _, topic := range topics {
		err := c.EnsureNsqdTopic(topic)
		if err != nil {
			return err
		}
	}
	return nil
}
