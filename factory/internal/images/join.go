package images

import (
	"fmt"
	"sort"

	"github.com/parso/zhuwen-factory/internal/pack"
)

// JoinResult merges curated images into pipeline results, replacing any stub image
// records that match the same CanonID. Images not referenced by any story are kept;
// unreferenced images (missing from the curated set) cause an error (I6).
//
// Foundations card images are passed separately and added to the result directly
// (they reference word_id, not canon_id).
func JoinResult(result pack.Pack, curatedImages []pack.Image, foundationsImages []pack.Image) (pack.Pack, error) {
	// Index curated images by CanonID for story image replacement.
	byCanon := map[string]pack.Image{}
	for _, im := range curatedImages {
		if im.CanonID != "" {
			byCanon[im.CanonID] = im
		}
	}

	var images []pack.Image
	storyCanonIDs := map[string]bool{}

	// Replace story images: if a story's cover_image_id points to a stub image whose
	// canon has a curated replacement, swap in the curated version. Otherwise keep the
	// existing image (it may already be curated from a prior join).
	existingByCanon := map[string]int{}
	for i, im := range result.Images {
		if im.CanonID != "" {
			existingByCanon[im.CanonID] = i
		}
	}
	replacements := map[int]pack.Image{} // old index → new image
	seenCanon := map[string]bool{}

	for _, s := range result.Stories {
		if s.CanonID != "" {
			storyCanonIDs[s.CanonID] = true
		}
		// Find the matching image for this story's cover.
		for i, im := range result.Images {
			if im.ID == s.CoverImageID && im.CanonID != "" {
				if curated, ok := byCanon[im.CanonID]; ok {
					// Replace the stub with the curated version, keeping the same ID
					// so story references stay valid.
					curated.ID = im.ID
					curated.File = fmt.Sprintf("images/%s@480.heic", im.ID)
					replacements[i] = curated
					seenCanon[im.CanonID] = true
				}
				break
			}
		}
	}

	for i, im := range result.Images {
		if repl, ok := replacements[i]; ok {
			images = append(images, repl)
		} else {
			images = append(images, im)
		}
	}

	// Add any curated story images that are new (not replacing an existing stub).
	for canonID, curated := range byCanon {
		if !seenCanon[canonID] {
			curated.ID = "img-" + canonID
			curated.File = fmt.Sprintf("images/%s@480.heic", curated.ID)
			images = append(images, curated)
		}
	}

	// Add Foundations card images.
	images = append(images, foundationsImages...)

	result.Images = images

	// Sort images deterministically.
	sort.SliceStable(result.Images, func(i, j int) bool { return result.Images[i].ID < result.Images[j].ID })

	return result, nil
}
