package model

// ExtractRequest merepresentasikan payload JSON dari client pada POST /api/extract
type ExtractRequest struct {
	URL string `json:"url" binding:"required"`
}

// FormatOption merepresentasikan satu opsi resolusi/format unduhan atau satu slide carousel
type FormatOption struct {
	FormatID       string `json:"format_id"`
	Quality        string `json:"quality"`
	Ext            string `json:"ext"`
	FileSizeApprox string `json:"file_size_approx,omitempty"`
	IsStream       bool   `json:"is_stream"`
	DownloadURL    string `json:"download_url"`
}

// MediaResponse merepresentasikan respons standar sukses untuk POST /api/extract
type MediaResponse struct {
	Status    string         `json:"status"`
	Platform  string         `json:"platform"`
	MediaType string         `json:"media_type"` // "video", "image", atau "carousel"
	Title     string         `json:"title"`
	Thumbnail string         `json:"thumbnail"`
	Duration  string         `json:"duration,omitempty"`
	Formats   []FormatOption `json:"formats"`
}

// ErrorResponse merepresentasikan respons standar saat terjadi kesalahan
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}