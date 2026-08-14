package mailer

import (
	"strings"
	"testing"
)

func TestNew_NoHost_ReturnsLogMailer(t *testing.T) {
	if _, ok := New(Config{}).(*LogMailer); !ok {
		t.Fatal("expected LogMailer when SMTP host is empty")
	}
}

func TestNew_WithHost_ReturnsSMTPMailer(t *testing.T) {
	m, ok := New(Config{Host: "smtp.example.com", Port: 587}).(*SMTPMailer)
	if !ok {
		t.Fatal("expected SMTPMailer when a host is configured")
	}
	if m.cfg.From != "no-reply@iaas.local" {
		t.Fatalf("expected default From, got %q", m.cfg.From)
	}
}

func TestSMTPMailer_New_HonorsConfiguredFrom(t *testing.T) {
	m := New(Config{Host: "smtp.example.com", Port: 587, From: "alerts@example.com"}).(*SMTPMailer)
	if m.cfg.From != "alerts@example.com" {
		t.Fatalf("expected configured From, got %q", m.cfg.From)
	}
}

func TestBuildMessage_ContainsHeadersAndBody(t *testing.T) {
	msg := string(buildMessage("from@example.com", "to@example.com", "Subject line", "<p>Hello</p>"))
	for _, want := range []string{
		"From: from@example.com",
		"To: to@example.com",
		"Subject: Subject line",
		"Content-Type: text/html; charset=UTF-8",
		"<p>Hello</p>",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message missing %q: %q", want, msg)
		}
	}
}
