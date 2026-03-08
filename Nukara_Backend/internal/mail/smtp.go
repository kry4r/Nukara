package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	netsmtp "net/smtp"
	"strconv"
	"strings"
	"time"
)

type SettingsReader interface {
	GetSystemSetting(key string) (value string, ok bool)
}

type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	CodeTTL   time.Duration
}

type Message struct {
	Subject string
	HTML    string
	Text    string
}

type Sender struct {
	settings SettingsReader
	send     func(cfg SMTPConfig, to string, msg Message) error
}

func NewSMTPSender(settings SettingsReader) *Sender {
	return &Sender{settings: settings, send: sendViaSMTP}
}

func LoadSMTPConfig(settings SettingsReader) (SMTPConfig, error) {
	if settings == nil {
		return SMTPConfig{}, errors.New("smtp settings unavailable")
	}
	cfg := SMTPConfig{
		Host:      readSetting(settings, "smtp_host"),
		Username:  readSetting(settings, "smtp_username"),
		Password:  readSetting(settings, "smtp_password"),
		FromEmail: readSetting(settings, "smtp_from_email"),
		FromName:  readSetting(settings, "smtp_from_name"),
		CodeTTL:   15 * time.Minute,
	}
	if cfg.FromName == "" {
		cfg.FromName = "Nukara"
	}
	if cfg.FromEmail == "" {
		cfg.FromEmail = cfg.Username
	}
	if portRaw := readSetting(settings, "smtp_port"); portRaw != "" {
		port, err := strconv.Atoi(portRaw)
		if err != nil || port <= 0 {
			return SMTPConfig{}, errors.New("invalid smtp_port")
		}
		cfg.Port = port
	} else {
		cfg.Port = 465
	}
	if ttlRaw := readSetting(settings, "email_code_ttl_seconds"); ttlRaw != "" {
		seconds, err := strconv.Atoi(ttlRaw)
		if err != nil || seconds <= 0 {
			return SMTPConfig{}, errors.New("invalid email_code_ttl_seconds")
		}
		cfg.CodeTTL = time.Duration(seconds) * time.Second
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" || cfg.FromEmail == "" {
		return SMTPConfig{}, errors.New("smtp not configured")
	}
	return cfg, nil
}

func BuildVerificationMessage(code string, ttl time.Duration) Message {
	minutes := int(ttl.Round(time.Minute) / time.Minute)
	if minutes <= 0 {
		minutes = 15
	}
	html := fmt.Sprintf(`<!doctype html>
<html>
  <body style="margin:0;padding:24px;background:#f6f8fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#1f2937;">
    <div style="max-width:560px;margin:0 auto;background:#ffffff;border:1px solid #e5e7eb;border-radius:16px;overflow:hidden;box-shadow:0 12px 40px rgba(15,23,42,.08);">
      <div style="padding:24px 28px;background:linear-gradient(135deg,#111827,#1f2937);color:#ffffff;">
        <div style="font-size:14px;opacity:.85;letter-spacing:.08em;text-transform:uppercase;">Nukara</div>
        <h1 style="margin:8px 0 0;font-size:24px;line-height:1.3;">邮箱验证码</h1>
      </div>
      <div style="padding:28px;">
        <p style="margin:0 0 16px;font-size:16px;line-height:1.7;">你好，你本次的验证码如下：</p>
        <div style="margin:0 0 20px;padding:18px 20px;border-radius:14px;background:#f3f4f6;border:1px dashed #cbd5e1;text-align:center;">
          <div style="font-size:32px;font-weight:700;letter-spacing:.32em;color:#111827;">%s</div>
        </div>
        <p style="margin:0 0 12px;font-size:15px;line-height:1.7;">该验证码将在 <strong>%d 分钟</strong> 后失效，请尽快完成验证。</p>
        <p style="margin:0;font-size:14px;line-height:1.7;color:#6b7280;">如果这不是你的操作，请忽略本邮件。</p>
      </div>
    </div>
  </body>
</html>`, code, minutes)
	text := fmt.Sprintf("Nukara 邮箱验证码\n\n验证码：%s\n有效期：%d 分钟\n\n如果这不是你的操作，请忽略本邮件。", code, minutes)
	return Message{
		Subject: "Nukara 邮箱验证码",
		HTML:    html,
		Text:    text,
	}
}

func BuildTestMessage() Message {
	now := time.Now().Format("2006-01-02 15:04:05")
	html := fmt.Sprintf(`<!doctype html>
<html>
  <body style="margin:0;padding:24px;background:#f6f8fb;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#1f2937;">
    <div style="max-width:560px;margin:0 auto;background:#ffffff;border:1px solid #e5e7eb;border-radius:16px;overflow:hidden;box-shadow:0 12px 40px rgba(15,23,42,.08);">
      <div style="padding:24px 28px;background:linear-gradient(135deg,#312e81,#4338ca);color:#ffffff;">
        <div style="font-size:14px;opacity:.85;letter-spacing:.08em;text-transform:uppercase;">Nukara</div>
        <h1 style="margin:8px 0 0;font-size:24px;line-height:1.3;">SMTP 测试邮件</h1>
      </div>
      <div style="padding:28px;">
        <p style="margin:0 0 16px;font-size:16px;line-height:1.7;">这是一封来自 Admin 面板的测试邮件，以下验证码仅用于模板预览：</p>
        <div style="margin:0 0 20px;padding:18px 20px;border-radius:14px;background:#eef2ff;border:1px dashed #a5b4fc;text-align:center;">
          <div style="font-size:32px;font-weight:700;letter-spacing:.32em;color:#312e81;">246810</div>
        </div>
        <p style="margin:0 0 12px;font-size:15px;line-height:1.7;">预览验证码有效期为 <strong>15 分钟</strong>。</p>
        <p style="margin:0;font-size:14px;line-height:1.7;color:#6b7280;">发送时间：%s</p>
      </div>
    </div>
  </body>
</html>`, now)
	text := fmt.Sprintf("Nukara SMTP 测试邮件\n\n这是一封来自 Admin 面板的测试邮件，以下验证码仅用于模板预览：246810\n有效期：15 分钟\n发送时间：%s", now)
	return Message{Subject: "Nukara SMTP 测试邮件", HTML: html, Text: text}
}

func (s *Sender) SendVerificationCode(ctx context.Context, to, code string, ttl time.Duration) error {
	_ = ctx
	cfg, err := LoadSMTPConfig(s.settings)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = cfg.CodeTTL
	}
	return s.send(cfg, strings.TrimSpace(to), BuildVerificationMessage(code, ttl))
}

func (s *Sender) SendTestMail(ctx context.Context, to string) error {
	_ = ctx
	cfg, err := LoadSMTPConfig(s.settings)
	if err != nil {
		return err
	}
	return s.send(cfg, strings.TrimSpace(to), BuildTestMessage())
}

func readSetting(settings SettingsReader, key string) string {
	value, _ := settings.GetSystemSetting(key)
	return strings.TrimSpace(value)
}

func sendViaSMTP(cfg SMTPConfig, to string, msg Message) error {
	if strings.TrimSpace(to) == "" {
		return errors.New("recipient email required")
	}
	raw := buildMIMEMessage(cfg, to, msg)
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	if cfg.Port == 465 {
		return sendImplicitTLS(addr, cfg, to, raw)
	}
	return sendStartTLS(addr, cfg, to, raw)
}

func buildMIMEMessage(cfg SMTPConfig, to string, msg Message) []byte {
	boundary := fmt.Sprintf("nukara-%d", time.Now().UnixNano())
	fromDisplay := cfg.FromEmail
	if strings.TrimSpace(cfg.FromName) != "" {
		fromDisplay = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", cfg.FromName), cfg.FromEmail)
	}
	subject := mime.QEncoding.Encode("utf-8", msg.Subject)
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("From: %s\r\n", fromDisplay))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", to))
	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n", boundary))
	builder.WriteString("\r\n")
	builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	builder.WriteString(msg.Text)
	builder.WriteString("\r\n")
	builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	builder.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	builder.WriteString(msg.HTML)
	builder.WriteString("\r\n")
	builder.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return []byte(builder.String())
}

func sendImplicitTLS(addr string, cfg SMTPConfig, to string, raw []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := netsmtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Quit()
	if err := client.Auth(netsmtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return err
	}
	if err := client.Mail(cfg.FromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	wc, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(raw); err != nil {
		_ = wc.Close()
		return err
	}
	return wc.Close()
}

func sendStartTLS(addr string, cfg SMTPConfig, to string, raw []byte) error {
	client, err := netsmtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Quit()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return err
		}
	}
	if err := client.Auth(netsmtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return err
	}
	if err := client.Mail(cfg.FromEmail); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	wc, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(raw); err != nil {
		_ = wc.Close()
		return err
	}
	return wc.Close()
}
