package main

import (
	"log"
	"net/smtp"
)

// notifyAdmin sends an email notification to the admin about the failure to deliver an event.
func notifyAdmin(subject, body string) {
	from := emailUser
	to := []string{adminEmail}
	msg := "From: " + from + "\n" +
		"To: " + adminEmail + "\n" +
		"Subject: " + subject + "\n\n" +
		body
	err := smtp.SendMail(emailServer, smtp.PlainAuth("", emailUser, emailPassword, "smtp.gmail.com"), from, to, []byte(msg))
	if err != nil {
		log.Printf("Error notifying admin: %v", err)
	}
}
