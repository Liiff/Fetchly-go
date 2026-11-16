package validate

import "testing"

func TestHostMatches(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"youtube.com", true},
		{"www.youtube.com", true},
		{"M.YouTube.com", true},
		{"youtube.com.", true},
		{"youtube.com.evil.net", false},
		{"evilyoutube.com", false},
		{"notyoutube.com", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := HostMatches(tc.host, "youtube.com"); got != tc.want {
			t.Errorf("HostMatches(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestParsePlatformURL(t *testing.T) {
	ok := map[string]Platform{
		"https://www.youtube.com/watch?v=abc": YouTube,
		"https://youtu.be/abc":                YouTube,
		"https://vm.tiktok.com/xyz/":          TikTok,
		"https://www.instagram.com/p/abc/":    Instagram,
		"  https://youtu.be/abc  ":            YouTube,
	}
	for raw, want := range ok {
		_, got, err := ParsePlatformURL(raw)
		if err != nil || got != want {
			t.Errorf("ParsePlatformURL(%q) = %v, %v; want %v", raw, got, err, want)
		}
	}

	bad := []string{
		"",
		"youtube.com/watch?v=abc",
		"file:///etc/passwd",
		"ftp://youtube.com/x",
		"http://169.254.169.254/latest/meta-data/",
		"http://localhost:8080/",
		"https://youtube.com.evil.net/watch?v=abc",
		"https://user:pass@youtube.com/watch?v=abc",
		"https://youtube.com:8443/watch?v=abc",
		"--exec=calc",
	}
	for _, raw := range bad {
		if _, _, err := ParsePlatformURL(raw); err == nil {
			t.Errorf("ParsePlatformURL(%q) seharusnya ditolak", raw)
		}
	}
}

func TestParseCDNURL(t *testing.T) {
	good := []string{
		"https://scontent-sin6-1.cdninstagram.com/v/t51.2885-15/abc.jpg?x=1",
		"https://v16.tiktokcdn.com/video.mp4",
	}
	for _, raw := range good {
		if _, err := ParseCDNURL(raw); err != nil {
			t.Errorf("ParseCDNURL(%q) error: %v", raw, err)
		}
	}

	bad := []string{
		"http://scontent.cdninstagram.com/a.jpg", // bukan https
		"https://example.com/a.jpg",
		"https://cdninstagram.com.evil.net/a.jpg",
		"https://127.0.0.1/a.jpg",
		"https://[::1]/a.jpg",
		"https://169.254.169.254/latest/meta-data/",
		"https://scontent.cdninstagram.com:8080/a.jpg",
	}
	for _, raw := range bad {
		if _, err := ParseCDNURL(raw); err == nil {
			t.Errorf("ParseCDNURL(%q) seharusnya ditolak", raw)
		}
	}
}

func TestAddCDNSuffixes(t *testing.T) {
	if _, err := ParseCDNURL("https://media.example-cdn.test/a.mp4"); err == nil {
		t.Fatal("host belum ditambahkan, seharusnya ditolak")
	}
	AddCDNSuffixes(" Example-CDN.test ", "")
	if _, err := ParseCDNURL("https://media.example-cdn.test/a.mp4"); err != nil {
		t.Errorf("setelah AddCDNSuffixes seharusnya lolos: %v", err)
	}
}

func TestFilename(t *testing.T) {
	cases := []struct{ in, def, want string }{
		{"video_1.mp4", ".mp4", "video_1.mp4"},
		{"", ".mp4", "fetchly_download.mp4"},
		{"../../etc/passwd", ".mp4", "passwd.mp4"},
		{`..\..\evil.mp4`, ".mp4", "evil.mp4"},
		{"a\r\nSet-Cookie: x=1.mp4", ".mp4", "aSet-Cookie_x1.mp4"},
		{`na"me.jpg`, ".mp4", "name.jpg"},
		{"foto saya.JPG", ".mp4", "foto_saya.jpg"},
		{"malware.exe", ".mp4", "malware.mp4"},
		{"...", ".mp4", "fetchly_download.mp4"},
	}
	for _, tc := range cases {
		if got := Filename(tc.in, tc.def); got != tc.want {
			t.Errorf("Filename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFilenameWithExt(t *testing.T) {
	if got := FilenameWithExt("lagu.mp4", ".m4a"); got != "lagu.m4a" {
		t.Errorf("got %q", got)
	}
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if got := FilenameWithExt(string(long), ".mp4"); len(got) != 84 {
		t.Errorf("panjang nama = %d, want 84", len(got))
	}
}
