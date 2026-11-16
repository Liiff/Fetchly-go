package safehttp

import (
	"net/netip"
	"testing"
)

func TestIsPublicAddr(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"10.0.0.5", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false}, // metadata cloud
		{"100.64.0.1", false},      // CGNAT
		{"0.0.0.0", false},
		{"224.0.0.1", false},
		{"fe80::1", false},
		{"fc00::1", false},
		{"::ffff:127.0.0.1", false}, // IPv4-mapped IPv6
		{"::ffff:10.0.0.1", false},
	}
	for _, tc := range cases {
		if got := IsPublicAddr(netip.MustParseAddr(tc.ip)); got != tc.want {
			t.Errorf("IsPublicAddr(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}
