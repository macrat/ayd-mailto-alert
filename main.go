package main

import (
	"bufio"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"io"
	"net"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/google/shlex"
	"github.com/google/uuid"
	"github.com/macrat/ayd/lib-ayd"
	gomail "github.com/wneessen/go-mail"
)

var (
	version = "HEAD"
	commit  = "UNKNOWN"
)

//go:embed templates/mail.html
var htmlTemplate string

//go:embed templates/mail.txt
var textTemplate string

func ParseHost(s string) (host string, port int, err error) {
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

func ParseURL(s string) (host string, port int, ssl bool, err error) {
	u, err := url.Parse(s)
	if err != nil {
		return
	}

	switch u.Scheme {
	case "smtp":
		ssl = false
	case "smtps":
		ssl = true
	default:
		err = fmt.Errorf("unsupported protocol: '%s'", u.Scheme)
		return
	}

	if u.Port() == "" {
		if ssl {
			u.Host += ":465"
		} else {
			u.Host += ":25"
		}
	}

	host, port, err = ParseHost(u.Host)
	if err != nil {
		return "", 0, false, err
	}

	return
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

type Config struct {
	Host     string
	Port     int
	SSL      bool
	Username string
	Password string
	From     *mail.Address
}

func LoadConfig() (Config, error) {
	pathes := []string{"/usr/share/misc/mail.rc", "/usr/local/etc/mail.rc", "/etc/mail.rc"}
	if home, err := os.UserHomeDir(); err == nil {
		pathes = append(pathes, filepath.Join(home, ".mailrc"))
	}

	return LoadConfigByPath(pathes)
}

func LoadConfigByPath(pathes []string) (Config, error) {
	conf := Config{
		SSL:  true,
		From: &mail.Address{Name: "Ayd Alert", Address: "ayd@localhost"},
	}

	for _, p := range pathes {
		if f, err := os.Open(p); err == nil {
			if err = conf.LoadFile(f); err != nil {
				return conf, err
			}
		}
	}

	if err := conf.LoadEnv(); err != nil {
		return conf, err
	}

	if conf.Host == "" {
		return conf, errors.New("SMTP_SERVER is required")
	}
	if conf.Username == "" {
		return conf, errors.New("SMTP_USERNAME is required")
	}
	if conf.Password == "" {
		return conf, errors.New("SMTP_PASSWORD is required")
	}

	return conf, nil
}

func (conf *Config) LoadFile(f io.Reader) error {
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		xs, err := shlex.Split(scanner.Text())
		if err != nil {
			return err
		}

		if len(xs) < 2 || xs[0] != "set" {
			continue
		}

		for _, s := range xs[1:] {
			kv := strings.SplitN(s, "=", 2)
			if len(kv) != 2 {
				continue
			}

			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(strings.SplitN(kv[1], " #", 2)[0])

			switch key {
			case "smtp":
				conf.Host, conf.Port, conf.SSL, err = ParseURL(value)
				if err != nil {
					return err
				}
			case "smtp-auth-user":
				conf.Username = value
			case "smtp-auth-password":
				conf.Password = value
			case "from":
				conf.From, err = mail.ParseAddress(value)
				if err != nil {
					return err
				}
			default:
				continue
			}
		}
	}

	return scanner.Err()
}

func (conf *Config) LoadEnv() error {
	var err error

	if server := GetEnv("smtp_server", ""); server != "" {
		conf.Host, conf.Port, err = ParseHost(server)
		if err != nil {
			return fmt.Errorf("environment variable `smtp_server` is invalid: %s", err)
		}
	}

	if username := GetEnv("smtp_username", ""); username != "" {
		conf.Username = username
	}
	if password := GetEnv("smtp_password", ""); password != "" {
		conf.Password = password
	}

	if from := GetEnv("ayd_mail_from", ""); from != "" {
		conf.From, err = mail.ParseAddress(from)
		if err != nil {
			return fmt.Errorf("environment variable `ayd_mail_from` is invalid: %s", err)
		}
	}

	return nil
}

type Context struct {
	StatusPage string
	Target     string
	CheckedAt  string
	Status     string
	Message    string
	Extra      []ayd.ExtraPair
}

func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ayd-mailto-alert MAILTO_URL RECORD")
}

func main() {
	showVersion := flag.Bool("v", false, "show version and exit.")
	flag.Usage = Usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("ayd-mailto-alert %s (%s)\n", version, commit)
		return
	}

	if len(os.Args) != 3 {
		Usage()
		os.Exit(2)
	}
	alertURL, err := ayd.ParseURL(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		Usage()
		os.Exit(2)
	}
	record, err := ayd.ParseRecord(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		Usage()
		os.Exit(2)
	}

	target := &ayd.URL{
		Scheme: alertURL.Scheme,
		Opaque: alertURL.Opaque,
	}
	logger := ayd.NewLogger(target)

	conf, err := LoadConfig()
	extra := map[string]interface{}{}
	if from := alertURL.ToURL().Query().Get("from"); from != "" {
		var e2 error
		conf.From, e2 = mail.ParseAddress(from)
		if e2 != nil {
			logger.Failure(fmt.Sprintf("\"from\" e-mail address is invalid: %s", e2), extra)
			return
		}

		target.RawQuery = (url.Values{
			"from": {from},
		}).Encode()
	}
	if conf.Host != "" {
		extra["smtp_server"] = conf.Host
		extra["from_address"] = conf.From.String()
	}
	if err != nil {
		logger.Failure(err.Error(), extra)
		return
	}

	to, err := mail.ParseAddressList(alertURL.Opaque)
	if err != nil {
		logger.Failure(fmt.Sprintf("\"to\" e-mail address is invalid: %s", err), extra)
		return
	}

	var statusPage string
	if rawURL := GetEnv("ayd_url", ""); rawURL != "" {
		aydURL, err := url.Parse(rawURL)
		if err != nil {
			logger.Failure(fmt.Sprintf("environment variable `ayd_url` is invalid: %s", err), extra)
			return
		}
		pageURL, err := aydURL.Parse("status.html")
		if err != nil {
			logger.Failure(fmt.Sprintf("failed to generate status page URL: %s", err), extra)
			return
		}
		pageURL.Fragment = uuid.NewSHA1(uuid.NameSpaceURL, []byte(record.Target.String())).String()
		statusPage = pageURL.String()
	}

	ctx := Context{
		StatusPage: statusPage,
		Target:     record.Target.String(),
		Status:     record.Status.String(),
		CheckedAt:  record.Time.Format(time.RFC3339),
		Message:    record.Message,
		Extra:      record.ReadableExtra(),
	}

	if record.Status == ayd.StatusHealthy {
		ctx.Status = "RESOLVED"
	}

	text := texttemplate.Must(texttemplate.New("mail.txt").Parse(textTemplate))
	html := htmltemplate.Must(htmltemplate.New("mail.html").Parse(htmlTemplate))

	msg := gomail.NewMsg()
	msg.FromFormat(conf.From.Name, conf.From.Address)
	for _, t := range to {
		msg.AddToFormat(t.Name, t.Address)
	}
	msg.SetGenHeader("Subject", fmt.Sprintf("[%s] %s", ctx.Status, ctx.Target))
	msg.SetDateWithValue(record.Time)
	msg.SetUserAgent(fmt.Sprintf("ayd-mailto-alert/%s", version))

	if err := msg.AddAlternativeTextTemplate(text, ctx, gomail.WithPartCharset(gomail.CharsetUTF8)); err != nil {
		logger.Failure(fmt.Sprintf("failed to set text body: %s", err), extra)
		return
	}

	if err := msg.AddAlternativeHTMLTemplate(html, ctx, gomail.WithPartCharset(gomail.CharsetUTF8)); err != nil {
		logger.Failure(fmt.Sprintf("failed to set HTML body: %s", err), extra)
		return
	}

	client, err := gomail.NewClient(
		conf.Host,
		gomail.WithPort(conf.Port),
		gomail.WithSSLPort(true),
		gomail.WithUsername(conf.Username),
		gomail.WithPassword(conf.Password),
	)
	if err != nil {
		logger.Failure(fmt.Sprintf("failed to initialize SMTP client: %s", err), extra)
		return
	}
	defer client.Close()

	if conf.SSL {
		client.SetTLSPolicy(gomail.TLSMandatory)
	} else {
		client.SetTLSPolicy(gomail.TLSOpportunistic)
	}

	client.SetSMTPAuth(gomail.SMTPAuthPlain)

	if err := client.DialAndSend(msg); err != nil {
		logger.Failure(fmt.Sprintf("failed to send e-mail: %s", err), extra)
		return
	}

	logger.Healthy(fmt.Sprintf("sent alert to %s", to), extra)
}
