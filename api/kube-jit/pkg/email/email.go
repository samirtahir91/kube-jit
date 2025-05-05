package email

import (
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

func SendMail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", os.Getenv("SMTP_FROM"))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(
		os.Getenv("SMTP_HOST"),
		mustAtoi(os.Getenv("SMTP_PORT")),
		os.Getenv("SMTP_USER"),
		os.Getenv("SMTP_PASS"),
	)
	return d.DialAndSend(m)
}

func mustAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
