package main_test

import (
	"testing"

	"github.com/macrat/ayd-mail-alert"
)

func TestParseSMTPServer(t *testing.T) {
	tests := []struct {
		Input string
		Host  string
		Port  int
		Error string
	}{
		{
			Input: "smtp.gmail.com:465",
			Host:  "smtp.gmail.com",
			Port:  465,
		},
		{
			Input: "smtp.gmail.com",
			Error: "address smtp.gmail.com: missing port in address",
		},
		{
			Input: ":465",
			Error: "address :465: missing host in address",
		},
		{
			Input: "",
			Error: "missing port in address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.Input, func(t *testing.T) {
			host, port, err := main.ParseSMTPServer(tt.Input)

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
		})
	}
}
