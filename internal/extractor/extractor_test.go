package extractor

import (
	"context"
	"errors"
	"testing"
)

func TestExtractMediaRejectsBadURLs(t *testing.T) {
	bad := []string{
		"",
		"not a url",
		"https://example.com/video",
		"https://youtube.com.evil.net/watch?v=abc",
		"http://169.254.169.254/latest/meta-data/",
		"file:///etc/passwd",
	}
	for _, raw := range bad {
		// Semua ini harus ditolak sebelum yt-dlp dijalankan.
		_, err := ExtractMedia(context.Background(), raw)
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Errorf("ExtractMedia(%q) err = %v, want ErrUnsupportedPlatform", raw, err)
		}
	}
}

func TestLookupStreamFormat(t *testing.T) {
	for _, id := range []string{"1080p", "720p", "480p", "audio"} {
		if _, ok := LookupStreamFormat(id); !ok {
			t.Errorf("format %q seharusnya ada", id)
		}
	}
	for _, id := range []string{"", "best", "--exec=calc", "bestvideo+bestaudio"} {
		if _, ok := LookupStreamFormat(id); ok {
			t.Errorf("format %q seharusnya ditolak", id)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, ""}, {-5, ""}, {512, "512 B"}, {1024, "1.0 KB"},
		{1536, "1.5 KB"}, {25690112, "24.5 MB"},
	}
	for _, tc := range cases {
		if got := formatBytes(tc.in); got != tc.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, ""}, {-1, ""}, {5, "00:05"}, {65, "01:05"}, {3599, "59:59"}, {3600, "1:00:00"}, {3725, "1:02:05"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.in); got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCleanCaption(t *testing.T) {
	if got := cleanCaption("pendek"); got != "pendek" {
		t.Errorf("got %q", got)
	}
	// 150 karakter multibyte tidak boleh terpotong di tengah rune
	long := ""
	for i := 0; i < 150; i++ {
		long += "é"
	}
	got := cleanCaption(long)
	if r := []rune(got); len(r) != 100 {
		t.Errorf("jumlah rune = %d, want 100", len(r))
	}
}

func TestClassifyYTDLPError(t *testing.T) {
	base := errors.New("exit status 1")
	cases := []struct {
		stderr string
		want   error
	}{
		{"ERROR: Sign in to confirm your age", ErrAgeRestricted},
		{"ERROR: Private video. Sign in if you've been granted access", ErrPrivateMedia},
		{"ERROR: Login required", ErrPrivateMedia},
	}
	for _, tc := range cases {
		err := classifyYTDLPError(base, tc.stderr)
		if !errors.Is(err, tc.want) || !errors.Is(err, ErrExtractionFailed) {
			t.Errorf("stderr %q: err = %v, want %v", tc.stderr, err, tc.want)
		}
	}
	if err := classifyYTDLPError(base, "ERROR: unknown"); errors.Is(err, ErrAgeRestricted) || errors.Is(err, ErrPrivateMedia) {
		t.Error("error umum tidak boleh dikira age/private")
	}
}

func TestEmbedURL(t *testing.T) {
	got, err := embedURL("https://www.instagram.com/p/ABC123/?igsh=xyz#frag")
	if err != nil || got != "https://www.instagram.com/p/ABC123/embed/captioned/" {
		t.Errorf("embedURL = %q, %v", got, err)
	}
}
