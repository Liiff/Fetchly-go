package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Liiff/fetchly-go/internal/extractor"
	"github.com/Liiff/fetchly-go/internal/limit"
	"github.com/Liiff/fetchly-go/internal/model"
	"github.com/Liiff/fetchly-go/internal/streamer"
	"github.com/Liiff/fetchly-go/internal/validate"
	"github.com/gin-gonic/gin"
)

const (
	extractTimeout = 30 * time.Second
	streamTimeout  = 10 * time.Minute
	proxyTimeout   = 10 * time.Minute
	maxBodyBytes   = 16 << 10
)

type Config struct {
	MaxExtract int
	MaxStream  int
	MaxProxy   int
}

type MediaHandler struct {
	extractSem limit.Semaphore
	streamSem  limit.Semaphore
	proxySem   limit.Semaphore
}

func NewMediaHandler(cfg Config) *MediaHandler {
	def := func(v, d int) int {
		if v <= 0 {
			return d
		}
		return v
	}
	return &MediaHandler{
		extractSem: limit.NewSemaphore(def(cfg.MaxExtract, 4)),
		streamSem:  limit.NewSemaphore(def(cfg.MaxStream, 2)),
		proxySem:   limit.NewSemaphore(def(cfg.MaxProxy, 20)),
	}
}

func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, model.ErrorResponse{Status: "error", Message: msg})
}

func busy(c *gin.Context) {
	c.Header("Retry-After", "10")
	fail(c, http.StatusServiceUnavailable, "Server sedang sibuk. Coba lagi sebentar lagi.")
}

func (h *MediaHandler) Extract(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)

	var req model.ExtractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "Field 'url' wajib diisi.")
		return
	}

	if !h.extractSem.TryAcquire() {
		busy(c)
		return
	}
	defer h.extractSem.Release()

	ctx, cancel := context.WithTimeout(c.Request.Context(), extractTimeout)
	defer cancel()

	result, err := extractor.ExtractMedia(ctx, req.URL)
	if err != nil {
		log.Printf("[EXTRACT ERROR] URL: %s | Detail: %v\n", req.URL, err)

		switch {
		case errors.Is(err, extractor.ErrUnsupportedPlatform):
			fail(c, http.StatusBadRequest, "URL tidak valid atau platform belum didukung.")
		case errors.Is(err, extractor.ErrAgeRestricted):
			fail(c, http.StatusUnprocessableEntity, "Video ini dibatasi usia (Age-Restricted) oleh YouTube.")
		case errors.Is(err, extractor.ErrPrivateMedia):
			fail(c, http.StatusUnprocessableEntity, "Media bersifat privat dan tidak dapat diakses publik.")
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			fail(c, http.StatusGatewayTimeout, "Waktu habis saat mengekstrak media. Coba lagi.")
		default:
			fail(c, http.StatusBadGateway, "Gagal mengekstrak media dari server sumber.")
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *MediaHandler) Download(c *gin.Context) {
	if c.Query("proxy") == "1" {
		h.downloadProxy(c)
		return
	}
	h.downloadStream(c)
}

func (h *MediaHandler) downloadProxy(c *gin.Context) {
	u, err := validate.ParseCDNURL(c.Query("url"))
	if err != nil {
		fail(c, http.StatusBadRequest, "URL media tidak valid atau host tidak diizinkan.")
		return
	}

	if !h.proxySem.TryAcquire() {
		busy(c)
		return
	}
	defer h.proxySem.Release()

	filename := validate.Filename(c.Query("filename"), ".mp4")

	ctx, cancel := context.WithTimeout(c.Request.Context(), proxyTimeout)
	defer cancel()

	if err := streamer.ProxyDirectURL(ctx, c.Writer, u.String(), filename); err != nil {
		h.downloadFailed(c, u.Hostname(), err)
	}
}

func (h *MediaHandler) downloadStream(c *gin.Context) {
	u, platform, err := validate.ParsePlatformURL(c.Query("url"))
	if err != nil || platform != validate.YouTube {
		fail(c, http.StatusBadRequest, "URL tidak valid atau tidak didukung untuk unduhan stream.")
		return
	}

	sf, ok := extractor.LookupStreamFormat(c.Query("format_id"))
	if !ok {
		fail(c, http.StatusBadRequest, "Parameter 'format_id' tidak valid.")
		return
	}

	if !h.streamSem.TryAcquire() {
		busy(c)
		return
	}
	defer h.streamSem.Release()

	ext := "." + sf.Ext
	filename := validate.FilenameWithExt(c.Query("filename"), ext)

	ctx, cancel := context.WithTimeout(c.Request.Context(), streamTimeout)
	defer cancel()

	if err := streamer.StreamMedia(ctx, c.Writer, u.String(), sf.Selector, ext, filename); err != nil {
		h.downloadFailed(c, u.String(), err)
	}
}

func (h *MediaHandler) downloadFailed(c *gin.Context, target string, err error) {
	log.Printf("[DOWNLOAD ERROR] Target: %s | Detail: %v\n", target, err)
	if c.Writer.Written() {
		return
	}
	switch {
	case errors.Is(err, streamer.ErrTooLarge):
		fail(c, http.StatusRequestEntityTooLarge, "File terlalu besar untuk diunduh.")
	case errors.Is(err, context.DeadlineExceeded):
		fail(c, http.StatusGatewayTimeout, "Waktu habis saat mengunduh media.")
	default:
		fail(c, http.StatusBadGateway, "Gagal mengunduh media.")
	}
}
