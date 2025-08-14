/*
Copyright 2022 The KodeRover Authors.

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

package instantmessage

import (
	"fmt"
	"strings"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/adaptivecard"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
)

func (w *Service) sendMSTeamsMessage(uri, title, content, actionURL string, atEmails []string, taskStatus config.Status) error {
	card := adaptivecard.NewCard()

	_, content, found := strings.Cut(content, "\n")
	if !found {
		return fmt.Errorf("failed to cut content")
	}

	titleElement := adaptivecard.NewTitleTextBlock(title, false)

	if taskStatus == config.StatusPassed || taskStatus == config.StatusCreated {
		titleElement.Color = adaptivecard.ColorGood
	} else if taskStatus == config.StatusFailed {
		titleElement.Color = adaptivecard.ColorAttention
	} else {
		titleElement.Color = adaptivecard.ColorWarning
	}

	err := card.AddElement(false, titleElement)
	if err != nil {
		return fmt.Errorf("failed to add title element to card: %v", err)
	}

	bodyElement := adaptivecard.NewTextBlock(content, true)
	err = card.AddElement(false, bodyElement)
	if err != nil {
		return fmt.Errorf("failed to add body element to card: %v", err)
	}

	userMentions := make([]adaptivecard.Mention, 0, len(atEmails))
	for _, email := range atEmails {
		userMention, err := adaptivecard.NewMention(email, email)
		if err != nil {
			return fmt.Errorf("failed to create mention: %v", err)
		}
		userMentions = append(userMentions, userMention)
	}

	if len(userMentions) > 0 {
		if err := card.AddMention(false, userMentions...); err != nil {
			return fmt.Errorf("failed to add mention to card: %v", err)
		}
	}

	actionURLDesc := "点击查看更多信息"
	urlAction, err := adaptivecard.NewActionOpenURL(actionURL, actionURLDesc)
	if err != nil {
		return fmt.Errorf("failed to create action open url: %v", err)
	}

	err = card.AddAction(false, urlAction)
	if err != nil {
		return fmt.Errorf("failed to add action to card: %v", err)
	}

	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return fmt.Errorf("failed to create message from card: %v", err)
	}

	mstClient := goteamsnotify.NewTeamsClient()
	mstClient = mstClient.SkipWebhookURLValidationOnSend(true)
	err = mstClient.Send(uri, msg)
	if err != nil {
		return fmt.Errorf("failed to send message to MSTeams: %v", err)
	}

	return nil
}
