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
		{"0.0.0.0", true},
		{"0.255.255.255", true},
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.0.0.5", true},
		{"192.0.2.10", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"198.18.0.1", true},
		{"198.19.255.255", true},
		{"198.51.100.7", true},
		{"203.0.113.7", true},
		{"169.254.169.254", true},
		{"169.254.0.1", true},
		{"224.0.0.1", true},
		{"239.255.255.255", true},
		{"240.0.0.1", true},
		{"255.255.255.255", true},

		{"::", true},
		{"::1", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"ff02::1", true},
		{"2001:db8::1", true},

		{"::ffff:127.0.0.1", true},
		{"::ffff:0.0.0.0", true},
		{"::ffff:169.254.169.254", true},

		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"100.63.255.255", false},
		{"100.128.0.1", false},
		{"172.32.0.1", false},
		{"192.169.0.1", false},
		{"198.17.255.255", false},
		{"198.20.0.1", false},
		{"2607:f8b0:4004:800::200e", false},
		{"::ffff:8.8.8.8", false},
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
	results := CheckLinks(t.Context(), dir, body)
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
	results := CheckLinks(t.Context(), dir, body)
	if len(results) == 0 {
		t.Fatal("expected a result for blocked localhost link")
	}
	requireContains(t, results[0].Message, "request failed")
}
