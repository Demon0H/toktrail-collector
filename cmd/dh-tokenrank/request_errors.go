package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
)

func requestStage(path string) string {
	switch strings.SplitN(path, "?", 2)[0] {
	case "/api/enroll":
		return "setup"
	case "/api/import":
		return "upload"
	case "/api/history":
		return "history recovery"
	case "/api/device-status":
		return "sync status"
	case "/api/me":
		return "authentication"
	default:
		return "request"
	}
}
func retryHint(path string) string {
	if requestStage(path) == "upload" {
		return "; pending batch retained; retry sync"
	}
	if requestStage(path) == "setup" {
		return "; existing local configuration preserved"
	}
	return "; local state preserved"
}
func requestSecrets(token string, body []byte) []string {
	secrets := []string{token}
	var values map[string]json.RawMessage
	if json.Unmarshal(body, &values) == nil {
		for _, key := range []string{"ticket", "previousToken", "token", "client_secret", "access_token"} {
			var value string
			if json.Unmarshal(values[key], &value) == nil && value != "" {
				secrets = append(secrets, value)
			}
		}
	}
	return secrets
}
func redact(message string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
			message = strings.ReplaceAll(message, url.QueryEscape(secret), "[redacted]")
		}
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, message)
}

// Preserve ordinary transport error types and wrapping for errors.Is/As, while
// making sure credential-bearing URLs or custom transport messages cannot leak
// through either the visible error or the underlying error chain.
func safeCause(err error, secrets []string) error {
	switch e := err.(type) {
	case *url.Error:
		copy := *e
		copy.URL = redact(e.URL, secrets)
		copy.Op = redact(e.Op, secrets)
		copy.Err = safeCause(e.Err, secrets)
		return &copy
	case *net.OpError:
		copy := *e
		copy.Op = redact(e.Op, secrets)
		copy.Net = redact(e.Net, secrets)
		copy.Err = safeCause(e.Err, secrets)
		return &copy
	case *net.DNSError:
		copy := *e
		copy.Err = redact(e.Err, secrets)
		copy.Name = redact(e.Name, secrets)
		copy.Server = redact(e.Server, secrets)
		copy.UnwrapErr = safeCause(e.UnwrapErr, secrets)
		return &copy
	}
	if err == nil {
		return nil
	}
	clean := redact(err.Error(), secrets)
	if clean != err.Error() {
		return errors.New(clean)
	}
	return err
}
func networkKind(err error) string {
	var operation *net.OpError
	if errors.As(err, &operation) && strings.Contains(operation.Op, "proxy") {
		return "proxy connection"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "DNS lookup"
	}
	var verification *tls.CertificateVerificationError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &verification) || errors.As(err, &authority) || errors.As(err, &hostname) || errors.As(err, &invalid) {
		return "TLS certificate verification"
	}
	var record tls.RecordHeaderError
	if errors.As(err, &record) {
		return "TLS handshake"
	}
	var timeout net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &timeout) && timeout.Timeout()) {
		return "timeout"
	}
	if operation != nil {
		if strings.HasPrefix(operation.Net, "tcp") {
			return "TCP connection"
		}
		return "network connection"
	}
	return "network request"
}
func requestFailure(path, phase string, err error, secrets []string) error {
	cause := safeCause(err, secrets)
	return fmt.Errorf("DH %s %s failed (%s): %w%s", requestStage(path), phase, networkKind(cause), cause, retryHint(path))
}
