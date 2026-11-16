// Package ytdlp memusatkan pencarian binary dan argumen bersama untuk yt-dlp.
package ytdlp

import (
	"os"
	"path/filepath"
	"runtime"
)

// BinaryPath mencari <name>.exe di folder proyek (Windows) sebelum PATH sistem.
func BinaryPath(name string) string {
	if runtime.GOOS == "windows" {
		local := name + ".exe"
		if _, err := os.Stat(local); err == nil {
			if abs, err := filepath.Abs(local); err == nil {
				return abs
			}
		}
	}
	return name
}

// CookieArgs mengembalikan argumen cookies + runtime JS bila cookies.txt ada.
func CookieArgs() []string {
	if _, err := os.Stat("cookies.txt"); err != nil {
		return nil
	}
	return []string{
		"--cookies", "cookies.txt",
		"--js-runtimes", "deno:" + BinaryPath("deno"),
		"--remote-components", "ejs:github",
	}
}

// FFmpegArgs hanya menunjuk folder proyek bila ffmpeg lokal ada di sana;
// selain itu yt-dlp dibiarkan mencari ffmpeg di PATH (perilaku lama menunjuk "."
// di Linux/macOS sehingga ffmpeg sistem tidak ditemukan).
func FFmpegArgs() []string {
	name := "ffmpeg"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, err := os.Stat(name); err != nil {
		return nil
	}
	abs, err := filepath.Abs(".")
	if err != nil {
		return nil
	}
	return []string{"--ffmpeg-location", abs}
}

// Tail mengambil maksimal n byte terakhir dari s (untuk log stderr yang ringkas).
func Tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
