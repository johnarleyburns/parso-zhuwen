package images

import (
	"fmt"
	"strings"

	"github.com/parso/zhuwen-factory/internal/pack"
)

// EmbedThumbnails fetches Commons thumbnails at the given pixel width for every
// pack.Image whose Data is nil and whose SourceURL is a Commons File: page, and
// populates Data/W/H with JPEG bytes and the thumbnail dimensions. Images that
// already have Data set are skipped. Individual fetch failures are logged but
// do not stop the batch — the remaining images still get their thumbnails.
// This is the Go-only image-embed path that replaces the HEIC/Python encode stage.
func EmbedThumbnails(images []pack.Image, fc *FetchClient, px int) ([]pack.Image, error) {
	var titles []string
	var indices []int
	for i := range images {
		if images[i].Data != nil {
			continue
		}
		title := CommonsTitleFromSource(images[i].SourceURL)
		if title == "" {
			continue
		}
		indices = append(indices, i)
		titles = append(titles, title)
	}
	if len(indices) == 0 {
		return images, nil
	}

	thumbs, err := fc.FetchThumbs(titles, px)
	if err != nil {
		// If the batch fetch fails, try one-by-one to maximize coverage.
		fmt.Printf("embed: batch thumb fetch failed (%v), trying individually...\n", err)
		thumbs = map[string]ThumbResult{}
		for _, title := range titles {
			indiv, err := fc.FetchThumbs([]string{title}, px)
			if err != nil {
				fmt.Printf("embed: skip %s: %v\n", title, err)
				continue
			}
			for k, v := range indiv {
				thumbs[k] = v
			}
		}
	}

	var ok, skip int
	for _, idx := range indices {
		title := titles[0]
		titles = titles[1:]
		tr, found := thumbs[title]
		if !found {
			skip++
			continue
		}
		images[idx].Data = tr.Data
		images[idx].W = tr.W
		images[idx].H = tr.H
		images[idx].File = strings.Replace(images[idx].File, "@480.heic", "@480.jpg", 1)
		ok++
	}
	if ok > 0 || skip == 0 {
		return images, nil
	}
	return images, fmt.Errorf("all %d thumbnails failed", len(indices))
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
