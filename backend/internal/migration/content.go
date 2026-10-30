package migration

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// rewriteLegacyMediaURLs points post and page content at the asset original
// URLs created by backfillAssetBlob, so content stops depending on the legacy
// /uploads route. Only URLs that produced an asset are replaced, and the match
// is exact, so external links and unrelated text stay untouched.
func (s *Service) rewriteLegacyMediaURLs() error {
	replacements, err := s.legacyMediaURLReplacements()
	if err != nil {
		return err
	}
	if len(replacements) == 0 {
		return nil
	}

	replacer := newLegacyURLReplacer(replacements)
	posts, err := s.rewriteContent("posts", replacer)
	if err != nil {
		return err
	}
	pages, err := s.rewriteContent("pages", replacer)
	if err != nil {
		return err
	}

	if posts+pages > 0 {
		slog.Info(
			"rewrote legacy media URLs in content",
			"posts", posts,
			"pages", pages,
			"urls", len(replacements),
		)
	}
	return nil
}

// legacyMediaURLReplacements maps each legacy media URL to the original asset
// URL that the media backfill created for it. Rows without a key, without a
// blob, or without a public asset are left out: their content keeps using the
// legacy route.
func (s *Service) legacyMediaURLReplacements() (map[string]string, error) {
	var mediaFiles []model.MediaFile
	if err := s.db.Find(&mediaFiles).Error; err != nil {
		return nil, fmt.Errorf("list legacy media files: %w", err)
	}
	if len(mediaFiles) == 0 {
		return nil, nil
	}

	var blobs []model.Blob
	if err := s.db.Find(&blobs).Error; err != nil {
		return nil, fmt.Errorf("list blobs for content rewrite: %w", err)
	}
	blobIDs := make(map[string]uint, len(blobs))
	for _, blob := range blobs {
		blobIDs[blob.StorageKey] = blob.ID
	}

	var assets []model.Asset
	if err := s.db.Where("visibility = ?", model.AssetVisibilityPublic).Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("list assets for content rewrite: %w", err)
	}
	assetURLs := make(map[uint]string, len(assets))
	for _, item := range assets {
		assetURLs[item.BlobID] = asset.OriginalURL(item.PublicID)
	}

	replacements := make(map[string]string, len(mediaFiles))
	for _, mediaFile := range mediaFiles {
		if mediaFile.URL == "" {
			continue
		}

		key := mediaFile.StorageKey
		if key == "" {
			var ok bool
			if key, ok = strings.CutPrefix(mediaFile.URL, "/uploads/"); !ok {
				continue
			}
		}

		blobID, ok := blobIDs[key]
		if !ok {
			continue
		}
		assetURL, ok := assetURLs[blobID]
		if !ok {
			continue
		}

		replacements[mediaFile.URL] = assetURL
	}

	return replacements, nil
}

// newLegacyURLReplacer matches the longest URL first, so a URL that is a
// prefix of another cannot win inside the longer one.
func newLegacyURLReplacer(replacements map[string]string) *strings.Replacer {
	oldURLs := make([]string, 0, len(replacements))
	for oldURL := range replacements {
		oldURLs = append(oldURLs, oldURL)
	}
	sort.Slice(oldURLs, func(i, j int) bool {
		return len(oldURLs[i]) > len(oldURLs[j])
	})

	pairs := make([]string, 0, 2*len(oldURLs))
	for _, oldURL := range oldURLs {
		pairs = append(pairs, oldURL, replacements[oldURL])
	}
	return strings.NewReplacer(pairs...)
}

// rewriteContent rewrites every non-empty content cell of table. Only changed
// rows are written, so repeated startups are no-ops once the URLs are gone.
func (s *Service) rewriteContent(table string, replacer *strings.Replacer) (int64, error) {
	type contentRow struct {
		ID      uint
		Content string
	}

	var rows []contentRow
	if err := s.db.Table(table).
		Select("id, content").
		Where("content <> ''").
		Find(&rows).
		Error; err != nil {
		return 0, fmt.Errorf("load %s content: %w", table, err)
	}

	var updated int64
	for _, row := range rows {
		content := replacer.Replace(row.Content)
		if content == row.Content {
			continue
		}
		if err := s.db.Table(table).
			Where("id = ?", row.ID).
			UpdateColumn("content", content).
			Error; err != nil {
			return updated, fmt.Errorf("rewrite %s content (id=%d): %w", table, row.ID, err)
		}
		updated++
	}

	return updated, nil
}
