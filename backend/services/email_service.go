package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type EmailProvider string

const (
	ProviderSendGrid EmailProvider = "sendgrid"
	ProviderSES      EmailProvider = "ses"
)

const (
	defaultFromName           = "AssetForge"
	defaultFromAddress        = "no-reply@assetforge.io"
	defaultVerificationURL    = "https://app.assetforge.io/verify-email"
	emailQueueBuffer          = 100
	emailContentBoundary      = "--assetforge-boundary"
	emailContentTypeHTML      = "text/html; charset=UTF-8"
	emailContentTypePlainText = "text/plain; charset=UTF-8"
)

type EmailService interface {
	SendVerificationEmail(toEmail, toName, verificationToken string) error
	SendKYCStatusUpdate(toEmail, toName, status, reviewNotes string) error
	SendTransactionConfirmation(toEmail, toName, txHash string, amount int64, assetID uint, fromAddress, toAddress string) error
	SendApprovalPendingEmail(toEmail, toName string, requestID uint, expiresAt time.Time) error
}

func (s *emailService) SendApprovalPendingEmail(toEmail, toName string, requestID uint, expiresAt time.Time) error {
	subject := "Asset transfer approval required"
	plain := fmt.Sprintf("Hi %s,\n\nApproval request #%d is waiting for your decision. It expires at %s.\n", toName, requestID, expiresAt.Format(time.RFC3339))
	html := fmt.Sprintf("<p>Hi %s,</p><p>Approval request <strong>#%d</strong> is waiting for your decision.</p><p>It expires at %s.</p>", toName, requestID, expiresAt.Format(time.RFC3339))
	return s.queueEmail(&EmailMessage{To: toEmail, ToName: toName, Subject: subject, PlainText: plain, HTML: html})
}

type emailService struct {
	provider        EmailProvider
	fromAddress     string
	fromName        string
	verificationURL string
	sendGridAPIKey  string
	sesRegion       string
	sesSMTPUsername string
	sesSMTPPassword string
	queue           chan *EmailMessage
}

type EmailMessage struct {
	To        string
	ToName    string
	Subject   string
	PlainText string
	HTML      string
}

func NewEmailServiceFromEnv() EmailService {
	provider := EmailProvider(strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER"))))
	if provider == "" {
		if os.Getenv("SENDGRID_API_KEY") != "" {
			provider = ProviderSendGrid
		} else if os.Getenv("SES_SMTP_USERNAME") != "" && os.Getenv("SES_SMTP_PASSWORD") != "" {
			provider = ProviderSES
		}
	}

	fromAddress := os.Getenv("EMAIL_FROM_ADDRESS")
	if fromAddress == "" {
		fromAddress = defaultFromAddress
	}
	fromName := os.Getenv("EMAIL_FROM_NAME")
	if fromName == "" {
		fromName = defaultFromName
	}
	verificationURL := os.Getenv("EMAIL_VERIFICATION_URL_BASE")
	if verificationURL == "" {
		verificationURL = defaultVerificationURL
	}

	emailSvc := &emailService{
		provider:        provider,
		fromAddress:     fromAddress,
		fromName:        fromName,
		verificationURL: verificationURL,
		sendGridAPIKey:  os.Getenv("SENDGRID_API_KEY"),
		sesRegion:       os.Getenv("SES_REGION"),
		sesSMTPUsername: os.Getenv("SES_SMTP_USERNAME"),
		sesSMTPPassword: os.Getenv("SES_SMTP_PASSWORD"),
		queue:           make(chan *EmailMessage, emailQueueBuffer),
	}

	go emailSvc.worker()
	return emailSvc
}

func (s *emailService) queueEmail(msg *EmailMessage) error {
	if s == nil {
		return errors.New("email service not configured")
	}
	select {
	case s.queue <- msg:
		return nil
	default:
		return errors.New("email queue is full")
	}
}

func (s *emailService) worker() {
	for msg := range s.queue {
		if err := s.send(msg); err != nil {
			log.Printf("email worker failed to send message to %s: %v", msg.To, err)
		}
	}
}

func (s *emailService) send(msg *EmailMessage) error {
	switch s.provider {
	case ProviderSendGrid:
		return s.sendViaSendGrid(msg)
	case ProviderSES:
		return s.sendViaSES(msg)
	default:
		return fmt.Errorf("unsupported email provider: %s", s.provider)
	}
}

func (s *emailService) sendViaSendGrid(msg *EmailMessage) error {
	if s.sendGridAPIKey == "" {
		return errors.New("sendgrid api key missing")
	}

	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{
				"to":      []map[string]string{{"email": msg.To, "name": msg.ToName}},
				"subject": msg.Subject,
			},
		},
		"from": map[string]string{"email": s.fromAddress, "name": s.fromName},
		"content": []map[string]string{
			{"type": "text/plain", "value": msg.PlainText},
			{"type": "text/html", "value": msg.HTML},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.sendGridAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var bodyBytes bytes.Buffer
		bodyBytes.ReadFrom(resp.Body)
		return fmt.Errorf("sendgrid responded with %d: %s", resp.StatusCode, bodyBytes.String())
	}
	return nil
}

func (s *emailService) sendViaSES(msg *EmailMessage) error {
	if s.sesRegion == "" || s.sesSMTPUsername == "" || s.sesSMTPPassword == "" {
		return errors.New("ses smtp configuration missing")
	}

	smtpHost := fmt.Sprintf("email-smtp.%s.amazonaws.com", s.sesRegion)
	smtpAddr := smtpHost + ":587"
	auth := smtp.PlainAuth("", s.sesSMTPUsername, s.sesSMTPPassword, smtpHost)

	message := strings.Join([]string{
		fmt.Sprintf("From: %s <%s>", s.fromName, s.fromAddress),
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("Subject: %s", msg.Subject),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s", emailContentBoundary),
		"",
		fmt.Sprintf("--%s", emailContentBoundary),
		"Content-Type: text/plain; charset=UTF-8",
		"",
		msg.PlainText,
		fmt.Sprintf("--%s", emailContentBoundary),
		"Content-Type: text/html; charset=UTF-8",
		"",
		msg.HTML,
		fmt.Sprintf("--%s--", emailContentBoundary),
	}, "\r\n")

	return smtp.SendMail(smtpAddr, auth, s.fromAddress, []string{msg.To}, []byte(message))
}

func (s *emailService) SendVerificationEmail(toEmail, toName, verificationToken string) error {
	verificationURL := fmt.Sprintf("%s?token=%s", strings.TrimRight(s.verificationURL, "/"), verificationToken)
	subject := "Verify your AssetForge email"
	plain := fmt.Sprintf("Hi %s,\n\nThanks for registering with AssetForge. Please verify your email by visiting the link below:\n\n%s\n\nIf you did not create an account, you can ignore this message.\n", toName, verificationURL)
	html := fmt.Sprintf("<p>Hi %s,</p><p>Thanks for registering with <strong>AssetForge</strong>. Please verify your email by clicking the button below.</p><p><a href=\"%s\" style=\"display:inline-block;padding:12px 20px;background:#1a73e8;color:#fff;text-decoration:none;border-radius:4px;\">Verify Email</a></p><p>If you did not create an account, you can ignore this email.</p>", toName, verificationURL)
	return s.queueEmail(&EmailMessage{To: toEmail, ToName: toName, Subject: subject, PlainText: plain, HTML: html})
}

func (s *emailService) SendKYCStatusUpdate(toEmail, toName, status, reviewNotes string) error {
	subject := fmt.Sprintf("KYC status update: %s", strings.Title(status))
	plain := fmt.Sprintf("Hi %s,\n\nYour KYC status has been updated to %s.\n\nReview notes: %s\n\nIf you have questions, contact support.\n", toName, strings.Title(status), reviewNotes)
	html := fmt.Sprintf("<p>Hi %s,</p><p>Your KYC status has been updated to <strong>%s</strong>.</p><p><strong>Review notes:</strong> %s</p><p>If you have questions, reply to this email or contact support.</p>", toName, strings.Title(status), reviewNotes)
	return s.queueEmail(&EmailMessage{To: toEmail, ToName: toName, Subject: subject, PlainText: plain, HTML: html})
}

func (s *emailService) SendTransactionConfirmation(toEmail, toName, txHash string, amount int64, assetID uint, fromAddress, toAddress string) error {
	subject := "Transaction confirmation from AssetForge"
	plain := fmt.Sprintf("Hi %s,\n\nYour transaction has been recorded successfully.\n\nTransaction hash: %s\nAsset ID: %d\nAmount: %d\nFrom: %s\nTo: %s\n\nThank you for using AssetForge.\n", toName, txHash, assetID, amount, fromAddress, toAddress)
	html := fmt.Sprintf("<p>Hi %s,</p><p>Your transaction has been recorded successfully.</p><ul><li><strong>Transaction hash:</strong> %s</li><li><strong>Asset ID:</strong> %d</li><li><strong>Amount:</strong> %d</li><li><strong>From:</strong> %s</li><li><strong>To:</strong> %s</li></ul><p>Thank you for using AssetForge.</p>", toName, txHash, assetID, amount, fromAddress, toAddress)
	return s.queueEmail(&EmailMessage{To: toEmail, ToName: toName, Subject: subject, PlainText: plain, HTML: html})
}
