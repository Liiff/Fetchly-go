// Package safehttp menyediakan http.Client yang tidak bisa dipakai untuk SSRF:
// koneksi hanya ke IP publik (dicek saat dial, jadi kebal DNS rebinding) dan
// setiap redirect divalidasi ulang.
package safehttp

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

var (
	ErrBlockedAddress  = errors.New("alamat tujuan tidak diizinkan")
	ErrBlockedRedirect = errors.New("redirect ke tujuan yang tidak diizinkan")
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // CGNAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// IsPublicAddr true hanya untuk alamat unicast global yang bukan privat/loopback/link-local.
func IsPublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// controlPublicOnly dipanggil Go setelah DNS di-resolve, tepat sebelum connect().
func controlPublicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if !IsPublicAddr(addr) {
		return ErrBlockedAddress
	}
	return nil
}

// New membuat client aman. timeout 0 berarti tanpa batas total (andalkan context),
// allow memvalidasi host setiap redirect.
func New(timeout time.Duration, allow func(*url.URL) bool) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   controlPublicOnly,
	}
	transport := &http.Transport{
		Proxy:                 nil, // jangan pakai proxy dari environment
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConns:          20,
		IdleConnTimeout:       60 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("terlalu banyak redirect")
			}
			if req.URL.Scheme != "https" || !allow(req.URL) {
				return ErrBlockedRedirect
			}
			return nil
		},
	}
}
