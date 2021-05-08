package main

import (
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"io"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	gomail "github.com/go-mail/mail"
	"github.com/macrat/ayd/lib-ayd"
)

var (
	version = "HEAD"
	commit  = "UNKNOWN"
)

//go:embed templates/mail.html
var htmlTemplate string

//go:embed templates/mail.txt
var textTemplate string

func ParseSMTPServer(s string) (host string, port int, err error) {
	host, p, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, err
	}
	port, err = strconv.Atoi(p)
	if err != nil {
		return "", 0, err
	}
	if host == "" {
		return "", 0, fmt.Errorf("address %s: missing host in address", s)
	}
	return host, port, nil
}

func GetEnv(key string, default_ string) string {
	value := os.Getenv(strings.ToLower(key))
	if value == "" {
		value = os.Getenv(strings.ToUpper(key))
	}
	if value == "" {
		value = default_
	}
	return value
}

func GetRequiredEnv(key string) string {
	value := GetEnv(key, "")
	if value == "" {
		fmt.Fprintf(os.Stderr, "Environment variable `%s` is required.\n", key)
		os.Exit(2)
	}
	return value
}

func GetMessage(aydURL, targetURL *url.URL) (string, error) {
	resp, err := ayd.Fetch(aydURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch status: %w", err)
	}

	rs, err := resp.RecordsOf(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch status: %w", err)
	}

	return rs[len(rs)-1].Message, nil
}

type Context struct {
	StatusPage string
	Target     string
	CheckedAt  string
	Status     string
	Message    string
}

func main() {
	args, err := ayd.ParseAlertPluginArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, "$ ayd-mailto-alert MAILTO_URI TARGET_URI TARGET_STATUS TARGET_CHECKED_AT")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	logger := ayd.NewLogger(args.AlertURL)

	smtpHost, smtpPort, err := ParseSMTPServer(GetRequiredEnv("smtp_server"))
	if err != nil {
		logger.Failure(fmt.Sprintf("environment variable `smtp_server` is invalid: %s", err))
		return
	}
	smtpUsername := GetRequiredEnv("smtp_username")
	smtpPassword := GetRequiredEnv("smtp_password")

	from, err := mail.ParseAddress(GetEnv("ayd_mail_from", "Ayd? Alert <ayd@localhost>"))
	if err != nil {
		logger.Failure(fmt.Sprintf("environment variable `ayd_mail_from` is invalid: %s", err))
		return
	}

	to, err := mail.ParseAddressList(args.AlertURL.Opaque)
	if err != nil {
		logger.Failure(fmt.Sprintf("mail address is invalid: %s", err))
		return
	}

	aydURL, err := url.Parse(GetEnv("ayd_url", "http://localhost:9000"))
	if err != nil {
		logger.Failure(fmt.Sprintf("environment variable `ayd_url` is invalid: %s", err))
		return
	}
	statusPage, err := aydURL.Parse("status.html")
	if err != nil {
		logger.Failure(fmt.Sprintf("failed to generate status page URL: %s", err))
		return
	}

	message, err := GetMessage(aydURL, args.TargetURL)
	if err != nil {
		logger.Unknown(err.Error())
	}

	ctx := Context{
		StatusPage: statusPage.String(),
		Target:     args.TargetURL.String(),
		Status:     args.Status.String(),
		CheckedAt:  args.CheckedAt.Format(time.RFC3339),
		Message:    message,
	}

	html := htmltemplate.Must(htmltemplate.New("mail.html").Parse(htmlTemplate))
	text := texttemplate.Must(texttemplate.New("mail.txt").Parse(textTemplate))

	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", from.Address, from.Name)
	for _, t := range to {
		msg.SetAddressHeader("To", t.Address, t.Name)
	}
	msg.SetHeader("Subject", fmt.Sprintf("[%s] %s", ctx.Status, ctx.Target))
	msg.SetBodyWriter("text/text", func(w io.Writer) error {
		return text.Execute(w, ctx)
	})
	msg.AddAlternativeWriter("text/html", func(w io.Writer) error {
		return html.Execute(w, ctx)
	})

	dialer := gomail.NewDialer(smtpHost, smtpPort, smtpUsername, smtpPassword)
	dialer.StartTLSPolicy = gomail.MandatoryStartTLS

	if err := dialer.DialAndSend(msg); err != nil {
		logger.Failure(fmt.Sprintf("failed to send e-mail: %s", err))
		return
	}

	logger.Healthy(fmt.Sprintf("sent alert to %s", to))
}
