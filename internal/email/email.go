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
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:40px 0;">
    <tr><td align="center">
      <table width="520" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
        <!-- Header -->
        <tr>
          <td style="background:#18181b;padding:32px 40px;">
            <p style="margin:0;font-size:20px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">SignSafe</p>
            <p style="margin:4px 0 0;font-size:12px;color:#a1a1aa;">계약서 리스크 분석 플랫폼</p>
          </td>
        </tr>
        <!-- Body -->
        <tr>
          <td style="padding:40px;">
            <p style="margin:0 0 8px;font-size:22px;font-weight:600;color:#18181b;">이메일 인증</p>
            <p style="margin:0 0 24px;font-size:14px;color:#71717a;line-height:1.6;">
              SignSafe에 가입해주셔서 감사합니다.<br>
              아래 버튼을 클릭하여 이메일 인증을 완료해주세요.
            </p>
            <!-- CTA Button -->
            <table cellpadding="0" cellspacing="0" style="margin:0 0 32px;">
              <tr>
                <td style="background:#18181b;border-radius:8px;">
                  <a href="%s" style="display:inline-block;padding:14px 32px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;letter-spacing:-0.1px;">이메일 인증하기</a>
                </td>
              </tr>
            </table>
            <!-- Info -->
            <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;border-radius:8px;padding:16px;">
              <tr>
                <td style="font-size:12px;color:#71717a;line-height:1.7;">
                  • 이 링크는 <strong style="color:#52525b;">24시간</strong> 동안 유효합니다.<br>
                  • 버튼이 작동하지 않으면 아래 링크를 브라우저에 복사해주세요.<br>
                  <span style="color:#3f3f46;word-break:break-all;">%s</span>
                </td>
              </tr>
            </table>
          </td>
        </tr>
        <!-- Footer -->
        <tr>
          <td style="padding:20px 40px;border-top:1px solid #f4f4f5;">
            <p style="margin:0;font-size:12px;color:#a1a1aa;">
              본인이 가입하지 않으셨다면 이 이메일을 무시하셔도 됩니다.<br>
              © 2026 SignSafe. All rights reserved.
            </p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, link, link)

	return c.sendHTML(to, subject, body)
}

// SendPasswordResetEmail sends a password reset link to the user.
func (c *Client) SendPasswordResetEmail(to, token string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", c.appURL, token)
	subject := "[SignSafe] 비밀번호 재설정 안내"
	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="ko">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head>
<body style="margin:0;padding:0;background:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;padding:40px 0;">
    <tr><td align="center">
      <table width="520" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.1);">
        <!-- Header -->
        <tr>
          <td style="background:#18181b;padding:32px 40px;">
            <p style="margin:0;font-size:20px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">SignSafe</p>
            <p style="margin:4px 0 0;font-size:12px;color:#a1a1aa;">계약서 리스크 분석 플랫폼</p>
          </td>
        </tr>
        <!-- Body -->
        <tr>
          <td style="padding:40px;">
            <p style="margin:0 0 8px;font-size:22px;font-weight:600;color:#18181b;">비밀번호 재설정</p>
            <p style="margin:0 0 24px;font-size:14px;color:#71717a;line-height:1.6;">
              비밀번호 재설정 요청이 접수되었습니다.<br>
              아래 버튼을 클릭하여 새 비밀번호를 설정해주세요.
            </p>
            <!-- CTA Button -->
            <table cellpadding="0" cellspacing="0" style="margin:0 0 32px;">
              <tr>
                <td style="background:#18181b;border-radius:8px;">
                  <a href="%s" style="display:inline-block;padding:14px 32px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;letter-spacing:-0.1px;">비밀번호 재설정하기</a>
                </td>
              </tr>
            </table>
            <!-- Info -->
            <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f4f4f5;border-radius:8px;padding:16px;">
              <tr>
                <td style="font-size:12px;color:#71717a;line-height:1.7;">
                  • 이 링크는 <strong style="color:#52525b;">1시간</strong> 동안 유효합니다.<br>
                  • 본인이 요청하지 않으셨다면 이 이메일을 무시하셔도 됩니다.<br>
                  • 버튼이 작동하지 않으면 아래 링크를 브라우저에 복사해주세요.<br>
                  <span style="color:#3f3f46;word-break:break-all;">%s</span>
                </td>
              </tr>
            </table>
          </td>
        </tr>
        <!-- Footer -->
        <tr>
          <td style="padding:20px 40px;border-top:1px solid #f4f4f5;">
            <p style="margin:0;font-size:12px;color:#a1a1aa;">
              © 2026 SignSafe. All rights reserved.
            </p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, link, link)

	return c.sendHTML(to, subject, body)
}

// sendHTML delivers an HTML email. If SMTP is disabled, it logs the message.
func (c *Client) sendHTML(to, subject, body string) error {
	if c.disabled {
		slog.Warn("email: send skipped (SMTP disabled)",
			"to", to,
			"subject", subject,
		)
		return nil
	}

	addr := fmt.Sprintf("%s:%s", c.host, c.port)
	from := c.from
	if from == "" {
		from = c.user
	}

	msg := buildHTMLMessage(from, to, subject, body)

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

// buildHTMLMessage formats a MIME email message with HTML content.
func buildHTMLMessage(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	sb.WriteString(fmt.Sprintf("From: SignSafe <%s>\r\n", from))
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
