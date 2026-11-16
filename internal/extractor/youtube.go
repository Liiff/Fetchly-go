package extractor

import (
	"context"
	"fmt"
	"net/url"

	"github.com/Liiff/fetchly-go/internal/model"
)

// StreamFormat adalah satu pilihan unduhan YouTube. Client hanya mengirim ID pendek;
// selector yt-dlp tidak pernah diterima dari luar.
type StreamFormat struct {
	ID       string
	Quality  string
	Ext      string // tanpa titik
	Selector string
}

var youtubeFormats = []StreamFormat{
	{
		ID: "1080p", Quality: "1080p (Full HD)", Ext: "mp4",
		Selector: "bestvideo[height<=1080][vcodec^=avc1]+bestaudio[acodec^=mp4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]/best",
	},
	{
		ID: "720p", Quality: "720p (HD)", Ext: "mp4",
		Selector: "bestvideo[height<=720][vcodec^=avc1]+bestaudio[acodec^=mp4a]/bestvideo[height<=720]+bestaudio/best[height<=720]/best",
	},
	{
		ID: "480p", Quality: "480p (SD)", Ext: "mp4",
		Selector: "bestvideo[height<=480][vcodec^=avc1]+bestaudio[acodec^=mp4a]/bestvideo[height<=480]+bestaudio/best[height<=480]/best",
	},
	{
		ID: "audio", Quality: "Audio Only (M4A)", Ext: "m4a",
		Selector: "bestaudio[ext=m4a]/bestaudio/best",
	},
}

// LookupStreamFormat mencari format berdasarkan ID dari client.
func LookupStreamFormat(id string) (StreamFormat, bool) {
	for _, f := range youtubeFormats {
		if f.ID == id {
			return f, true
		}
	}
	return StreamFormat{}, false
}

type YouTubeExtractor struct{}

func (y *YouTubeExtractor) Extract(ctx context.Context, targetURL string) (*model.MediaResponse, error) {
	raw, err := runYTDLP(ctx, targetURL)
	if err != nil {
		return nil, err
	}

	encodedURL := url.QueryEscape(targetURL)
	options := make([]model.FormatOption, 0, len(youtubeFormats))
	for _, f := range youtubeFormats {
		options = append(options, model.FormatOption{
			FormatID:    f.ID,
			Quality:     f.Quality,
			Ext:         f.Ext,
			IsStream:    true,
			DownloadURL: fmt.Sprintf("/api/download?url=%s&format_id=%s", encodedURL, url.QueryEscape(f.ID)),
		})
	}

	return &model.MediaResponse{
		Status:    "success",
		Platform:  "youtube",
		MediaType: "video",
		Title:     raw.Title,
		Thumbnail: raw.Thumbnail,
		Duration:  formatDuration(raw.Duration),
		Formats:   options,
	}, nil
}
