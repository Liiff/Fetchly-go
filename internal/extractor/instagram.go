package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Liiff/fetchly-go/internal/model"
	"github.com/Liiff/fetchly-go/internal/safehttp"
	"github.com/Liiff/fetchly-go/internal/validate"
)

const (
	igUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	igMaxBodyBytes = 8 << 20
)

var (
	reShortcodeMedia = regexp.MustCompile(`"shortcode_media"\s*:\s*(\{.*?\})\s*,\s*"context"`)
	reCDNImage       = regexp.MustCompile(`https://[^"'\s\\]+cdninstagram\.com/v/t51\.2885-15/[^"'\s\\]+`)

	igClient = safehttp.New(15*time.Second, func(u *url.URL) bool {
		return validate.HostMatches(u.Hostname(), "instagram.com")
	})
)

type InstagramExtractor struct{}

func (ig *InstagramExtractor) Extract(ctx context.Context, targetURL string) (*model.MediaResponse, error) {
	if raw, err := runYTDLP(ctx, targetURL); err == nil && raw.URL != "" {
		return &model.MediaResponse{
			Status:    "success",
			Platform:  "instagram",
			MediaType: "video",
			Title:     cleanCaption(raw.Title),
			Thumbnail: raw.Thumbnail,
			Duration:  formatDuration(raw.Duration),
			Formats: []model.FormatOption{
				{
					FormatID:    "ig_video",
					Quality:     "Original Video (HD)",
					Ext:         "mp4",
					IsStream:    false,
					DownloadURL: raw.URL,
				},
			},
		}, nil
	}

	return ig.extractPhotoOrCarousel(ctx, targetURL)
}

func embedURL(targetURL string) (string, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	u.Scheme = "https"
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/") + "/embed/captioned/"
	u.RawPath = ""
	return u.String(), nil
}

type igNode struct {
	DisplayURL string `json:"display_url"`
	IsVideo    bool   `json:"is_video"`
	VideoURL   string `json:"video_url"`
}

func (ig *InstagramExtractor) extractPhotoOrCarousel(ctx context.Context, targetURL string) (*model.MediaResponse, error) {
	embed, err := embedURL(targetURL)
	if err != nil {
		return nil, fmt.Errorf("%w: URL tidak valid: %v", ErrExtractionFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, embed, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", igUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := igClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: gagal menghubungi Instagram: %v", ErrExtractionFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("%w: %w: Instagram membalas status %d", ErrExtractionFailed, ErrPrivateMedia, resp.StatusCode)
		}
		return nil, fmt.Errorf("%w: Instagram membalas status %d", ErrExtractionFailed, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, igMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: gagal membaca respons Instagram: %v", ErrExtractionFailed, err)
	}
	html := string(bodyBytes)

	var options []model.FormatOption

	if matches := reShortcodeMedia.FindStringSubmatch(html); len(matches) > 1 {
		var mediaData struct {
			igNode
			EdgeSidecar struct {
				Edges []struct {
					Node igNode `json:"node"`
				} `json:"edges"`
			} `json:"edge_sidecar_to_children"`
		}

		if err := json.Unmarshal([]byte(matches[1]), &mediaData); err == nil {
			if len(mediaData.EdgeSidecar.Edges) > 0 {
				// Carousel
				for i, edge := range mediaData.EdgeSidecar.Edges {
					node := edge.Node
					if node.IsVideo && node.VideoURL != "" {
						options = append(options, model.FormatOption{
							FormatID:    fmt.Sprintf("slide_%d", i+1),
							Quality:     fmt.Sprintf("Slide %d — Video", i+1),
							Ext:         "mp4",
							DownloadURL: node.VideoURL,
						})
					} else if node.DisplayURL != "" {
						options = append(options, model.FormatOption{
							FormatID:    fmt.Sprintf("slide_%d", i+1),
							Quality:     fmt.Sprintf("Slide %d — Foto HD", i+1),
							Ext:         "jpg",
							DownloadURL: node.DisplayURL,
						})
					}
				}
			} else if mediaData.DisplayURL != "" {
				// Foto tunggal
				options = append(options, model.FormatOption{
					FormatID:    "photo_1",
					Quality:     "Original Photo (HD)",
					Ext:         "jpg",
					DownloadURL: mediaData.DisplayURL,
				})
			}
		}
	}

	if len(options) == 0 {
		seen := make(map[string]bool)
		slideIdx := 1
		for _, rawImg := range reCDNImage.FindAllString(html, -1) {
			u := strings.ReplaceAll(rawImg, `\u0026`, "&")
			u = strings.ReplaceAll(u, `\/`, "/")
			if seen[u] || strings.Contains(u, "s150x150") || strings.Contains(u, "s320x320") {
				continue
			}
			seen[u] = true
			options = append(options, model.FormatOption{
				FormatID:    fmt.Sprintf("slide_%d", slideIdx),
				Quality:     fmt.Sprintf("Slide %d — Foto HD", slideIdx),
				Ext:         "jpg",
				DownloadURL: u,
			})
			slideIdx++
		}
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("%w: %w: tidak dapat mengambil media Instagram", ErrExtractionFailed, ErrPrivateMedia)
	}

	mediaType := "image"
	if len(options) > 1 {
		mediaType = "carousel"
	}

	return &model.MediaResponse{
		Status:    "success",
		Platform:  "instagram",
		MediaType: mediaType,
		Title:     "Instagram Post",
		Thumbnail: options[0].DownloadURL,
		Formats:   options,
	}, nil
}

func cleanCaption(title string) string {
	return truncateRunes(title, 100)
}
