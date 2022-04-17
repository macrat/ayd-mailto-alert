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

	gomail "github.com/go-mail/mail"
	"github.com/google/shlex"
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
	conf := Config{SSL: true, From: &mail.Address{"Ayd? Alert", "ayd@localhost"}}

	pathes := []string{"/usr/share/misc/mail.rc", "/usr/local/etc/mail.rc", "/etc/mail.rc"}
	if home, err := os.UserHomeDir(); err == nil {
		pathes = append(pathes, filepath.Join(home, ".mailrc"))
	}

	for _, p := range pathes {
		if f, err := os.Open(p); err == nil {
			fmt.Println("load", p, pathes)
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
}

func Usage() {
	fmt.Fprintln(os.Stderr, "Usage: ayd-mailto-alert MAILTO_URL CHECKED_AT TARGET_STATUS LATENCY TARGET_URL MESSAGE")
}

func main() {
	showVersion := flag.Bool("v", false, "show version and exit.")
	flag.Usage = Usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("ayd-mailto-alert %s (%s)\n", version, commit)
		return
	}

	args, err := ayd.ParseAlertPluginArgs()
	if err != nil {
		Usage()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	logger := ayd.NewLogger(&url.URL{
		Scheme: args.AlertURL.Scheme,
		Opaque: args.AlertURL.Opaque,
	})

	conf, err := LoadConfig()
	if err != nil {
		logger.Failure(err.Error())
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

	ctx := Context{
		StatusPage: statusPage.String(),
		Target:     args.TargetURL.String(),
		Status:     args.Status.String(),
		CheckedAt:  args.CheckedAt.Format(time.RFC3339),
		Message:    args.Message,
	}

	if args.Status == ayd.StatusHealthy {
		ctx.Status = "RESOLVED"
	}

	html := htmltemplate.Must(htmltemplate.New("mail.html").Parse(htmlTemplate))
	text := texttemplate.Must(texttemplate.New("mail.txt").Parse(textTemplate))

	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", conf.From.Address, conf.From.Name)
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

	dialer := gomail.NewDialer(conf.Host, conf.Port, conf.Username, conf.Password)
	if conf.SSL {
		dialer.StartTLSPolicy = gomail.MandatoryStartTLS
	} else {
		dialer.StartTLSPolicy = gomail.OpportunisticStartTLS
	}

	if err := dialer.DialAndSend(msg); err != nil {
		logger.Failure(fmt.Sprintf("failed to send e-mail: %s", err))
		return
	}

	logger.Healthy(fmt.Sprintf("sent alert to %s", to))
}
