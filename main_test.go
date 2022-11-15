package main_test

import (
	"fmt"
	"net/mail"
	"strings"
	"testing"

	"github.com/macrat/ayd-mailto-alert"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		Input string
		Host  string
		Port  int
		SSL   bool
		Error string
	}{
		{
			Input: "smtps://smtp.gmail.com:465",
			Host:  "smtp.gmail.com",
			Port:  465,
			SSL:   true,
		},
		{
			Input: "smtp://example.com:25",
			Host:  "example.com",
			Port:  25,
			SSL:   false,
		},
		{
			Input: "smtps://smtp.gmail.com",
			Host:  "smtp.gmail.com",
			Port:  465,
			SSL:   true,
		},
		{
			Input: "smtp://example.com",
			Host:  "example.com",
			Port:  25,
			SSL:   false,
		},
		{
			Input: "smtps://:465",
			Error: "address :465: missing host in address",
		},
		{
			Input: "http://example.com",
			Error: "unsupported protocol: 'http'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Input, func(t *testing.T) {
			host, port, ssl, err := main.ParseURL(tt.Input)

			if err != nil {
				if tt.Error == "" {
					t.Fatalf("unexpected error: %s", err)
				} else if err.Error() != tt.Error {
					t.Fatalf("expected error %#v but got %#v", tt.Error, err.Error())
				}
			} else if tt.Error != "" {
				t.Fatal("expected error but got nil")
			}

			if host != tt.Host {
				t.Errorf("expected host is %#v but got %#v", tt.Host, host)
			}

			if port != tt.Port {
				t.Errorf("expected port is %#v but got %#v", tt.Port, port)
			}

			if ssl != tt.SSL {
				t.Errorf("expected ssl is %#v but got %#v", tt.SSL, ssl)
			}
		})
	}
}

func TestLoadConfigByPath(t *testing.T) {
	tests := []struct {
		Envs   map[string]string
		Expect main.Config
		Error  string
	}{
		{
			nil,
			main.Config{},
			"SMTP_SERVER is required",
		},
		{
			map[string]string{
				"SMTP_SERVER": "example.com:25",
			},
			main.Config{},
			"SMTP_USERNAME is required",
		},
		{
			map[string]string{
				"SMTP_SERVER":   "example.com:25",
				"SMTP_USERNAME": "foo",
			},
			main.Config{},
			"SMTP_PASSWORD is required",
		},
		{
			map[string]string{
				"SMTP_SERVER":   "example.com:25",
				"SMTP_USERNAME": "foo",
				"SMTP_PASSWORD": "bar",
			},
			main.Config{
				Host:     "example.com",
				Port:     25,
				SSL:      true,
				Username: "foo",
				Password: "bar",
				From:     &mail.Address{Name: "Ayd? Alert", Address: "ayd@localhost"},
			},
			"",
		},
		{
			map[string]string{
				"SMTP_SERVER":   "example.com:25",
				"SMTP_USERNAME": "foo",
				"SMTP_PASSWORD": "bar",
				"AYD_MAIL_FROM": "baz",
			},
			main.Config{},
			"environment variable `ayd_mail_from` is invalid: mail: missing '@' or angle-addr",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			for k, v := range tt.Envs {
				t.Setenv(k, v)
			}

			c, err := main.LoadConfigByPath(nil)
			if tt.Error != "" {
				if err == nil || err.Error() != tt.Error {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			if c.Host != tt.Expect.Host {
				t.Errorf("expected host %#v but got %#v", tt.Expect.Host, c.Host)
			}

			if c.Port != tt.Expect.Port {
				t.Errorf("expected port %#v but got %#v", tt.Expect.Port, c.Port)
			}

			if c.SSL != tt.Expect.SSL {
				t.Errorf("expected ssl %#v but got %#v", tt.Expect.SSL, c.SSL)
			}

			if c.Username != tt.Expect.Username {
				t.Errorf("expected username %#v but got %#v", tt.Expect.Username, c.Username)
			}

			if c.Password != tt.Expect.Password {
				t.Errorf("expected password %#v but got %#v", tt.Expect.Password, c.Password)
			}

			if c.From == nil || c.From.String() != tt.Expect.From.String() {
				t.Errorf("expected from %#v but got %#v", tt.Expect.From, c.From)
			}
		})
	}
}

func TestConfig_LoadFile(t *testing.T) {
	tests := []struct {
		Input  []string
		Expect main.Config
	}{
		{
			[]string{
				`set smtp=smtps://smtp.gmail.com:465`,
				`set smtp-auth-user=hello`,
				`set smtp-auth-password=world`,
				`set from="me <me@example.com>"`,
			},
			main.Config{
				Host:     "smtp.gmail.com",
				Port:     465,
				SSL:      true,
				Username: "hello",
				Password: "world",
				From:     &mail.Address{Name: "me", Address: "me@example.com"},
			},
		},
		{
			[]string{
				`# comment`,
				`set unknown=option`,
				`alias hoge fuga`,
				`set smtp=smtp://example.com/foo/bar smtp-auth-user="it's me" smtp-auth-password="secret"`,
				`set from=me<me@example.com>  # comment`,
			},
			main.Config{
				Host:     "example.com",
				Port:     25,
				SSL:      false,
				Username: "it's me",
				Password: "secret",
				From:     &mail.Address{Name: "me", Address: "me@example.com"},
			},
		},
	}

	for i, tt := range tests {
		var c main.Config

		if err := c.LoadFile(strings.NewReader(strings.Join(tt.Input, "\n"))); err != nil {
			t.Errorf("%d: failed to parse: %s", i, err)
			continue
		}

		if c.Host != tt.Expect.Host {
			t.Errorf("%d: expected host %#v but got %#v", i, tt.Expect.Host, c.Host)
		}

		if c.Port != tt.Expect.Port {
			t.Errorf("%d: expected port %#v but got %#v", i, tt.Expect.Port, c.Port)
		}

		if c.SSL != tt.Expect.SSL {
			t.Errorf("%d: expected ssl %#v but got %#v", i, tt.Expect.SSL, c.SSL)
		}

		if c.Username != tt.Expect.Username {
			t.Errorf("%d: expected username %#v but got %#v", i, tt.Expect.Username, c.Username)
		}

		if c.Password != tt.Expect.Password {
			t.Errorf("%d: expected password %#v but got %#v", i, tt.Expect.Password, c.Password)
		}

		if c.From == nil || c.From.String() != tt.Expect.From.String() {
			t.Errorf("%d: expected from %#v but got %#v", i, tt.Expect.From, c.From)
		}
	}
}
