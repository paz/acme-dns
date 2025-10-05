package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Config holds email configuration
type Config struct {
	Enabled     bool
	SMTPHost    string
	SMTPPort    int
	SMTPUser    string
	SMTPPass    string
	FromEmail   string
	FromName    string
	UseTLS      bool
	UseStartTLS bool
}

// Mailer handles email sending
type Mailer struct {
	config Config
}

// NewMailer creates a new email sender
func NewMailer(config Config) *Mailer {
	return &Mailer{config: config}
}

// SendEmail sends an email using the configured SMTP server
func (m *Mailer) SendEmail(to, subject, body string) error {
	if !m.config.Enabled {
		log.Debug("Email disabled, skipping send")
		return fmt.Errorf("email functionality is disabled")
	}

	// Build message
	from := m.config.FromEmail
	if m.config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", m.config.FromName, m.config.FromEmail)
	}

	msg := buildMessage(from, to, subject, body)

	// Determine authentication
	var auth smtp.Auth
	if m.config.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.config.SMTPUser, m.config.SMTPPass, m.config.SMTPHost)
	}

	addr := fmt.Sprintf("%s:%d", m.config.SMTPHost, m.config.SMTPPort)

	// Send email based on TLS configuration
	if m.config.UseTLS {
		// Direct TLS connection (port 465)
		return m.sendWithTLS(addr, auth, from, to, msg)
	} else if m.config.UseStartTLS {
		// STARTTLS (port 587)
		return m.sendWithStartTLS(addr, auth, from, to, msg)
	}

	// Plain SMTP (port 25)
	return smtp.SendMail(addr, auth, m.config.FromEmail, []string{to}, msg)
}

// sendWithTLS sends email with direct TLS connection (port 465)
func (m *Mailer) sendWithTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	// TLS config
	tlsConfig := &tls.Config{
		ServerName: m.config.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}

	// Connect with TLS
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "addr": addr}).Error("Failed to connect with TLS")
		return fmt.Errorf("TLS connection failed: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, m.config.SMTPHost)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to create SMTP client")
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Quit()

	// Authenticate
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			log.WithFields(log.Fields{"error": err}).Error("SMTP authentication failed")
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	// Send email
	if err = client.Mail(m.config.FromEmail); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("message write failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("message close failed: %w", err)
	}

	log.WithFields(log.Fields{"to": to, "subject": extractSubject(string(msg))}).Info("Email sent successfully via TLS")
	return nil
}

// sendWithStartTLS sends email with STARTTLS (port 587)
func (m *Mailer) sendWithStartTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	// Connect
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "addr": addr}).Error("Failed to connect to SMTP server")
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, m.config.SMTPHost)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to create SMTP client")
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Quit()

	// STARTTLS
	tlsConfig := &tls.Config{
		ServerName: m.config.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}

	if err = client.StartTLS(tlsConfig); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("STARTTLS failed")
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	// Authenticate
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			log.WithFields(log.Fields{"error": err}).Error("SMTP authentication failed")
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	// Send email
	if err = client.Mail(m.config.FromEmail); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("message write failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("message close failed: %w", err)
	}

	log.WithFields(log.Fields{"to": to, "subject": extractSubject(string(msg))}).Info("Email sent successfully via STARTTLS")
	return nil
}

// buildMessage builds an RFC 5322 email message
func buildMessage(from, to, subject, body string) []byte {
	msg := fmt.Sprintf("From: %s\r\n", from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: text/html; charset=UTF-8\r\n"
	msg += "\r\n"
	msg += body

	return []byte(msg)
}

// extractSubject extracts subject from message for logging
func extractSubject(msg string) string {
	lines := strings.Split(msg, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Subject: ") {
			return strings.TrimPrefix(line, "Subject: ")
		}
	}
	return "(no subject)"
}
