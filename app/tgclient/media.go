package tgclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"github.com/sirupsen/logrus"
)

func downloadMessageMedia(ctx context.Context, api *tg.Client, dl *downloader.Downloader, outputDir string, msg *tg.Message, logger *logrus.Logger) ([]MediaRecord, error) {
	if msg.Media == nil {
		return nil, nil
	}
	result := make([]MediaRecord, 0, 2)
	mediaDir := filepath.Join(outputDir, "media")

	switch m := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		ph, ok := m.Photo.(*tg.Photo)
		if !ok {
			return nil, nil
		}
		loc := &tg.InputPhotoFileLocation{ID: ph.ID, AccessHash: ph.AccessHash, FileReference: ph.FileReference, ThumbSize: "x"}
		ext := ".jpg"
		rel := filepath.ToSlash(filepath.Join("media", fmt.Sprintf("%d_photo%s", msg.ID, ext)))
		full := filepath.Join(outputDir, rel)
		if err := saveLocation(ctx, api, dl, loc, full); err != nil {
			return nil, err
		}
		result = append(result, MediaRecord{Kind: "image", RelPath: rel, Name: filepath.Base(rel)})
	case *tg.MessageMediaDocument:
		doc, ok := m.Document.(*tg.Document)
		if !ok {
			return nil, nil
		}
		isVideo := strings.HasPrefix(doc.MimeType, "video/")
		kind := "file"
		ext := extByMime(doc.MimeType)
		if isVideo {
			kind = "video"
		}
		if ext == "" {
			ext = ".bin"
		}
		rel := filepath.ToSlash(filepath.Join("media", fmt.Sprintf("%d_doc%s", msg.ID, ext)))
		full := filepath.Join(outputDir, rel)
		loc := &tg.InputDocumentFileLocation{ID: doc.ID, AccessHash: doc.AccessHash, FileReference: doc.FileReference, ThumbSize: ""}
		if err := saveLocation(ctx, api, dl, loc, full); err != nil {
			return nil, err
		}
		result = append(result, MediaRecord{Kind: kind, RelPath: rel, Name: filepath.Base(rel)})
	default:
		logger.Debugf("unsupported media for msg=%d type=%T", msg.ID, msg.Media)
	}

	if len(result) == 0 {
		_ = os.MkdirAll(mediaDir, 0o755)
	}
	return result, nil
}

func saveLocation(ctx context.Context, api *tg.Client, dl *downloader.Downloader, loc tg.InputFileLocationClass, fullPath string) error {
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	b := dl.WithPartSize(512 * 1024).Download(api, loc)
	if _, err := b.Stream(ctx, f); err != nil {
		return err
	}
	return nil
}

func extByMime(mime string) string {
	switch mime {
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	default:
		if strings.HasPrefix(mime, "video/") {
			return ".mp4"
		}
		if strings.HasPrefix(mime, "image/") {
			return ".jpg"
		}
	}
	return ""
}
