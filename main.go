package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	texttemplate "text/template"

	gomail "github.com/go-mail/mail"
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

func GetMessage(aydURL *url.URL, target string) string {
	u, err := aydURL.Parse("status.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate status endpoint URL: %s\n", err)
		os.Exit(1)
	}

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch message: %s\n", err)
		return ""
	}
	defer resp.Body.Close()

	var msg struct {
		Incidents []struct {
			Target  string `json:"target"`
			Message string `json:"message"`
		} `json:"current_incidents"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse status data: %s\n", err)
		return ""
	}

	for _, incident := range msg.Incidents {
		if incident.Target == target {
			return incident.Message
		}
	}
	fmt.Fprintf(os.Stderr, "No such incident information: %s\n", target)
	return ""
}

type Context struct {
	StatusPage string
	Target     string
	CheckedAt  string
	Status     string
	Message    string
}

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "$ ayd-mailto-alert MAILTO_URI TARGET_URI TARGET_STATUS TARGET_CHECKED_AT")
		os.Exit(2)
	}

	fmt.Printf("ayd-mailto-alert %s (%s): ", version, commit)

	smtpHost, smtpPort, err := ParseSMTPServer(GetRequiredEnv("smtp_server"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Environment variable `smtp_server` is invalid: %s\n", err)
		os.Exit(2)
	}
	smtpUsername := GetRequiredEnv("smtp_username")
	smtpPassword := GetRequiredEnv("smtp_password")

	from, err := mail.ParseAddress(GetEnv("ayd_mail_from", "Ayd? Alert <ayd@localhost>"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Environment variable `ayd_mail_from` is invalid: %s\n", err)
		os.Exit(2)
	}

	mailto, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Argument MAILTO_URI is invalid: %s\n", err)
		os.Exit(2)
	}
	to, err := mail.ParseAddressList(mailto.Opaque)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Argument MAILTO_URI is invalid: %s\n", err)
		os.Exit(2)
	}

	aydURL, err := url.Parse(GetEnv("ayd_url", "http://localhost:9000"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Environment variable `ayd_url` is invalid: %s\n", err)
		os.Exit(2)
	}
	statusPage, err := aydURL.Parse("status.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate status page URL: %s\n", err)
		os.Exit(1)
	}

	ctx := Context{
		StatusPage: statusPage.String(),
		Target:     os.Args[2],
		Status:     os.Args[3],
		CheckedAt:  os.Args[4],
		Message:    GetMessage(aydURL, os.Args[2]),
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
		fmt.Fprintf(os.Stderr, "failed to send e-mail: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sent alert to %s\n", to)
}
