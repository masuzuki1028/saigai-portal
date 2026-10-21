package notifier

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/smtp"
	"time"
)

// SMTPMailer は SMTP でメールを送る。ローカルでは Mailpit（127.0.0.1:1025）に送る。認証は行わない。
type SMTPMailer struct {
	Addr string
	From string
}

func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", m.From)
	fmt.Fprintf(&b, "To: %s\r\n", oneLine(to))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.BEncoding.Encode("UTF-8", oneLine(subject)))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(body)
	return smtp.SendMail(m.Addr, nil, m.From, []string{to}, b.Bytes())
}
