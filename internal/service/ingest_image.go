package service

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

// supportedRawFormats lists file extensions that IngestFile accepts.
var supportedRawFormats = map[string]bool{
	".md":       true,
	".markdown": true,
	".pdf":      true,
	".png":      true,
	".jpg":      true,
	".jpeg":     true,
	".gif":      true,
	".webp":     true,
}

// ErrUnsupportedRawFormat indicates the file type is not supported for ingest.
var ErrUnsupportedRawFormat = fmt.Errorf("unsupported raw format")

// IsSupportedFormat checks if the file extension is supported for ingest.
func IsSupportedFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return supportedRawFormats[ext]
}

// ImageMeta holds basic image metadata extracted during ingest.
type ImageMeta struct {
	Width  int
	Height int
	Format string // "png", "jpeg", "gif"
}

// ExtractImageMeta reads image dimensions from the file header.
// Returns nil if the file cannot be decoded as an image.
func ExtractImageMeta(path string) *ImageMeta {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil
	}
	return &ImageMeta{
		Width:  cfg.Width,
		Height: cfg.Height,
		Format: format,
	}
}
