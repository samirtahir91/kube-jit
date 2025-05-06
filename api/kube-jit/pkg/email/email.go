package email

import (
	"kube-jit/pkg/utils"
	"strconv"

	"gopkg.in/gomail.v2"
)

// SendMail sends an email using the SMTP server configured in the environment variables.
// It takes the recipient's email address, subject, and body of the email as parameters.
// It returns an error if the email could not be sent.
func SendMail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", utils.MustGetEnv("SMTP_FROM"))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(
		utils.MustGetEnv("SMTP_HOST"),
		mustAtoi(utils.MustGetEnv("SMTP_PORT")),
		utils.GetEnv("SMTP_USER", ""),
		utils.GetEnv("SMTP_PASS", ""),
	)
	return d.DialAndSend(m)
}

// mustAtoi is a helper function that converts a string to an integer.
func mustAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
