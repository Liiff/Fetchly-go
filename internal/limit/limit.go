// Package limit berisi pembatas konkurensi dan rate limit per IP (tanpa dependensi luar).
package limit

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Semaphore membatasi jumlah pekerjaan berat yang berjalan bersamaan.
type Semaphore chan struct{}

func NewSemaphore(n int) Semaphore {
	if n < 1 {
		n = 1
	}
	return make(Semaphore, n)
}

// TryAcquire tidak menunggu: false bila slot penuh.
func (s Semaphore) TryAcquire() bool {
	select {
	case s <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s Semaphore) Release() { <-s }

type visitor struct {
	tokens float64
	last   time.Time
}

// IPLimiter adalah token bucket per alamat IP.
type IPLimiter struct {
	mu       sync.Mutex
	perSec   float64
	burst    float64
	visitors map[string]*visitor
}

// NewIPLimiter: perMinute = laju isi ulang, burst = kapasitas bucket.
func NewIPLimiter(perMinute float64, burst int) *IPLimiter {
	l := &IPLimiter{
		perSec:   perMinute / 60,
		burst:    float64(burst),
		visitors: make(map[string]*visitor),
	}
	go l.cleanupLoop()
	return l
}

func (l *IPLimiter) Allow(ip string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		l.visitors[ip] = &visitor{tokens: l.burst - 1, last: now}
		return true
	}
	v.tokens += now.Sub(v.last).Seconds() * l.perSec
	if v.tokens > l.burst {
		v.tokens = l.burst
	}
	v.last = now
	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func (l *IPLimiter) cleanupLoop() {
	for range time.Tick(time.Minute) {
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.last) > 10*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// Middleware menolak dengan 429 bila IP melewati batas.
func (l *IPLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.Header("Retry-After", "30")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"status":  "error",
				"message": "Terlalu banyak permintaan. Coba lagi sebentar lagi.",
			})
			return
		}
		c.Next()
	}
}
