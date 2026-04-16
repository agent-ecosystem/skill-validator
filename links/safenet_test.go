package links

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		// IPv4 private/reserved ranges
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"169.254.169.254", true}, // cloud metadata
		{"169.254.0.1", true},

		// IPv6 private/reserved ranges
		{"::1", true},                       // IPv6 loopback
		{"fc00::1", true},                   // IPv6 unique local
		{"fe80::1", true},                   // IPv6 link-local
		{"8.8.8.8", false},                  // Google DNS
		{"93.184.216.34", false},            // example.com
		{"172.32.0.1", false},               // just outside 172.16/12
		{"192.169.0.1", false},              // just outside 192.168/16
		{"2607:f8b0:4004:800::200e", false}, // Google public IPv6
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %s", tt.ip)
			}
			got := isPrivateIP(ip)
			if got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestSafeTransportBlocksPrivateIPs(t *testing.T) {
	// CheckLinks with the default safe client should block links to private IPs.
	// We don't need a server running; the dialer should refuse before connecting.
	dir := t.TempDir()
	body := "[metadata](http://169.254.169.254/latest/meta-data/)"
	results := CheckLinks(t.Context(), dir, body, Options{})
	if len(results) == 0 {
		t.Fatal("expected a result for blocked private IP link")
	}
	r := results[0]
	if r.Message == "" {
		t.Fatal("expected non-empty message")
	}
	requireContains(t, r.Message, "request failed")
}

func TestSafeTransportBlocksLocalhost(t *testing.T) {
	dir := t.TempDir()
	body := "[local](http://127.0.0.1:8080/admin)"
	results := CheckLinks(t.Context(), dir, body, Options{})
	if len(results) == 0 {
		t.Fatal("expected a result for blocked localhost link")
	}
	requireContains(t, results[0].Message, "request failed")
}
