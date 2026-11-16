package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/Liiff/fetchly/internal/ytdlp"
)

// ytdlpOutput merepresentasikan field penting dari output `yt-dlp --dump-json`
type ytdlpOutput struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Thumbnail string        `json:"thumbnail"`
	Duration  float64       `json:"duration"`
	URL       string        `json:"url"`
	Ext       string        `json:"ext"`
	Formats   []ytdlpFormat `json:"formats"`
}

type ytdlpFormat struct {
	FormatID       string `json:"format_id"`
	Ext            string `json:"ext"`
	Height         int    `json:"height"`
	VCodec         string `json:"vcodec"`
	ACodec         string `json:"acodec"`
	Filesize       int64  `json:"filesize"`
	FilesizeApprox int64  `json:"filesize_approx"`
	URL            string `json:"url"`
	FormatNote     string `json:"format_note"`
}

// runYTDLP menjalankan `yt-dlp --dump-json --no-playlist` dengan context timeout.
// targetURL HARUS sudah divalidasi oleh pemanggil (lihat validate.ParsePlatformURL).
func runYTDLP(ctx context.Context, targetURL string) (*ytdlpOutput, error) {
	args := []string{
		"--dump-json",
		"--no-playlist",
		"--no-warnings",
		"--ignore-no-formats-error",
		"--socket-timeout", "15",
	}
	args = append(args, ytdlp.CookieArgs()...)
	// "--" memastikan targetURL tidak pernah dibaca sebagai opsi yt-dlp.
	args = append(args, "--", targetURL)

	cmd := exec.CommandContext(ctx, ytdlp.BinaryPath("yt-dlp"), args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, classifyYTDLPError(err, stderr.String())
	}

	var data ytdlpOutput
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("%w: gagal parsing metadata json: %v", ErrExtractionFailed, err)
	}
	return &data, nil
}

// classifyYTDLPError mengubah stderr yt-dlp menjadi sentinel error agar handler
// tidak perlu mencocokkan string.
func classifyYTDLPError(runErr error, stderr string) error {
	lower := strings.ToLower(stderr)
	tail := ytdlp.Tail(strings.TrimSpace(stderr), 500)

	var kind error
	switch {
	case strings.Contains(lower, "confirm your age"):
		kind = ErrAgeRestricted
	case strings.Contains(lower, "private video"),
		strings.Contains(lower, "account is private"),
		strings.Contains(lower, "login required"),
		strings.Contains(lower, "requires login"):
		kind = ErrPrivateMedia
	}

	if kind != nil {
		return fmt.Errorf("%w: %w: %v | stderr: %s", ErrExtractionFailed, kind, runErr, tail)
	}
	return fmt.Errorf("%w: %v | stderr: %s", ErrExtractionFailed, runErr, tail)
}

// formatBytes mengubah ukuran byte menjadi string yang mudah dibaca (contoh: "24.5 MB")
func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatDuration mengubah detik menjadi MM:SS, atau H:MM:SS untuk durasi >= 1 jam
func formatDuration(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	total := int(seconds)
	hours := total / 3600
	mins := (total % 3600) / 60
	secs := total % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

// truncateRunes memotong string berdasarkan jumlah karakter (bukan byte) agar UTF-8 tidak rusak.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-3]) + "..."
}
