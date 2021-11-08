package mail

import (
	"bytes"
	"html/template"

	"gopkg.in/gomail.v2"
)

type EmailParams struct {
	From     string
	To       string
	Subject  string
	Body     string
	Host     string
	Port     int
	UserName string
	Password string
}

func SendEmail(param *EmailParams) error {
	m := gomail.NewMessage()
	m.SetHeader("From", param.From)
	m.SetHeader("To", param.To)
	m.SetHeader("Subject", param.Subject)
	m.SetBody("text/html", param.Body)

	d := gomail.NewDialer(param.Host, param.Port, param.UserName, param.Password)

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
