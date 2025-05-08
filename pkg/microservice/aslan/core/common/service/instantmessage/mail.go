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
	_ "embed"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	"github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/mail"
)

func (w *Service) sendMailMessage(title, content string, users []*models.User) error {
	if len(users) == 0 {
		return nil
	}

	email, err := systemconfig.New().GetEmailHost()
	if err != nil {
		log.Errorf("sendMailMessage GetEmailHost error, error msg:%s", err)
	}

	emailSvc, err := systemconfig.New().GetEmailService()
	if err != nil {
		log.Errorf("sendMailMessage GetEmailService error, error msg:%s", err)
	}

	users, userMap := util.GeneFlatUsers(users)
	for _, u := range users {
		info, ok := userMap[u.UserID]
		if !ok {
			info, err = user.New().GetUserByID(u.UserID)
			if err != nil {
				log.Warnf("sendMailMessage GetUserByUid error, error msg:%s", err)
				continue
			}
		}

		if info.Email == "" {
			log.Warnf("sendMailMessage user %s email is empty", info.Name)
			continue
		}
		err = mail.SendEmail(&mail.EmailParams{
			From:     emailSvc.Address,
			To:       info.Email,
			Subject:  title,
			Host:     email.Name,
			UserName: email.UserName,
			Password: email.Password,
			Port:     email.Port,
			Body:     content,
		})
		if err != nil {
			log.Errorf("sendMailMessage SendEmail error, error msg:%s", err)
			continue
		}
	}

	return err
}
