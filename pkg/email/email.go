package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/caspianex/exchange-backend/internal/domain"
)

type EmailService struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewEmailService(host string, port int, username, password, from string) *EmailService {
	return &EmailService{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

func (e *EmailService) SendEmail(to, subject, body string) error {
	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	msg := []byte(fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"%s\r\n", e.from, to, subject, body))

	addr := fmt.Sprintf("%s:%d", e.host, e.port)
	return smtp.SendMail(addr, auth, e.from, []string{to}, msg)
}

func (e *EmailService) SendWelcomeEmail(to, firstName string) error {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 10px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to CaspianEx</h1>
        </div>
        <div class="content">
            <h2>Hello {{.FirstName}},</h2>
            <p>Thank you for registering with CaspianEx. We're excited to have you on board!</p>
            <p>You can now start exchanging currencies on our platform.</p>
            <p>Best regards,<br>The CaspianEx Team</p>
        </div>
    </div>
</body>
</html>
`
	t, err := template.New("welcome").Parse(tmpl)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	if err := t.Execute(&body, struct{ FirstName string }{FirstName: firstName}); err != nil {
		return err
	}

	return e.SendEmail(to, "Welcome to CaspianEx", body.String())
}

func (e *EmailService) SendOrderCreatedEmail(to, firstName string, exchange *domain.CurrencyExchange) error {
	subject := fmt.Sprintf("Exchange Completed - %s", exchange.UID)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4CAF50; color: white; padding: 10px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .info-box { background: white; padding: 15px; margin: 10px 0; border-left: 4px solid #4CAF50; }
        .uid { font-size: 20px; font-weight: bold; color: #4CAF50; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Exchange Completed</h1>
        </div>
        <div class="content">
            <h2>Hello %s,</h2>
            <p>Your currency exchange has been completed successfully!</p>
            <div class="info-box">
                <p><strong>Exchange UID:</strong> <span class="uid">%s</span></p>
                <p><strong>Amount Exchanged:</strong> %.8f</p>
                <p><strong>Amount Received:</strong> %.8f (after %.2f%% fee)</p>
                <p><strong>Exchange Rate:</strong> %.8f</p>
            </div>
            <p>The funds have been credited to your wallet.</p>
            <p>Thank you for using CaspianEx!</p>
            <p>Best regards,<br>The CaspianEx Team</p>
        </div>
    </div>
</body>
</html>
	`, firstName, exchange.UID, exchange.FromAmount, exchange.ToAmountWithFee, exchange.Fee, exchange.ExchangeRate)

	return e.SendEmail(to, subject, body)
}
