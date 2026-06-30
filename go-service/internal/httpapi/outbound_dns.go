package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

func ConfigureOutboundDNSServers(raw string) bool {
	servers := normalizeDNSServers(raw)
	if len(servers) == 0 {
		return false
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: 5 * time.Second}
			var lastErr error
			for _, server := range servers {
				conn, err := dialer.DialContext(ctx, "udp", server)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr != nil {
				return nil, lastErr
			}
			return dialer.DialContext(ctx, network, address)
		},
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver:  resolver,
	}).DialContext

	client := &http.Client{Transport: transport}
	proxyHTTPClient = client
	http.DefaultTransport = transport
	http.DefaultClient = client
	return true
}

func normalizeDNSServers(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		server := normalizeDNSServer(strings.TrimSpace(part))
		if server != "" {
			out = append(out, server)
		}
	}
	return out
}

func normalizeDNSServer(raw string) string {
	if raw == "" {
		return ""
	}
	if host, port, err := net.SplitHostPort(raw); err == nil && host != "" && port != "" {
		return net.JoinHostPort(host, port)
	}
	if ip := net.ParseIP(raw); ip != nil {
		return net.JoinHostPort(raw, "53")
	}
	if strings.Contains(raw, ":") {
		return ""
	}
	return net.JoinHostPort(raw, "53")
}
