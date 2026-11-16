package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Liiff/fetchly-go/internal/handler"
	"github.com/Liiff/fetchly-go/internal/limit"
	"github.com/Liiff/fetchly-go/internal/validate"
	"github.com/gin-gonic/gin"
)

func envList(key string) []string {
	var out []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n > 0 {
		return n
	}
	return def
}

func main() {
	// Release mode secara default; set GIN_MODE=debug saat development.
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Domain CDN tambahan yang boleh di-proxy, contoh: EXTRA_CDN_HOSTS=example-cdn.com,foo.net
	validate.AddCDNSuffixes(envList("EXTRA_CDN_HOSTS")...)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// Secara default tidak ada proxy yang dipercaya, jadi X-Forwarded-For dari client diabaikan
	// dan rate limit memakai IP koneksi asli. Jika di belakang reverse proxy/Cloudflare,
	// isi TRUSTED_PROXIES (daftar IP/CIDR dipisah koma), jika tidak semua pengguna dianggap satu IP.
	if err := r.SetTrustedProxies(envList("TRUSTED_PROXIES")); err != nil {
		log.Fatalf("TRUSTED_PROXIES tidak valid: %v", err)
	}

	r.StaticFile("/", "./web/index.html")

	mediaHandler := handler.NewMediaHandler(handler.Config{
		MaxExtract: envInt("MAX_EXTRACT", 4),
		MaxStream:  envInt("MAX_STREAM", 2),
		MaxProxy:   envInt("MAX_PROXY", 20),
	})

	extractLimiter := limit.NewIPLimiter(20, 5)   // 20 permintaan/menit per IP, burst 5
	downloadLimiter := limit.NewIPLimiter(30, 10) // 30 permintaan/menit per IP, burst 10

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "fetchly-go"})
		})
		api.POST("/extract", extractLimiter.Middleware(), mediaHandler.Extract)
		api.GET("/download", downloadLimiter.Middleware(), mediaHandler.Download)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		// WriteTimeout sengaja tidak diset: unduhan besar bisa berjalan lama,
		// batas waktunya diatur per-request lewat context di handler.
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Server Fetchly berjalan di http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Gagal menjalankan server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Mematikan server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown tidak bersih: %v", err)
	}
}
