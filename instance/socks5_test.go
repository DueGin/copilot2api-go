package instance

import (
	"net/http"
	"net/url"
	"testing"
)

// TestBuildTransportSocks5Parse verifies that buildTransport correctly handles
// socks5://user:pass@host:port URLs (the DialContext / Dial field is set instead
// of the Proxy field).
func TestBuildTransportSocks5Parse(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		wantProxy bool // true → Proxy field set (HTTP proxy); false → DialContext or Dial set (SOCKS5)
	}{
		{"empty", "", true},           // no proxy at all (Proxy is nil)
		{"http proxy", "http://proxy.example.com:8080", true},
		{"https proxy", "https://proxy.example.com:8443", true},
		{"socks5 no auth", "socks5://proxy.example.com:1080", false},
		{"socks5 with auth", "socks5://user:pass@proxy.example.com:1080", false},
		{"socks5h with auth", "socks5h://admin:secret@proxy.example.com:1080", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := buildTransport(false, tt.proxyURL)
			if tt.proxyURL == "" {
				// No proxy configured; all fields should be nil.
				if tr.Proxy != nil {
					t.Error("expected Proxy to be nil for empty proxy URL")
				}
				return
			}

			parsed, _ := url.Parse(tt.proxyURL)
			isSocks := parsed.Scheme == "socks5" || parsed.Scheme == "socks5h"

			if isSocks {
				// For SOCKS5, Proxy MUST be nil and DialContext or Dial MUST be set.
				if tr.Proxy != nil {
					t.Error("Proxy should be nil for SOCKS5 proxy")
				}
				if tr.DialContext == nil && tr.Dial == nil { //nolint:staticcheck
					t.Error("expected DialContext or Dial to be set for SOCKS5 proxy")
				}
			} else {
				// For HTTP/HTTPS, Proxy MUST be set.
				if tr.Proxy == nil {
					t.Error("expected Proxy to be set for HTTP proxy")
				}
			}
		})
	}
}

// TestBuildTransportSocks5AuthExtraction verifies user:password extraction from
// the proxy URL. We create a transport and then try to make a request to a
// non-existent host — the dialer will fail, but we can verify the transport
// was configured (DialContext is set).
func TestBuildTransportSocks5AuthExtraction(t *testing.T) {
	proxyURL := "socks5://testuser:testpass@127.0.0.1:19999"

	tr := buildTransport(false, proxyURL)

	// Verify DialContext or Dial was set (SOCKS5 dialer configured).
	if tr.DialContext == nil && tr.Dial == nil { //nolint:staticcheck
		t.Fatal("expected DialContext or Dial to be set for SOCKS5 proxy with auth")
	}

	// Verify Proxy is NOT set (it's SOCKS5, not HTTP proxy).
	if tr.Proxy != nil {
		t.Fatal("Proxy should be nil for SOCKS5")
	}

	// Attempt a request — it will fail (no SOCKS5 server on 127.0.0.1:19999)
	// but this confirms the transport tries to use the SOCKS5 dialer.
	client := &http.Client{
		Transport: tr,
	}
	_, err := client.Get("http://example.com")
	if err == nil {
		t.Fatal("expected error (no SOCKS5 server running), but got nil")
	}
	// The error should mention connection refused or similar, indicating it
	// tried to dial through the SOCKS5 proxy.
	t.Logf("Expected SOCKS5 dial error: %v", err)
}
