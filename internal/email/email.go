package email

import (
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

// Client sends emails via SMTP.
// When SMTP is not configured, all sends are no-ops that log a warning.
type Client struct {
	host     string
	port     string
	user     string
	pass     string
	from     string
	appURL   string
	disabled bool
}

// Config holds SMTP configuration.
type Config struct {
	Host   string
	Port   string
	User   string
	Pass   string
	From   string
	AppURL string
}

// NewClient creates a new email client.
// If SMTP_HOST is empty, the client operates in no-op mode (logs only).
func NewClient(cfg Config) *Client {
	disabled := cfg.Host == ""
	if disabled {
		slog.Warn("email: SMTP_HOST not configured — email sending disabled, will log only")
	}
	appURL := cfg.AppURL
	if appURL == "" {
		appURL = "https://signsafe-web.dsmhs.kr"
	}
	port := cfg.Port
	if port == "" {
		port = "587"
	}
	return &Client{
		host:     cfg.Host,
		port:     port,
		user:     cfg.User,
		pass:     cfg.Pass,
		from:     cfg.From,
		appURL:   appURL,
		disabled: disabled,
	}
}

// SendVerificationEmail sends an email verification link to the user.
func (c *Client) SendVerificationEmail(to, token string) error {
	link := fmt.Sprintf("%s/verify-email?token=%s", c.appURL, token)
	subject := "[SignSafe] 이메일 인증을 완료해주세요"
	body := fmt.Sprintf(`안녕하세요,

SignSafe에 가입해주셔서 감사합니다.
아래 링크를 클릭하여 이메일 인증을 완료해주세요.

%s

이 링크는 24시간 동안 유효합니다.

감사합니다,
SignSafe 팀`, link)

	return c.send(to, subject, body)
}

// SendPasswordResetEmail sends a password reset link to the user.
func (c *Client) SendPasswordResetEmail(to, token string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", c.appURL, token)
	subject := "[SignSafe] 비밀번호 재설정 안내"
	body := fmt.Sprintf(`안녕하세요,

비밀번호 재설정 요청이 접수되었습니다.
아래 링크를 클릭하여 새 비밀번호를 설정해주세요.

%s

이 링크는 1시간 동안 유효합니다.
본인이 요청하지 않은 경우 이 이메일을 무시하시면 됩니다.

감사합니다,
SignSafe 팀`, link)

	return c.send(to, subject, body)
}

// send delivers a plain-text email. If SMTP is disabled, it logs the message.
func (c *Client) send(to, subject, body string) error {
	if c.disabled {
		slog.Warn("email: send skipped (SMTP disabled)",
			"to", to,
			"subject", subject,
			"body_preview", truncate(body, 120),
		)
		return nil
	}

	addr := fmt.Sprintf("%s:%s", c.host, c.port)
	from := c.from
	if from == "" {
		from = c.user
	}

	msg := buildMIMEMessage(from, to, subject, body)

	var auth smtp.Auth
	if c.user != "" && c.pass != "" {
		auth = smtp.PlainAuth("", c.user, c.pass, c.host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("email.send: %w", err)
	}

	slog.Info("email: sent", "to", to, "subject", subject)
	return nil
}

// buildMIMEMessage formats a basic MIME email message.
func buildMIMEMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString(fmt.Sprintf("From: %s\r\n", from))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", to))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
