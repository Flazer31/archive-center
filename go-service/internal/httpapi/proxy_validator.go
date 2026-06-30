package httpapi

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

var (
	blockedLocalHosts = map[string]struct{}{
		"localhost":             {},
		"localhost.localdomain": {},
		"ip6-localhost":         {},
		"ip6-loopback":          {},
	}
	blockedMetadataHosts = map[string]struct{}{
		"metadata":                   {},
		"metadata.google.internal":   {},
		"metadata.aws":               {},
		"metadata.azure":             {},
		"instance-data":              {},
		"instance-data.ec2.internal": {},
	}
	authHeaderRegex = regexp.MustCompile(`(?i)Authorization\s*:\s*Bearer\s+\S+`)
	bearerRegex     = regexp.MustCompile(`(?i)\bBearer\s+\S+`)
)

// ValidateProxyEndpoint returns an error if the endpoint URL is disallowed.
// This replicates the 0.8 _proxy_validate_endpoint behavior.
func ValidateProxyEndpoint(endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme")
	}
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return fmt.Errorf("invalid endpoint URL")
	}
	if _, ok := blockedLocalHosts[hostname]; ok {
		return fmt.Errorf("endpoint host is not allowed")
	}
	if strings.HasPrefix(hostname, "localhost.") {
		return fmt.Errorf("endpoint host is not allowed")
	}
	if _, ok := blockedMetadataHosts[hostname]; ok {
		return fmt.Errorf("endpoint host is not allowed")
	}
	if addr, err := netip.ParseAddr(hostname); err == nil {
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
			addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
			return fmt.Errorf("endpoint host is not allowed")
		}
		// IPv4 reserved (Class E, 240.0.0.0/4 excluding broadcast)
		if addr.Is4() {
			b := addr.As4()
			if b[0] >= 240 && b[0] < 255 {
				return fmt.Errorf("endpoint host is not allowed")
			}
		}
	}
	return nil
}

// RedactSecrets scrubs API keys and bearer tokens from error detail strings.
func RedactSecrets(detail string, apiKey string) string {
	text := detail
	if apiKey != "" {
		text = strings.ReplaceAll(text, apiKey, "***")
	}
	text = authHeaderRegex.ReplaceAllString(text, "Authorization: Bearer ***")
	text = bearerRegex.ReplaceAllString(text, "Bearer ***")
	return text
}
