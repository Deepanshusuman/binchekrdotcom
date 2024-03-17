package api

import (
	"binchecker/credential"
	"strings"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

func SendEmail(to string, subject string, body string) error {
	toes := []string{to}
	msg := strings.NewReader("To:" + to + "\r\nSubject: " + subject + "\r\n\r\n" + body + "\r\n")
	return smtp.SendMail(credential.SMTP_ADDRESS, sasl.NewPlainClient("", credential.SMTP_USERNAME, credential.SMTP_PASSWORD), "maik@site.com", toes, msg)
}
