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

package mail

import (
	"bytes"
	"crypto/tls"
	"html/template"

	"gopkg.in/gomail.v2"
)

type EmailParams struct {
	From          string
	To            string
	Subject       string
	Body          string
	Host          string
	Port          int
	UserName      string
	Password      string
	TlsSkipVerify bool
}

func SendEmail(param *EmailParams) error {
	m := gomail.NewMessage()
	m.SetHeader("From", param.From)
	m.SetHeader("To", param.To)
	m.SetHeader("Subject", param.Subject)
	m.SetBody("text/html", param.Body)

	d := gomail.NewDialer(param.Host, param.Port, param.UserName, param.Password)

	if param.TlsSkipVerify {
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if err := d.DialAndSend(m); err != nil {
		return err
	}
	return nil
}

type GenerateUrl struct {
	Url string
}

func RenderEmailTemplate(url string, html string) (content string, err error) {
	buf := new(bytes.Buffer)
	t := template.Must(template.New("email").Funcs(template.FuncMap{}).Parse(html))

	if err = t.Execute(buf, &GenerateUrl{
		Url: url,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
