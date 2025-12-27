package downloader

type Format struct {
	FormatID   string   `json:"format_id"`
	Ext        string   `json:"ext"`
	Resolution string   `json:"resolution"`
	Filesize   *int64   `json:"filesize"`
	TBR        *float64 `json:"tbr"`
	Acodec     string   `json:"acodec"`
	Vcodec     string   `json:"vcodec"`
}

type VideoInfo struct {
	Duration float64  `json:"duration"`
	Formats  []Format `json:"formats"`
}

type FormatInfo struct {
	FormatID   string
	Display    string
	FilesizeMB string
}
