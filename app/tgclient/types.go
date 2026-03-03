package tgclient

import "time"

// MessageRecord is normalized message data for rendering.
type MessageRecord struct {
	ID        int
	Date      time.Time
	Text      string
	Media     []MediaRecord
	Permalink string
}

// MediaRecord describes one downloaded media file.
type MediaRecord struct {
	Kind    string // image|video|file|voice
	RelPath string // relative to output dir, e.g. media/123_1.jpg
	Name    string
}

// Config controls one-shot Telegram sync.
type Config struct {
	AppID       int
	AppHash     string
	Phone       string
	Password    string
	Channel     string
	SessionFile string
	OutputDir   string
	Limit       int
	PageSize    int
}

// Stats tracks filtering and rendering counters.
type Stats struct {
	TotalMessages    int
	KeptMessages     int
	FilteredMessages int
	DownloadedMedia  int
}
