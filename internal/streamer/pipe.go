package streamer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/Liiff/fetchly-go/internal/safehttp"
	"github.com/Liiff/fetchly-go/internal/validate"
	"github.com/Liiff/fetchly-go/internal/ytdlp"
)

const (
	// MaxFileBytes adalah batas ukuran satu unduhan (yt-dlp --max-filesize dan proxy CDN).
	MaxFileBytes = 500 << 20

	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// cdnClient tanpa timeout total (file besar); batas waktu diatur lewat context pemanggil.
var cdnClient = safehttp.New(0, func(u *url.URL) bool { return validate.CDNAllowed(u) })

var ErrTooLarge = errors.New("file melebihi batas ukuran")

// contentDisposition membangun header yang aman dari karakter kontrol/kutip (anti header injection).
func contentDisposition(filename string) string {
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); v != "" {
		return v
	}
	return "attachment"
}

func setDownloadHeaders(w http.ResponseWriter, filename string) {
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", contentDisposition(filename))
	h.Set("X-Content-Type-Options", "nosniff")
}

// StreamMedia menjalankan yt-dlp ke folder sementara lalu mengirim hasilnya ke client.
// targetURL, selector, dan ext HARUS sudah divalidasi oleh pemanggil.
// ext memakai titik di depan (contoh: ".mp4").
func StreamMedia(ctx context.Context, w http.ResponseWriter, targetURL, selector, ext, filename string) error {
	// Folder sementara: file bagian (video/audio terpisah) ikut terhapus walau proses dibatalkan.
	dir, err := os.MkdirTemp("", "fetchly-*")
	if err != nil {
		return fmt.Errorf("gagal membuat folder sementara: %w", err)
	}
	defer os.RemoveAll(dir)

	outPath := filepath.Join(dir, "media"+ext)

	args := []string{
		"-f", selector,
		"--force-overwrites",
		"--no-playlist",
		"--no-warnings",
		"--socket-timeout", "15",
		"--max-filesize", strconv.Itoa(MaxFileBytes),
	}
	args = append(args, ytdlp.FFmpegArgs()...)
	if ext == ".mp4" {
		args = append(args, "--merge-output-format", "mp4")
	}
	args = append(args, ytdlp.CookieArgs()...)
	args = append(args, "-o", outPath, "--", targetURL)

	cmd := exec.CommandContext(ctx, ytdlp.BinaryPath("yt-dlp"), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yt-dlp gagal memproses media (%v): %s", err, ytdlp.Tail(stderr.String(), 500))
	}

	f, err := os.Open(outPath)
	if err != nil {
		return fmt.Errorf("file hasil tidak ditemukan: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("gagal membaca ukuran file hasil: %w", err)
	}
	if info.Size() > MaxFileBytes {
		return ErrTooLarge
	}

	setDownloadHeaders(w, filename)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	_, err = io.Copy(w, f)
	return err
}

// ProxyDirectURL meneruskan file dari CDN sumber ke browser.
// cdnURL HARUS sudah lolos validate.ParseCDNURL; client-nya sendiri juga menolak IP
// privat dan redirect ke host di luar allowlist.
func ProxyDirectURL(ctx context.Context, w http.ResponseWriter, cdnURL, filename string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdnURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := cdnClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("CDN mengembalikan status %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxFileBytes {
		return ErrTooLarge
	}

	setDownloadHeaders(w, filename)
	if resp.ContentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	// Batas keras juga untuk respons tanpa Content-Length (chunked).
	_, err = io.Copy(w, io.LimitReader(resp.Body, MaxFileBytes))
	return err
}
