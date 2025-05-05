package email

import (
	"kube-jit/pkg/utils"
	"strconv"

	"gopkg.in/gomail.v2"
)

func SendMail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", utils.MustGetEnv("SMTP_FROM"))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(
		utils.MustGetEnv("SMTP_HOST"),
		mustAtoi(utils.MustGetEnv("SMTP_PORT")),
		utils.MustGetEnv("SMTP_USER"),
		utils.MustGetEnv("SMTP_PASS"),
	)
	return d.DialAndSend(m)
}

func mustAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
