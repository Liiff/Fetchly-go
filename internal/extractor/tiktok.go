package extractor

import (
	"context"
	"fmt"
	"strings"

	"github.com/Liiff/fetchly-go/internal/model"
)

type TikTokExtractor struct{}

func (t *TikTokExtractor) Extract(ctx context.Context, targetURL string) (*model.MediaResponse, error) {
	raw, err := runYTDLP(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	var options []model.FormatOption

	var best *ytdlpFormat
	for i := range raw.Formats {
		f := &raw.Formats[i]
		note := strings.ToLower(f.FormatNote)
		if f.Ext != "mp4" || f.VCodec == "none" || f.ACodec == "none" || f.URL == "" || strings.Contains(note, "watermark") {
			continue
		}
		if best == nil || f.Height > best.Height {
			best = f
		}
	}
	if best != nil {
		size := best.Filesize
		if size == 0 {
			size = best.FilesizeApprox
		}
		quality := "Video MP4"
		if best.Height > 0 {
			quality = fmt.Sprintf("Video MP4 %dp", best.Height)
		}
		options = append(options, model.FormatOption{
			FormatID:       best.FormatID,
			Quality:        quality,
			Ext:            "mp4",
			FileSizeApprox: formatBytes(size),
			IsStream:       false,
			DownloadURL:    best.URL,
		})
	}

	if len(options) == 0 && raw.URL != "" {
		options = append(options, model.FormatOption{
			FormatID:    "default",
			Quality:     "Original Quality",
			Ext:         "mp4",
			IsStream:    false,
			DownloadURL: raw.URL,
		})
	}

	for _, f := range raw.Formats {
		if f.VCodec == "none" && f.ACodec != "none" && f.URL != "" {
			size := f.Filesize
			if size == 0 {
				size = f.FilesizeApprox
			}
			options = append(options, model.FormatOption{
				FormatID:       f.FormatID,
				Quality:        "Audio Only",
				Ext:            f.Ext,
				FileSizeApprox: formatBytes(size),
				IsStream:       false,
				DownloadURL:    f.URL,
			})
			break
		}
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("%w: tidak ada format yang bisa diunduh", ErrExtractionFailed)
	}

	return &model.MediaResponse{
		Status:    "success",
		Platform:  "tiktok",
		MediaType: "video",
		Title:     raw.Title,
		Thumbnail: raw.Thumbnail,
		Duration:  formatDuration(raw.Duration),
		Formats:   options,
	}, nil
}
