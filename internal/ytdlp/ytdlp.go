package ytdlp

import (
	"os"
	"path/filepath"
	"runtime"
)

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

func Tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
