package links

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// privateRanges are the IPv4 CIDR blocks that should not be reachable
// via link validation. This covers loopback, RFC 1918 private networks,
// link-local, and the cloud metadata endpoint range.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::/128",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
		"2001:db8::/32",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, block)
	}
}

// isPrivateIP reports whether ip falls within a private or reserved range.
func isPrivateIP(ip net.IP) bool {
	for _, block := range privateRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// safeTransport returns an *http.Transport whose DialContext resolves the
// target hostname and refuses to connect if the resolved IP is private.
// This prevents SSRF when following URLs extracted from untrusted skill content.
func safeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isPrivateIP(ip.IP) {
					return nil, fmt.Errorf("link validation blocked request to private address %s (%s)", ip.IP, host)
				}
			}
			// Connect to the first resolved address.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
}

// newHTTPClient creates the HTTP client used for link checking. It is a
// package-level variable so tests can replace it with a client that permits
// loopback connections to httptest servers.
var newHTTPClient = func() *http.Client {
	return &http.Client{
		Transport: safeTransport(),
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			scheme := req.URL.Scheme
			if scheme != "http" && scheme != "https" {
				return fmt.Errorf("refusing redirect to non-http(s) scheme %q", scheme)
			}
			return nil
		},
	}
}
