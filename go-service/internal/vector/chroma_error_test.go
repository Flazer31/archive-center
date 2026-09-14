package vector

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type chromaErrorTransport func(*http.Request) (*http.Response, error)

func (f chromaErrorTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type chromaFailedReader struct{}

func (chromaFailedReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestChromaResponseReadFailurePreservesCause(t *testing.T) {
	for _, tc := range []struct {
		name, prefix string
		status       int
	}{
		{"empty successful response", "", 200},
		{"partial JSON", `{"ids":`, 200},
		{"valid JSON followed by read error", `{"ids":[]}`, 200},
		{"upstream error body interrupted", "collection unavailable", 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := &chromaStore{endpoint: "http://fixture.invalid", client: &http.Client{Transport: chromaErrorTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodGet || r.URL.Path != "/fixture" {
					t.Fatalf("unexpected external request: %s %s", r.Method, r.URL)
				}
				calls++
				return &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(io.MultiReader(strings.NewReader(tc.prefix), chromaFailedReader{}))}, nil
			})}}
			var out map[string]any
			status, err := s.doJSON(context.Background(), http.MethodGet, "/fixture", nil, &out, 200)
			if calls != 1 || status != tc.status || !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("read cause/status lost: calls=%d status=%d error=%v", calls, status, err)
			}
			if out != nil {
				t.Fatalf("failed read was decoded as successful data: %#v", out)
			}
			if tc.status == 500 && !strings.Contains(err.Error(), tc.prefix) {
				t.Fatalf("upstream failure detail lost: %v", err)
			}
		})
	}
}
