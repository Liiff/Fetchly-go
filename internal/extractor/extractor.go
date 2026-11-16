package extractor

import (
	"context"
	"errors"

	"github.com/Liiff/fetchly-go/internal/model"
	"github.com/Liiff/fetchly-go/internal/validate"
)

var (
	ErrUnsupportedPlatform = errors.New("URL tidak valid atau platform belum didukung")
	ErrExtractionFailed    = errors.New("gagal mengekstrak media dari server sumber")
	ErrAgeRestricted       = errors.New("media dibatasi usia")
	ErrPrivateMedia        = errors.New("media privat atau butuh login")
)

// Extractor mendefinisikan kontrak standar untuk semua modul platform
type Extractor interface {
	Extract(ctx context.Context, targetURL string) (*model.MediaResponse, error)
}

// ExtractMedia memvalidasi URL, mendeteksi platform, lalu memanggil extractor yang tepat.
func ExtractMedia(ctx context.Context, rawURL string) (*model.MediaResponse, error) {
	u, platform, err := validate.ParsePlatformURL(rawURL)
	if err != nil {
		return nil, ErrUnsupportedPlatform
	}
	target := u.String()

	switch platform {
	case validate.YouTube:
		return (&YouTubeExtractor{}).Extract(ctx, target)
	case validate.TikTok:
		return (&TikTokExtractor{}).Extract(ctx, target)
	case validate.Instagram:
		return (&InstagramExtractor{}).Extract(ctx, target)
	default:
		return nil, ErrUnsupportedPlatform
	}
}
