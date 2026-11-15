package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"
)

func TestTransportDiagnosticsKeepCauseAndClassifyFailures(t *testing.T) {
	certificate := &x509.Certificate{DNSNames: []string{"another.example"}}
	tests := []struct {
		name, kind string
		cause      error
	}{
		{"dns", "DNS lookup", &net.DNSError{Name: "tokenrank.demonh.site", Err: "no such host"}},
		{"tcp", "TCP connection", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}},
		{"timeout", "timeout", context.DeadlineExceeded},
		{"proxy", "proxy connection", &net.OpError{Op: "proxyconnect", Net: "tcp", Err: errors.New("proxy refused connection")}},
		{"tls", "TLS certificate verification", &tls.CertificateVerificationError{UnverifiedCertificates: []*x509.Certificate{certificate}, Err: x509.HostnameError{Certificate: certificate, Host: "tokenrank.demonh.site"}}},
		{"other", "network request", errors.New("synthetic transport failure")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := http.DefaultTransport
			http.DefaultTransport = transport(func(*http.Request) (*http.Response, error) { return nil, tt.cause })
			t.Cleanup(func() { http.DefaultTransport = old })
			_, err := call(http.MethodPost, "/api/import", "synthetic-secret", []byte(`{"records":[]}`))
			if err == nil {
				t.Fatal("transport failure was hidden")
			}
			if !strings.Contains(err.Error(), tt.kind) || !strings.Contains(err.Error(), tt.cause.Error()) {
				t.Fatalf("missing precise cause: %v", err)
			}
			if tt.name != "tls" && strings.Contains(err.Error(), "TLS") {
				t.Fatalf("non-TLS failure misdiagnosed: %v", err)
			}
			if tt.name == "dns" {
				var dns *net.DNSError
				if !errors.As(err, &dns) {
					t.Fatal("DNS cause type lost")
				}
			}
			if tt.name == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("deadline sentinel lost")
			}
			if !strings.Contains(err.Error(), "pending batch retained") {
				t.Fatal("upload recovery guidance missing")
			}
		})
	}
}

func TestCredentialsAreRedactedFromTransportErrorChain(t *testing.T) {
	token, ticket, previous := "SYNTHETIC_TOKEN_PRIVATE", "SYNTHETIC_TICKET_PRIVATE", "SYNTHETIC_PREVIOUS_PRIVATE"
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("proxy response reflected %s %s %s", token, ticket, previous)
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	_, err := call(http.MethodPost, "/api/enroll", token, []byte(fmt.Sprintf(`{"ticket":%q,"previousToken":%q}`, ticket, previous)))
	for current := err; current != nil; current = errors.Unwrap(current) {
		for _, secret := range []string{token, ticket, previous} {
			if strings.Contains(current.Error(), secret) {
				t.Fatal("credential leaked into error chain")
			}
		}
	}
	if strings.Contains(err.Error(), "pending batch") {
		t.Fatalf("setup failure mentions nonexistent batch: %v", err)
	}
}

func TestSetupHTTPErrorHasStageSpecificGuidanceAndRedactsBeforeTruncation(t *testing.T) {
	ticket := strings.Repeat("private-ticket-", 6)
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(*http.Request) (*http.Response, error) {
		body := fmt.Sprintf(`{"error":%q}`, strings.Repeat("x", 380)+ticket)
		return &http.Response{StatusCode: 401, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	_, err := call(http.MethodPost, "/api/enroll", "", []byte(fmt.Sprintf(`{"ticket":%q}`, ticket)))
	if err == nil || !strings.Contains(err.Error(), "setup returned HTTP 401") {
		t.Fatalf("wrong setup diagnostic: %v", err)
	}
	if strings.Contains(err.Error(), "pending batch") || strings.Contains(err.Error(), "private-ticket") {
		t.Fatalf("misleading or sensitive setup error: %v", err)
	}
}
