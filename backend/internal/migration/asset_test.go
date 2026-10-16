package migration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	// pngPayload starts with the PNG magic bytes, so http.DetectContentType
	// reports image/png no matter which extension the key carries.
	pngPayload = "\x89PNG\r\n\x1a\nfake-image-bytes"
	// textPayload sniffs as text/plain, which maps to BlobTypeUnknown.
	textPayload = "hello, world"
)

// newMigrationService returns a migration service backed by an in-memory
// schema-migrated database and a local storage rooted in a temp directory, so
// tests exercise the same read-the-real-file path production uses.
func newMigrationService(t *testing.T) (*Service, *gorm.DB, storage.Storage) {
	t.Helper()

	db := newAutoMigratedDB(t)
	local := storage.NewLocalStorage(t.TempDir())

	return NewService(Deps{DB: db, Storage: local}), db, local
}

// putObject writes content into the storage backend under key, so the migration
// finds a real file exactly where LocalStorage.Open will look for it.
func putObject(t *testing.T, s storage.Storage, key, content string) {
	t.Helper()

	if err := s.Put(context.Background(), key, strings.NewReader(content), int64(len(content)), ""); err != nil {
		t.Fatalf("put object %q: %v", key, err)
	}
}

// seedMediaFile inserts a legacy media_files row. Passing an empty storageKey
// reproduces a row written before keys were recorded.
func seedMediaFile(t *testing.T, db *gorm.DB, mediaFile model.MediaFile) model.MediaFile {
	t.Helper()

	if err := db.Create(&mediaFile).Error; err != nil {
		t.Fatalf("seed media file (url=%q): %v", mediaFile.URL, err)
	}

	return mediaFile
}

func countRows(t *testing.T, db *gorm.DB, value any) int64 {
	t.Helper()

	var count int64
	if err := db.Model(value).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", value, err)
	}

	return count
}

func findBlobByKey(t *testing.T, db *gorm.DB, key string) (model.Blob, bool) {
	t.Helper()

	var blob model.Blob
	switch err := db.Where("storage_key = ?", key).First(&blob).Error; {
	case err == nil:
		return blob, true
	case !errors.Is(err, gorm.ErrRecordNotFound):
		t.Fatalf("find blob %q: %v", key, err)
	}

	return model.Blob{}, false
}

// TestBackfillAssetBlob_NoMediaFileTable covers a database that predates the
// media_files table: nothing to migrate, and in particular no query against a
// missing table.
func TestBackfillAssetBlob_NoMediaFileTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Storage is nil on purpose: a table that is absent must short-circuit
	// before any file backend is touched.
	if err := NewService(Deps{DB: db}).backfillAssetBlob(); err != nil {
		t.Fatalf("backfill without media_files table: %v", err)
	}
}

// TestBackfillAssetBlob_NoMediaFiles covers an installation that has the table
// but never uploaded anything.
func TestBackfillAssetBlob_NoMediaFiles(t *testing.T) {
	svc, db, _ := newMigrationService(t)

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill with no media files: %v", err)
	}

	if count := countRows(t, db, &model.Blob{}); count != 0 {
		t.Errorf("expected 0 blobs, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 0 {
		t.Errorf("expected 0 assets, got %d", count)
	}
}

// TestBackfillAssetBlob_MigratesLegacyRowFromURL is the main path: a row from
// before keys were recorded, whose URL is the only place the key lives.
func TestBackfillAssetBlob_MigratesLegacyRowFromURL(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "0ae2915d-88c2-4e2e-be99-40282e922bf2.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{
		URL:    "/uploads/" + key,
		Size:   int64(len(pngPayload)),
		Type:   "unknown",
		UserID: 7,
	})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.MimeType != "image/png" {
		t.Errorf("expected MimeType image/png, got %q", blob.MimeType)
	}
	if blob.Type != model.BlobTypeImage {
		t.Errorf("expected Type image, got %q", blob.Type)
	}
	if blob.Size != int64(len(pngPayload)) {
		t.Errorf("expected Size %d, got %d", len(pngPayload), blob.Size)
	}

	var asset model.Asset
	if err := db.First(&asset).Error; err != nil {
		t.Fatalf("find asset: %v", err)
	}
	if asset.BlobID != blob.ID {
		t.Errorf("expected Asset.BlobID %d, got %d", blob.ID, asset.BlobID)
	}
	if asset.OriginalName != key {
		t.Errorf("expected OriginalName %q, got %q", key, asset.OriginalName)
	}
	if asset.UserID != 7 {
		t.Errorf("expected UserID 7, got %d", asset.UserID)
	}
	if asset.ID == 0 {
		t.Error("expected the asset to be persisted with an ID")
	}
}

// TestBackfillAssetBlob_PrefersStorageKeyOverURL covers rows that already carry
// a key: the URL must not be consulted at all.
func TestBackfillAssetBlob_PrefersStorageKeyOverURL(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "0ee4c663-dbc8-4f11-99cc-8d66c0bcd088.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{
		URL:        "https://example.invalid/other/file.png",
		StorageKey: key,
		Size:       int64(len(pngPayload)),
	})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if _, ok := findBlobByKey(t, db, key); !ok {
		t.Errorf("expected the stored StorageKey to be used, found no blob for %q", key)
	}
}

// TestBackfillAssetBlob_MigratesEveryRow covers a batch: every row is converted,
// and each key lands on its own blob.
func TestBackfillAssetBlob_MigratesEveryRow(t *testing.T) {
	svc, db, local := newMigrationService(t)

	keys := []string{
		"1ab74e4a-74d4-4be4-8dc0-8f8630906c39.png",
		"1c8b8568-fbf5-4ca6-86d1-35ce574400f7.png",
		"2424366d-cd43-4c85-a280-31f7ce91eb56.webp",
	}
	for _, key := range keys {
		putObject(t, local, key, pngPayload)
		seedMediaFile(t, db, model.MediaFile{
			URL:    "/uploads/" + key,
			Size:   int64(len(pngPayload)),
			UserID: 3,
		})
	}

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if count := countRows(t, db, &model.Blob{}); count != int64(len(keys)) {
		t.Errorf("expected %d blobs, got %d", len(keys), count)
	}
	if count := countRows(t, db, &model.Asset{}); count != int64(len(keys)) {
		t.Errorf("expected %d assets, got %d", len(keys), count)
	}
	for _, key := range keys {
		if _, ok := findBlobByKey(t, db, key); !ok {
			t.Errorf("expected a blob for key %q", key)
		}
	}
}

// TestBackfillAssetBlob_SizeComesFromTheFile pins that Blob.Size reflects the
// bytes on disk rather than a stale MediaFile.Size, which legacy rows may not
// have recorded at all.
func TestBackfillAssetBlob_SizeComesFromTheFile(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "31f7759d-c21e-4eaa-9005-40d887b0cd04.jpg"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{
		URL:    "/uploads/" + key,
		Size:   999_999, // wrong on purpose
		UserID: 1,
	})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.Size != int64(len(pngPayload)) {
		t.Errorf("expected Size %d from the file, got %d", len(pngPayload), blob.Size)
	}
}

// TestBackfillAssetBlob_MigratesRowWithUnrecordedSize covers the oldest rows,
// whose size column was never filled in.
func TestBackfillAssetBlob_MigratesRowWithUnrecordedSize(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "3d7437fb-06eb-4246-ae21-bfc2ae2e0bf8.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: 0})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.Size != int64(len(pngPayload)) {
		t.Errorf("expected Size %d from the file, got %d", len(pngPayload), blob.Size)
	}
}

// TestBackfillAssetBlob_TypeComesFromContentNotExtension pins that the type is
// sniffed, not taken from the key: image bytes behind a .mp4 key stay an image.
func TestBackfillAssetBlob_TypeComesFromContentNotExtension(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "4844ea1d-060e-4957-ad31-e11fac2f9806.mp4"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: int64(len(pngPayload))})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.Type != model.BlobTypeImage {
		t.Errorf("expected Type image from the sniffed content, got %q", blob.Type)
	}
	if blob.MimeType != "image/png" {
		t.Errorf("expected MimeType image/png, got %q", blob.MimeType)
	}
}

// TestBackfillAssetBlob_TypeIsUnknownForNonMediaContent covers a text file: it
// migrates, but the type stays unknown.
func TestBackfillAssetBlob_TypeIsUnknownForNonMediaContent(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "d41d8cd98f00b204e9800998ecf8427e.txt"
	putObject(t, local, key, textPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: int64(len(textPayload))})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.Type != model.BlobTypeUnknown {
		t.Errorf("expected Type unknown, got %q", blob.Type)
	}
	if !strings.HasPrefix(blob.MimeType, "text/plain") {
		t.Errorf("expected a text/* MimeType, got %q", blob.MimeType)
	}
}

// TestBackfillAssetBlob_ExtensionlessKey covers a key with no extension: the
// file is still migrated, sniffed as binary content.
func TestBackfillAssetBlob_ExtensionlessKey(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "9f8e7d6c-5b4a-3210-fedc-ba9876543210"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.Size != int64(len(pngPayload)) {
		t.Errorf("expected Size %d, got %d", len(pngPayload), blob.Size)
	}
}

// TestBackfillAssetBlob_SkipsMissingObject covers a row whose file is gone:
// it must be skipped without failing the migration.
func TestBackfillAssetBlob_SkipsMissingObject(t *testing.T) {
	svc, db, _ := newMigrationService(t)

	const key = "37f01d7d-2d9a-4b31-9c3e-0d7e0a1b2c3d.png"
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: 1024})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("a missing object must not fail the migration: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 0 {
		t.Errorf("expected 0 blobs, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 0 {
		t.Errorf("expected 0 assets, got %d", count)
	}
}

// TestBackfillAssetBlob_SkipsEmptyFile covers a zero-byte object: it has no
// header to sniff, so the row is skipped instead of migrated with a bogus type.
func TestBackfillAssetBlob_SkipsEmptyFile(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "c0ffee00-dead-beef-cafe-000000000000.png"
	putObject(t, local, key, "")
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: 0})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("an empty file must not fail the migration: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 0 {
		t.Errorf("expected 0 blobs, got %d", count)
	}
}

// TestBackfillAssetBlob_SkipsUnusableURL covers rows that carry no addressable
// key at all: an empty URL and a bare scheme-less junk value.
func TestBackfillAssetBlob_SkipsUnusableURL(t *testing.T) {
	svc, db, _ := newMigrationService(t)

	seedMediaFile(t, db, model.MediaFile{URL: "", Size: 10})
	seedMediaFile(t, db, model.MediaFile{URL: "/", Size: 10})
	seedMediaFile(t, db, model.MediaFile{URL: "not-a-url", Size: 10})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("an unusable URL must not fail the migration: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 0 {
		t.Errorf("expected 0 blobs, got %d", count)
	}
}

// TestBackfillAssetBlob_SkipsAbsoluteURL covers a row still hosted on S3: the
// migration only handles the local backend, so the row is skipped. It must not
// be resolved against the local files.
func TestBackfillAssetBlob_SkipsAbsoluteURL(t *testing.T) {
	svc, db, _ := newMigrationService(t)

	seedMediaFile(t, db, model.MediaFile{
		URL:  "https://bucket.s3.eu-central-1.amazonaws.com/media/1ab74e4a-74d4-4be4-8dc0-8f8630906c39.png",
		Size: 2048,
	})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("an S3-hosted row must not fail the migration: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 0 {
		t.Errorf("expected 0 blobs for an S3-hosted row, got %d", count)
	}
}

// TestBackfillAssetBlob_DoesNotClaimLocalFileForAbsoluteURL is the trap: an S3
// URL whose last segment happens to match a file that exists in the local
// backend must not be migrated onto that unrelated file.
func TestBackfillAssetBlob_DoesNotClaimLocalFileForAbsoluteURL(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "5d41402abc4b2a76b9719d911017c592.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{
		URL:  "https://bucket.s3.eu-central-1.amazonaws.com/media/" + key,
		Size: 2048,
	})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if _, ok := findBlobByKey(t, db, key); ok {
		t.Error("an S3-hosted URL must not claim the unrelated local file with the same name")
	}
	if count := countRows(t, db, &model.Asset{}); count != 0 {
		t.Errorf("expected 0 assets, got %d", count)
	}
}

// TestBackfillAssetBlob_SkipsTraversingURL covers a hostile stored URL: it must
// not escape the storage root.
func TestBackfillAssetBlob_SkipsTraversingURL(t *testing.T) {
	svc, db, local := newMigrationService(t)

	putObject(t, local, "secret.txt", textPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/../../secret.txt", Size: 5})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if _, ok := findBlobByKey(t, db, "secret.txt"); ok {
		t.Error("a traversing URL must not resolve to a file outside the uploads tree")
	}
}

// TestBackfillAssetBlob_KeepsMediaFileTable pins that the legacy table survives
// this migration: its rows are still referenced by unmigrated content.
func TestBackfillAssetBlob_KeepsMediaFileTable(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "7c9e6679-7425-40de-944b-e07fc1f90ae7.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: int64(len(pngPayload))})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if !db.Migrator().HasTable(&model.MediaFile{}) {
		t.Error("media_files must not be dropped by this migration")
	}
	if count := countRows(t, db, &model.MediaFile{}); count != 1 {
		t.Errorf("expected the legacy row to remain, got %d rows", count)
	}
}

// TestBackfillAssetBlob_KeepsRowWithoutUser covers media whose uploader row is
// gone: the bytes are still worth keeping, so the asset is recorded with the
// orphaned owner id rather than dropped.
func TestBackfillAssetBlob_KeepsRowWithoutUser(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "a3f1c2d4-5b6e-4f70-8192-a3b4c5d6e7f8.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{
		URL:    "/uploads/" + key,
		Size:   int64(len(pngPayload)),
		UserID: 4242, // no such user
	})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	var asset model.Asset
	if err := db.First(&asset).Error; err != nil {
		t.Fatalf("expected an asset for orphaned media: %v", err)
	}
	if asset.UserID != 4242 {
		t.Errorf("expected UserID 4242, got %d", asset.UserID)
	}
}

// TestBackfillAssetBlob_MigratesSharedObjectOnce covers two legacy rows pointing
// at the same file: the unique index on Blob.StorageKey means only one blob.
func TestBackfillAssetBlob_MigratesSharedObjectOnce(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "b4f2d3e5-6c7f-4081-9203-b4c5d6e7f809.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, UserID: 1})
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, UserID: 2})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 1 {
		t.Errorf("expected 1 blob for the shared object, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 1 {
		t.Errorf("expected 1 asset for the shared object, got %d", count)
	}
}

// TestBackfillAssetBlob_SkipsAlreadyMigratedKey covers a blob that already
// exists for the key: re-running must not insert a second one.
func TestBackfillAssetBlob_SkipsAlreadyMigratedKey(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "c5e3d4f6-7d80-4192-9304-c5d6e7f80912.png"
	putObject(t, local, key, pngPayload)
	if err := db.Create(&model.Blob{
		StorageKey: key,
		MimeType:   "image/png",
		Type:       model.BlobTypeImage,
		Size:       int64(len(pngPayload)),
	}).Error; err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: int64(len(pngPayload))})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 1 {
		t.Errorf("expected the existing blob to be reused, got %d blobs", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 0 {
		t.Errorf("expected 0 assets, got %d", count)
	}
}

// TestBackfillAssetBlob_DoesNotResurrectSoftDeletedBlob covers a blob an admin
// deleted: it still occupies the unique index, so the migration must not try to
// re-insert it (which would fail) and must not revive it either.
func TestBackfillAssetBlob_DoesNotResurrectSoftDeletedBlob(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "d6f4e5a7-8e91-4a2b-8415-d6e7f8091223.png"
	putObject(t, local, key, pngPayload)
	blob := model.Blob{
		StorageKey: key,
		MimeType:   "image/png",
		Type:       model.BlobTypeImage,
		Size:       int64(len(pngPayload)),
	}
	if err := db.Create(&blob).Error; err != nil {
		t.Fatalf("seed blob: %v", err)
	}
	if err := db.Delete(&blob).Error; err != nil {
		t.Fatalf("soft delete blob: %v", err)
	}
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, Size: int64(len(pngPayload))})

	if err := svc.backfillAssetBlob(); err != nil {
		t.Fatalf("a soft-deleted blob must not fail the migration: %v", err)
	}
	if count := countRows(t, db, &model.Blob{}); count != 0 {
		t.Errorf("expected 0 live blobs, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 0 {
		t.Errorf("expected 0 assets, got %d", count)
	}

	var softDeleted model.Blob
	if err := db.Unscoped().Where("storage_key = ?", key).First(&softDeleted).Error; err != nil {
		t.Fatalf("expected the soft-deleted blob to still exist: %v", err)
	}
	if !softDeleted.DeletedAt.Valid {
		t.Error("the soft-deleted blob must not be revived by the migration")
	}
}

// TestBackfillAssetBlob_SurvivesRepeatedRuns covers the migration running again
// on every boot: a full run must migrate once and stay a no-op afterwards.
func TestBackfillAssetBlob_SurvivesRepeatedRuns(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "e7a5f6b8-9fa2-4b3c-9526-e7f809122334.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, UserID: 5, Size: 1})

	for run := range 3 {
		if err := svc.backfillAssetBlob(); err != nil {
			t.Fatalf("run %d: %v", run+1, err)
		}
	}

	if count := countRows(t, db, &model.Blob{}); count != 1 {
		t.Errorf("expected 1 blob after 3 runs, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 1 {
		t.Errorf("expected 1 asset after 3 runs, got %d", count)
	}
}

// TestMigrateDB_RepeatedStartsMigrateMediaOnce exercises the real startup path:
// MigrateDB runs autoMigrate, the moderation switch, the slug backfill and the
// media backfill, and it must be safe to run again on the next boot.
func TestMigrateDB_RepeatedStartsMigrateMediaOnce(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const key = "f8b6a7c9-0ab3-4c4d-a637-f80912233445.png"
	putObject(t, local, key, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + key, UserID: 5, Size: 1})

	for run := range 3 {
		if err := svc.MigrateDB(); err != nil {
			t.Fatalf("start %d: %v", run+1, err)
		}
	}

	if count := countRows(t, db, &model.Blob{}); count != 1 {
		t.Errorf("expected 1 blob after 3 startups, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 1 {
		t.Errorf("expected 1 asset after 3 startups, got %d", count)
	}

	blob, ok := findBlobByKey(t, db, key)
	if !ok {
		t.Fatalf("expected a blob for key %q", key)
	}
	if blob.Size != int64(len(pngPayload)) {
		t.Errorf("expected Size %d, got %d", len(pngPayload), blob.Size)
	}
}

// TestMigrateDB_RepeatedStartsSurviveUnmigratableRows covers a boot where some
// rows cannot be migrated: they are skipped, and the next boot must still
// succeed rather than refusing to start.
func TestMigrateDB_RepeatedStartsSurviveUnmigratableRows(t *testing.T) {
	svc, db, local := newMigrationService(t)

	const missing = "0918273645-1627-4e8e-b748-f80912233455.png"
	const empty = "1a2b3c4d-5e6f-4071-8293-a4b5c6d7e8f9.png"
	const good = "2b3c4d5e-6f70-4182-93a4-b5c6d7e8f901.png"
	putObject(t, local, empty, "")
	putObject(t, local, good, pngPayload)
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + missing, UserID: 1})
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + empty, UserID: 1})
	seedMediaFile(t, db, model.MediaFile{URL: "", UserID: 1})
	seedMediaFile(t, db, model.MediaFile{URL: "/uploads/" + good, UserID: 1, Size: 1})

	for run := range 3 {
		if err := svc.MigrateDB(); err != nil {
			t.Fatalf("start %d must not fail on unmigratable rows: %v", run+1, err)
		}
	}

	if count := countRows(t, db, &model.Blob{}); count != 1 {
		t.Errorf("expected only the migratable row to produce a blob, got %d", count)
	}
	if count := countRows(t, db, &model.Asset{}); count != 1 {
		t.Errorf("expected 1 asset, got %d", count)
	}
	if _, ok := findBlobByKey(t, db, good); !ok {
		t.Errorf("expected a blob for the migratable key %q", good)
	}
}

// TestMigrateDB_MigratesMediaOfAnExistingInstallation covers the whole shape of
// an upgrade: legacy rows and their files already present, no assets or blobs
// yet.
func TestMigrateDB_MigratesMediaOfAnExistingInstallation(t *testing.T) {
	svc, db, local := newMigrationService(t)

	keys := []string{
		"3c4d5e6f-7081-4293-a4b5-c6d7e8f901122.png",
		"4d5e6f70-8192-43a4-b5c6-d7e8f9011223.jpg",
	}
	for _, key := range keys {
		putObject(t, local, key, pngPayload)
	}
	for i, key := range keys {
		seedMediaFile(t, db, model.MediaFile{
			URL:        "/uploads/" + key,
			Size:       int64(len(pngPayload)),
			Type:       "unknown",
			UserID:     uint(i + 1),
			StorageKey: "",
		})
	}

	if err := svc.MigrateDB(); err != nil {
		t.Fatalf("MigrateDB: %v", err)
	}

	if count := countRows(t, db, &model.Blob{}); count != int64(len(keys)) {
		t.Errorf("expected %d blobs, got %d", len(keys), count)
	}
	if count := countRows(t, db, &model.Asset{}); count != int64(len(keys)) {
		t.Errorf("expected %d assets, got %d", len(keys), count)
	}
	for i, key := range keys {
		asset := model.Asset{}
		if err := db.Where("original_name = ?", key).First(&asset).Error; err != nil {
			t.Fatalf("expected an asset named %q: %v", key, err)
		}
		if asset.UserID != uint(i+1) {
			t.Errorf("expected UserID %d for %q, got %d", i+1, key, asset.UserID)
		}
	}
}
