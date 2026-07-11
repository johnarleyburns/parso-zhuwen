package images

import (
	"fmt"
	"strings"

	"github.com/parso/zhuwen-factory/internal/pack"
)

// EmbedThumbnails fetches Commons thumbnails at the given pixel width for every
// pack.Image whose Data is nil and whose SourceURL is a Commons File: page, and
// populates Data/W/H with JPEG bytes and the thumbnail dimensions. Images that
// already have Data set are skipped. This is the Go-only image-embed path that
// replaces the HEIC/Python encode stage (B-4, build-time, behind --live, I2).
func EmbedThumbnails(images []pack.Image, fc *FetchClient, px int) ([]pack.Image, error) {
	// Collect titles for images that need thumbnails.
	var need []int
	var titles []string
	for i := range images {
		if images[i].Data != nil {
			continue
		}
		title := CommonsTitleFromSource(images[i].SourceURL)
		if title == "" {
			continue
		}
		need = append(need, i)
		titles = append(titles, title)
	}
	if len(need) == 0 {
		return images, nil
	}

	thumbs, err := fc.FetchThumbs(titles, px)
	if err != nil {
		return nil, fmt.Errorf("embed thumbnails: %w", err)
	}

	for j, idx := range need {
		title := titles[j]
		tr, ok := thumbs[title]
		if !ok {
			return nil, fmt.Errorf("embed thumbnails: no thumbnail for %q", title)
		}
		images[idx].Data = tr.Data
		images[idx].W = tr.W
		images[idx].H = tr.H
		images[idx].File = strings.Replace(images[idx].File, "@480.heic", "@480.jpg", 1)
	}
	return images, nil
}

// CommonsTitleFromSource extracts a File: title from a Commons source URL.
func CommonsTitleFromSource(sourceURL string) string {
	if !strings.Contains(sourceURL, "commons.wikimedia.org/wiki/File:") {
		return ""
	}
	idx := strings.LastIndex(sourceURL, "File:")
	if idx < 0 {
		return ""
	}
	title := sourceURL[idx:]
	// Remove query params and fragments.
	if qi := strings.IndexAny(title, "?#"); qi >= 0 {
		title = title[:qi]
	}
	return title
}
