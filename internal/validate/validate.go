// Package validate berisi validasi input dari client: URL platform, URL CDN, dan nama file.
package validate

import (
	"errors"
	"net/url"
	"path"
	"strings"
)

// Platform adalah platform sumber media yang didukung.
type Platform string

const (
	YouTube   Platform = "youtube"
	TikTok    Platform = "tiktok"
	Instagram Platform = "instagram"
)

var (
	ErrInvalidURL     = errors.New("URL tidak valid")
	ErrHostNotAllowed = errors.New("host tidak diizinkan")
)

// URL CDN TikTok/Instagram yang ditandatangani bisa sangat panjang.
const maxURLLength = 8192

var platformHosts = map[Platform][]string{
	YouTube:   {"youtube.com", "youtu.be"},
	TikTok:    {"tiktok.com"},
	Instagram: {"instagram.com"},
}

// cdnSuffixes adalah domain CDN yang boleh diakses lewat /api/download?proxy=1.
// Tambahkan lewat AddCDNSuffixes (mis. dari env EXTRA_CDN_HOSTS) bila ada CDN baru.
var cdnSuffixes = []string{
	"cdninstagram.com", "fbcdn.net", "instagram.com",
	"tiktokcdn.com", "tiktokcdn-us.com", "tiktokv.com", "tiktokv.us", "tiktok.com",
	"byteoversea.com", "ibytedtos.com", "ibyteimg.com", "muscdn.com",
}

// AddCDNSuffixes menambah domain CDN yang diizinkan. Panggil hanya saat startup.
func AddCDNSuffixes(suffixes ...string) {
	for _, s := range suffixes {
		s = strings.ToLower(strings.Trim(strings.TrimSpace(s), "."))
		if s != "" {
			cdnSuffixes = append(cdnSuffixes, s)
		}
	}
}

// HostMatches true jika host sama persis dengan salah satu suffix, atau subdomain-nya.
// "youtube.com.evil.net" TIDAK cocok dengan "youtube.com".
func HostMatches(host string, suffixes ...string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, s := range suffixes {
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}

func parseHTTPURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLLength {
		return nil, ErrInvalidURL
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if u.User != nil || u.Hostname() == "" {
		return nil, ErrInvalidURL
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return nil, ErrInvalidURL
	}
	return u, nil
}

// ParsePlatformURL memvalidasi URL halaman media dan mendeteksi platformnya.
func ParsePlatformURL(raw string) (*url.URL, Platform, error) {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return nil, "", err
	}
	for platform, hosts := range platformHosts {
		if HostMatches(u.Hostname(), hosts...) {
			return u, platform, nil
		}
	}
	return nil, "", ErrHostNotAllowed
}

// CDNAllowed true jika host URL termasuk allowlist CDN.
func CDNAllowed(u *url.URL) bool {
	return HostMatches(u.Hostname(), cdnSuffixes...)
}

// ParseCDNURL memvalidasi URL file media langsung: wajib https dan host dalam allowlist.
func ParseCDNURL(raw string) (*url.URL, error) {
	u, err := parseHTTPURL(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" {
		return nil, ErrInvalidURL
	}
	if !CDNAllowed(u) {
		return nil, ErrHostNotAllowed
	}
	return u, nil
}

var allowedExt = map[string]bool{
	".mp4": true, ".m4a": true, ".mp3": true, ".webm": true,
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

// splitName membersihkan nama file: hanya ASCII aman, tanpa path, panjang dibatasi.
func splitName(name string) (base, ext string) {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('_')
		}
	}
	clean := b.String()
	ext = strings.ToLower(path.Ext(clean))
	base = strings.Trim(strings.TrimSuffix(clean, path.Ext(clean)), ".")
	if len(base) > 80 {
		base = base[:80]
	}
	if base == "" {
		base = "fetchly_download"
	}
	return base, ext
}

// Filename membersihkan nama file dari client. Ekstensi dipertahankan bila ada di
// allowlist, selain itu diganti defaultExt (contoh: ".mp4").
func Filename(name, defaultExt string) string {
	base, ext := splitName(name)
	if !allowedExt[ext] {
		ext = defaultExt
	}
	return base + ext
}

// FilenameWithExt membersihkan nama file dan memaksa ekstensi tertentu (contoh: ".m4a").
func FilenameWithExt(name, ext string) string {
	base, _ := splitName(name)
	return base + ext
}
