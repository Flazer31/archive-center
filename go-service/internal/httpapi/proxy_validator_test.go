package httpapi

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Endpoint validation
// ---------------------------------------------------------------------------

func TestValidateProxyEndpointEmpty(t *testing.T) {
	if err := ValidateProxyEndpoint(""); err == nil {
		t.Error("expected error for empty endpoint")
	}
}

func TestValidateProxyEndpointInvalidURL(t *testing.T) {
	if err := ValidateProxyEndpoint("://not-a-url"); err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestValidateProxyEndpointUnsupportedScheme(t *testing.T) {
	for _, scheme := range []string{"ftp", "file", "gopher", "data", "javascript"} {
		if err := ValidateProxyEndpoint(scheme + "://example.com"); err == nil {
			t.Errorf("expected error for scheme %s", scheme)
		}
	}
}

func TestValidateProxyEndpointBlockedLocalHosts(t *testing.T) {
	blocked := []string{
		"https://localhost/v1",
		"https://localhost.localdomain/v1",
		"https://ip6-localhost/v1",
		"https://ip6-loopback/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		} else if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("expected 'not allowed' error for %s, got %v", url, err)
		}
	}
}

func TestValidateProxyEndpointLocalhostPrefix(t *testing.T) {
	blocked := []string{
		"https://localhost.foo.com/v1",
		"https://localhost.internal/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointBlockedMetadata(t *testing.T) {
	blocked := []string{
		"https://metadata/v1",
		"https://metadata.google.internal/v1",
		"https://metadata.aws/v1",
		"https://metadata.azure/v1",
		"https://instance-data/v1",
		"https://instance-data.ec2.internal/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointLoopbackIP(t *testing.T) {
	blocked := []string{
		"https://127.0.0.1/v1",
		"https://[::1]/v1",
		"https://127.255.255.255/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointPrivateIP(t *testing.T) {
	blocked := []string{
		"https://10.0.0.1/v1",
		"https://192.168.1.1/v1",
		"https://172.16.0.1/v1",
		"https://[fc00::1]/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointLinkLocalIP(t *testing.T) {
	blocked := []string{
		"https://169.254.1.1/v1",
		"https://[fe80::1]/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointMulticastIP(t *testing.T) {
	blocked := []string{
		"https://224.0.0.1/v1",
		"https://[ff02::1]/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointUnspecifiedIP(t *testing.T) {
	blocked := []string{
		"https://0.0.0.0/v1",
		"https://[::]/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointReservedIPv4(t *testing.T) {
	// Class E (240.0.0.0/4 excluding 255.255.255.255)
	blocked := []string{
		"https://240.0.0.1/v1",
		"https://250.0.0.1/v1",
	}
	for _, url := range blocked {
		if err := ValidateProxyEndpoint(url); err == nil {
			t.Errorf("expected error for %s", url)
		}
	}
}

func TestValidateProxyEndpointValid(t *testing.T) {
	valid := []string{
		"https://api.openai.com/v1",
		"https://generativelanguage.googleapis.com/v1beta",
		"https://api.githubcopilot.com",
		"https://openrouter.ai/api",
		"https://bigmodel.cn/v1",
		"https://example.com:8080/v1",
	}
	for _, url := range valid {
		if err := ValidateProxyEndpoint(url); err != nil {
			t.Errorf("unexpected error for %s: %v", url, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Secret redaction
// ---------------------------------------------------------------------------

func TestRedactSecretsAPIKeyLiteral(t *testing.T) {
	detail := "request failed with key sk-abc123"
	got := RedactSecrets(detail, "sk-abc123")
	if strings.Contains(got, "sk-abc123") {
		t.Errorf("expected api key to be redacted, got %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Errorf("expected *** placeholder, got %q", got)
	}
}

func TestRedactSecretsAuthorizationHeader(t *testing.T) {
	detail := "trace: Authorization: Bearer abc123"
	got := RedactSecrets(detail, "")
	want := "trace: Authorization: Bearer ***"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRedactSecretsBearerStandalone(t *testing.T) {
	detail := "token was Bearer abc123 in header"
	got := RedactSecrets(detail, "")
	want := "token was Bearer *** in header"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRedactSecretsEmptyDetail(t *testing.T) {
	got := RedactSecrets("", "sk-abc")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRedactSecretsEmptyAPIKey(t *testing.T) {
	detail := "some error"
	got := RedactSecrets(detail, "")
	if got != detail {
		t.Errorf("expected %q, got %q", detail, got)
	}
}

func TestRedactSecretsMultipleOccurrences(t *testing.T) {
	detail := "key sk-abc and again sk-abc"
	got := RedactSecrets(detail, "sk-abc")
	if strings.Contains(got, "sk-abc") {
		t.Errorf("expected all occurrences redacted, got %q", got)
	}
}
