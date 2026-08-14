// Package mailer delivers transactional email. When no SMTP server is
// configured the LogMailer is used, which logs messages to the application
// logger so reset links remain visible in development.
package mailer

import (
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
)

// Mailer sends an HTML email to a single recipient.
type Mailer interface {
	SendEmail(to, subject, htmlBody string) error
}

// Config describes how to reach the SMTP relay. An empty Host selects the
// LogMailer.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// New returns an SMTP-backed mailer when a host is configured, otherwise a
// LogMailer that logs messages instead of sending them.
func New(cfg Config) Mailer {
	if cfg.Host == "" {
		return &LogMailer{logger: slog.Default()}
	}
	if cfg.From == "" {
		cfg.From = "no-reply@iaas.local"
	}
	return &SMTPMailer{cfg: cfg}
}

// LogMailer records the message in the application log. Used when SMTP is not
// configured so password reset links are still reachable in development.
type LogMailer struct {
	logger *slog.Logger
}

func (m *LogMailer) SendEmail(to, subject, htmlBody string) error {
	m.logger.Info("email would be sent (no SMTP configured)", "to", to, "subject", subject, "body", htmlBody)
	return nil
}

// SMTPMailer relays mail through an SMTP server using STARTTLS.
type SMTPMailer struct {
	cfg Config
}

func (m *SMTPMailer) SendEmail(to, subject, htmlBody string) error {
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}
	msg := buildMessage(m.cfg.From, to, subject, htmlBody)
	if err := smtp.SendMail(addr, auth, m.cfg.From, []string{to}, msg); err != nil {
		return fmt.Errorf("send email via SMTP: %w", err)
	}
	return nil
}

func buildMessage(from, to, subject, htmlBody string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	b.WriteString("\r\n")
	return []byte(b.String())
}
