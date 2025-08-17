package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	config "github.com/NordCoder/Pingerus/internal/config/email-notifier"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Mailer struct {
	addr       string
	auth       smtp.Auth
	useTLS     bool
	timeout    time.Duration
	from       string
	subjPrefix string
}

func NewMailer(cfg config.SMTP) *Mailer {
	var auth smtp.Auth
	if cfg.User != "" || cfg.Password != "" {
		host := cfg.Addr
		if i := strings.Index(host, ":"); i >= 0 {
			host = host[:i]
		}
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, host)
	}
	return &Mailer{
		addr:       cfg.Addr,
		auth:       auth,
		useTLS:     cfg.UseTLS,
		timeout:    cfg.Timeout,
		from:       cfg.From,
		subjPrefix: cfg.SubjPrefix,
	}
}

func (m *Mailer) Send(ctx context.Context, to string, subject string, body string) error {
	subj := fmt.Sprintf("%s %s", m.subjPrefix, subject)

	msg := []byte(
		"From: " + m.from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subj + "\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" + body + "\r\n")

	dialer := net.Dialer{Timeout: m.timeout}
	if m.useTLS {
		conn, err := tls.DialWithDialer(&dialer, "tcp", m.addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		c, err := smtp.NewClient(conn, smtpHost(m.addr))
		if err != nil {
			return err
		}
		defer c.Close()
		if m.auth != nil {
			if ok, _ := c.Extension("AUTH"); ok {
				if err := c.Auth(m.auth); err != nil {
					return err
				}
			}
		}
		if err := c.Mail(m.from); err != nil {
			return err
		}
		if err := c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err = w.Write(msg); err != nil {
			return err
		}
		return w.Close()
	}

	return smtp.SendMail(m.addr, m.auth, m.from, []string{to}, msg)
}

func smtpHost(addr string) string {
	if i := strings.Index(addr, ":"); i >= 0 {
		return addr[:i]
	}
	return addr
}
