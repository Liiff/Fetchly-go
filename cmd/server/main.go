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
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	validate.AddCDNSuffixes(envList("EXTRA_CDN_HOSTS")...)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	if err := r.SetTrustedProxies(envList("TRUSTED_PROXIES")); err != nil {
		log.Fatalf("TRUSTED_PROXIES tidak valid: %v", err)
	}

	r.StaticFile("/", "./web/index.html")

	mediaHandler := handler.NewMediaHandler(handler.Config{
		MaxExtract: envInt("MAX_EXTRACT", 4),
		MaxStream:  envInt("MAX_STREAM", 2),
		MaxProxy:   envInt("MAX_PROXY", 20),
	})

	extractLimiter := limit.NewIPLimiter(20, 5)
	downloadLimiter := limit.NewIPLimiter(30, 10)

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
