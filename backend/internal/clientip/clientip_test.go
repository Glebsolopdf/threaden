package clientip

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	long := strings.Repeat("1.1.1.1,", 600)
	for _, test := range []struct {
		name, remote, forwarded string
		trusted                 []string
		want                    string
	}{
		{"direct ignores spoof", "198.51.100.10:1234", "1.1.1.1", nil, "198.51.100.10"},
		{"trusted one client", "127.0.0.1:80", "198.51.100.10", []string{"127.0.0.1"}, "198.51.100.10"},
		{"trusted chain", "10.0.0.2:80", "198.51.100.10, 10.0.0.1", []string{"10.0.0.0/24"}, "198.51.100.10"},
		{"untrusted intermediary", "10.0.0.2:80", "198.51.100.10, 203.0.113.2", []string{"10.0.0.0/24"}, "203.0.113.2"},
		{"forged left address", "127.0.0.1:80", "1.1.1.1, 198.51.100.10", []string{"127.0.0.1"}, "198.51.100.10"},
		{"invalid chain", "127.0.0.1:80", "198.51.100.10, nonsense", []string{"127.0.0.1"}, "127.0.0.1"},
		{"empty chain", "127.0.0.1:80", "", []string{"127.0.0.1"}, "127.0.0.1"},
		{"ipv4", "127.0.0.1:80", "192.0.2.1", []string{"127.0.0.1"}, "192.0.2.1"},
		{"ipv6", "[::1]:80", "2001:db8::1", []string{"::1"}, "2001:db8::1"},
		{"ipv6 peer port", "[2001:db8::2]:443", "2001:db8::1", []string{"2001:db8::2"}, "2001:db8::1"},
		{"single proxy IP", "10.2.3.4:80", "198.51.100.10", []string{"10.2.3.4"}, "198.51.100.10"},
		{"proxy CIDR", "10.2.3.4:80", "198.51.100.10", []string{"10.2.0.0/16"}, "198.51.100.10"},
		{"untrusted peer", "10.2.3.4:80", "198.51.100.10", []string{"10.3.0.0/16"}, "10.2.3.4"},
		{"no trusted proxies", "10.2.3.4:80", "198.51.100.10", nil, "10.2.3.4"},
		{"oversized chain", "127.0.0.1:80", long, []string{"127.0.0.1"}, "127.0.0.1"},
		{"too many addresses", "127.0.0.1:80", strings.TrimSuffix(strings.Repeat("192.0.2.1,", 33), ","), []string{"127.0.0.1"}, "127.0.0.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Resolve(test.remote, test.forwarded, test.trusted); got != test.want {
				t.Fatalf("Resolve() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSpoofedForwardedValueDoesNotChangeDirectKey(t *testing.T) {
	base := Resolve("198.51.100.10:1234", "1.1.1.1", nil)
	if changed := Resolve("198.51.100.10:1234", "2.2.2.2", nil); changed != base {
		t.Fatalf("direct client key changed from %q to %q", base, changed)
	}
}
