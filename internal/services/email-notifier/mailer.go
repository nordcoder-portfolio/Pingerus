package notifier

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strings"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/email-notifier"
	"go.uber.org/zap"
)

type Mailer struct {
	addr       string
	auth       smtp.Auth
	useTLS     bool
	timeout    time.Duration
	from       string
	subjPrefix string

	log *zap.Logger
}

func New(cfg config.SMTP) *Mailer {
	var auth smtp.Auth
	if cfg.User != "" || cfg.Password != "" {
		host := host(cfg.Addr)
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, host)
	}
	return &Mailer{
		addr:       cfg.Addr,
		auth:       auth,
		useTLS:     cfg.UseTLS,
		timeout:    cfg.Timeout,
		from:       cfg.From,
		subjPrefix: cfg.SubjPrefix,
		log:        zap.L().With(zap.String("component", "email-notifier.mailer")),
	}
}

func (m *Mailer) WithLogger(l *zap.Logger) *Mailer {
	if l == nil {
		return m
	}
	cp := *m
	cp.log = l.With(zap.String("component", "email-notifier.mailer"))
	return &cp
}

func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	subj := strings.TrimSpace(m.subjPrefix + " " + subject)
	msg := []byte(
		"From: " + m.from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subj + "\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" + body + "\r\n")

	start := time.Now()
	log := m.log.With(
		zap.String("smtp_addr", m.addr),
		zap.Bool("tls", m.useTLS),
		zap.String("from", m.from),
		zap.String("to", to),
		zap.String("subject", subj),
	)

	dialer := net.Dialer{Timeout: m.timeout}

	if m.useTLS {
		log.Debug("sending email (TLS)...")
		conn, err := tls.DialWithDialer(&dialer, "tcp", m.addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			log.Error("tls dial failed", zap.Error(err))
			return err
		}
		c, err := smtp.NewClient(conn, host(m.addr))
		if err != nil {
			log.Error("smtp client failed", zap.Error(err))
			return err
		}
		defer func() { _ = c.Close() }()

		if m.auth != nil {
			if ok, _ := c.Extension("AUTH"); ok {
				if err := c.Auth(m.auth); err != nil {
					log.Error("smtp auth failed", zap.Error(err))
					return err
				}
			}
		}
		if err := c.Mail(m.from); err != nil {
			log.Error("smtp MAIL FROM failed", zap.Error(err))
			return err
		}
		if err := c.Rcpt(to); err != nil {
			log.Error("smtp RCPT TO failed", zap.Error(err))
			return err
		}
		w, err := c.Data()
		if err != nil {
			log.Error("smtp DATA failed", zap.Error(err))
			return err
		}
		if _, err = w.Write(msg); err != nil {
			log.Error("smtp write failed", zap.Error(err))
			return err
		}
		if err := w.Close(); err != nil {
			log.Error("smtp close failed", zap.Error(err))
			return err
		}
		log.Info("email sent (TLS)", zap.Duration("elapsed", time.Since(start)))
		return nil
	}

	log.Debug("sending email (PLAIN)...")
	if err := smtp.SendMail(m.addr, m.auth, m.from, []string{to}, msg); err != nil {
		log.Error("sendmail failed", zap.Error(err))
		return err
	}
	log.Info("email sent (PLAIN)", zap.Duration("elapsed", time.Since(start)))
	return nil
}

func host(addr string) string {
	if i := strings.Index(addr, ":"); i >= 0 {
		return addr[:i]
	}
	return addr
}
