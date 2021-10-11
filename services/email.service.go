package services

import (
	"bytes"
	"encoding/base64"
	"net/smtp"
	"os"
	"strings"
	"text/template"
	"time"

	"devinterface.com/goaas-api-starter/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

// EmailService struct
type EmailService struct{ BaseService }

// SendActivationEmail function
func (emailService *EmailService) SendActivationEmail(q bson.M) (success bool, err error) {
	user := &models.User{}
	var userService = UserService{}
	q["active"] = false
	err = userService.getCollection().First(q, user)
	if err != nil {
		return false, err
	}
	confirmationTokenUUID, _ := uuid.NewRandom()
	user.ConfirmationToken = confirmationTokenUUID.String()
	err = userService.getCollection().Update(user)
	frontendActivationURL := os.Getenv("FRONTEND_ACTIVATION_URL")
	t := template.Must(template.New("activationLink.email.tmpl").ParseFiles("./emails/activationLink.email.tmpl"))
	data := struct {
		Email                 string
		FrontendActivationURL string
		ConfirmationToken     string
	}{
		Email:                 user.Email,
		FrontendActivationURL: frontendActivationURL,
		ConfirmationToken:     user.ConfirmationToken,
	}
	var tpl bytes.Buffer
	if err = t.Execute(&tpl, data); err != nil {
		return false, err
	}
	result := tpl.String()
	err = SendMail(os.Getenv("MAILER"), os.Getenv("DEFAULT_EMAIL_FROM"), "[MiniMarket24] Istruzioni per attivare l'account", result, []string{user.Email})
	return true, err
}

// SendForgotPasswordEmail function
func (emailService *EmailService) SendForgotPasswordEmail(q bson.M) (success bool, err error) {
	user := &models.User{}
	var userService = UserService{}
	err = userService.getCollection().First(q, user)
	if err != nil {
		return false, err
	}
	generatedUUID, _ := uuid.NewRandom()
	user.PasswordResetToken = generatedUUID.String()
	user.PasswordResetExpires = time.Now().Add(time.Hour * 3)
	err = userService.getCollection().Update(user)
	frontendForgotURL := os.Getenv("FRONTEND_FORGOT_URL")

	t := template.Must(template.New("forgotPassword.email.tmpl").ParseFiles("./emails/forgotPassword.email.tmpl"))
	data := struct {
		Email              string
		FrontendForgotURL  string
		PasswordResetToken string
	}{
		Email:              user.Email,
		FrontendForgotURL:  frontendForgotURL,
		PasswordResetToken: user.PasswordResetToken,
	}
	var tpl bytes.Buffer
	if err = t.Execute(&tpl, data); err != nil {
		return false, err
	}
	result := tpl.String()

	err = SendMail(os.Getenv("MAILER"), os.Getenv("DEFAULT_EMAIL_FROM"), "[MiniMarket24] Istruzioni di ripristino password", result, []string{user.Email})
	return true, err
}

// SendActiveEmail function
func (emailService *EmailService) SendActiveEmail(q bson.M) (success bool, err error) {
	user := &models.User{}
	var userService = UserService{}
	err = userService.getCollection().First(q, user)
	if err != nil {
		return false, err
	}
	frontendLoginURL := os.Getenv("FRONTEND_LOGIN_URL")

	t := template.Must(template.New("activate.email.tmpl").ParseFiles("./emails/activate.email.tmpl"))
	data := struct {
		Email            string
		FrontendLoginURL string
	}{
		Email:            user.Email,
		FrontendLoginURL: frontendLoginURL,
	}
	var tpl bytes.Buffer
	if err = t.Execute(&tpl, data); err != nil {
		return false, err
	}
	result := tpl.String()
	err = SendMail(os.Getenv("MAILER"), os.Getenv("DEFAULT_EMAIL_FROM"), "[MiniMarket24] Attivazione completata", result, []string{user.Email})
	return true, err
}

// SendStripeNotificationEmail function
func (emailService *EmailService) SendStripeNotificationEmail(q bson.M, subject string, mainMessage string) (success bool, err error) {
	users := []models.User{}
	var userService = UserService{}
	err = userService.getCollection().SimpleFind(&users, q)
	if err != nil {
		return false, err
	}
	frontendLoginURL := os.Getenv("FRONTEND_LOGIN_URL")
	for _, user := range users {
		go func(u models.User) {
			t := template.Must(template.New("stripe.notification.tmpl").ParseFiles("./emails/stripe.notification.tmpl"))
			data := struct {
				Subject          string
				Email            string
				Message          string
				FrontendLoginURL string
			}{
				Subject:          subject,
				Email:            u.Email,
				Message:          mainMessage,
				FrontendLoginURL: frontendLoginURL,
			}
			var tpl bytes.Buffer
			if err := t.Execute(&tpl, data); err != nil {
				return
			}

			result := tpl.String()

			err := SendMail(os.Getenv("MAILER"), os.Getenv("DEFAULT_EMAIL_FROM"), subject, result, []string{u.Email})
			if err != nil {
			}
		}(user)
	}
	return true, err
}

// SendMail function
func SendMail(addr, from, subject, body string, to []string) error {
	r := strings.NewReplacer("\r\n", "", "\r", "", "\n", "", "%0a", "", "%0d", "")

	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	if err = c.Mail(r.Replace(from)); err != nil {
		return err
	}
	for i := range to {
		to[i] = r.Replace(to[i])
		if err = c.Rcpt(to[i]); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	msg := "To: " + strings.Join(to, ",") + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	_, err = w.Write([]byte(msg))
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}
