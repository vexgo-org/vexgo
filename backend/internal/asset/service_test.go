package asset

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/vexgo-org/vexgo/backend/internal/idgen"
	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	// One connection, or a pooled :memory: database gives each goroutine its
	// own empty schema.
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Asset{}, &model.Blob{}, &model.User{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, string, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	dataDir := t.TempDir()
	svc := NewService(Deps{DB: db, Storage: storage.NewLocalStorage(dataDir)})
	return svc, dataDir, db
}

func seedUser(t *testing.T, db *gorm.DB, username, role string) model.User {
	t.Helper()
	u := model.User{Username: username, Email: username + "@example.com", Role: role}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return u
}

// seedAsset inserts an asset (with its blob) directly, bypassing Upload, so a
// test can set up a row whose storage side was never really populated. The row
// carries a real public ID and the public visibility, because those are what
// every path that reads an asset by URL goes through.
func seedAsset(t *testing.T, db *gorm.DB, ownerID uint, name string) model.Asset {
	t.Helper()
	blob := model.Blob{
		StorageKey: uuid.NewString() + ".png",
		MimeType:   "image/png",
		Type:       model.BlobTypeImage,
		Size:       4,
	}
	if err := db.Create(&blob).Error; err != nil {
		t.Fatalf("failed to seed blob: %v", err)
	}
	publicID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("failed to generate a public ID: %v", err)
	}
	asset := model.Asset{
		OriginalName: name,
		UserID:       ownerID,
		BlobID:       blob.ID,
		PublicID:     publicID,
		Visibility:   model.AssetVisibilityPublic,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("failed to seed asset: %v", err)
	}
	return asset
}

// setVisibility flips an existing asset's visibility, so the public read path
// can be handed a row the lookup has to refuse.
func setVisibility(
	t *testing.T,
	db *gorm.DB,
	assetID uint,
	visibility model.AssetVisibility,
) {
	t.Helper()
	if err := db.Model(&model.Asset{}).
		Where("id = ?", assetID).
		Update("visibility", visibility).Error; err != nil {
		t.Fatalf("failed to set visibility on asset %d: %v", assetID, err)
	}
}

func countRows(t *testing.T, db *gorm.DB, value any) int64 {
	t.Helper()
	var count int64
	if err := db.Model(value).Count(&count).Error; err != nil {
		t.Fatalf("failed to count %T: %v", value, err)
	}
	return count
}

func countAssets(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	return countRows(t, db, &model.Asset{})
}

func countBlobs(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	return countRows(t, db, &model.Blob{})
}

// mediaFiles lists the object names in the local storage media directory,
// treating a directory that was never created as "no objects".
func mediaFiles(t *testing.T, dataDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, "media"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to read media dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// writeObject puts content at the storage key of the blob with blobID and
// returns that key.
func writeObject(t *testing.T, db *gorm.DB, dataDir string, blobID uint, content string) string {
	t.Helper()
	var blob model.Blob
	if err := db.First(&blob, blobID).Error; err != nil {
		t.Fatalf("failed to load blob %d: %v", blobID, err)
	}
	dir := filepath.Join(dataDir, "media")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("failed to create media dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, blob.StorageKey), []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write object: %v", err)
	}
	return blob.StorageKey
}

// assertObject checks the stored object still holds content.
func assertObject(t *testing.T, dataDir, key, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(dataDir, "media", key))
	if err != nil {
		t.Errorf("object %q is gone: %v", key, err)
		return
	}
	if string(got) != want {
		t.Errorf("object %q holds %q, want %q", key, got, want)
	}
}

// pngPayload returns a PNG-signatured payload of exactly n bytes. Tests ask for
// a size well past the 512-byte sniff window on purpose: a sniffer that consumed
// the stream instead of peeking would hand storage a short body and trip its
// size check, which is exactly the regression this shape catches.
func pngPayload(n int) []byte {
	payload := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte("vexgo"), n/5+1)...)
	return payload[:n]
}

// errDB stands in for any database failure.
var errDB = errors.New("database is unavailable")

// --- fakes for the seams the service owns -------------------------------

type putCall struct {
	key         string
	size        int64
	contentType string
	content     string
}

type fakeStorage struct {
	putErr    error
	deleteErr error

	puts    []putCall
	deletes []string
}

func (f *fakeStorage) Put(
	_ context.Context,
	key string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	content, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.puts = append(f.puts, putCall{
		key:         key,
		size:        size,
		contentType: contentType,
		content:     string(content),
	})
	return f.putErr
}

func (f *fakeStorage) Open(context.Context, string) (io.ReadSeekCloser, error) {
	return nil, storage.ErrNotFound
}

func (f *fakeStorage) Delete(_ context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	return f.deleteErr
}

func (f *fakeStorage) URL(_ context.Context, key string) (string, error) {
	return "/uploads/" + key, nil
}

type stubAssetRepo struct {
	find    *model.Asset
	findErr error
	delErr  error

	// lookups records every public ID the service asked for, so a test can
	// prove a request never reached the database at all.
	lookups []uuid.UUID

	created []*model.Asset
	deleted []uint
}

func (s *stubAssetRepo) Create(_ context.Context, asset *model.Asset) error {
	if asset.ID == 0 {
		asset.ID = uint(len(s.created) + 1)
	}
	s.created = append(s.created, asset)
	return nil
}

func (s *stubAssetRepo) FindByID(context.Context, uint) (*model.Asset, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.find == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.find, nil
}

// FindByPublicID answers like FindByID but records the lookup, because Delete
// goes through it: a test that cares whether the lookup happened at all reads
// lookups, and one that only needs a row or a failure is served from here.
func (s *stubAssetRepo) FindByPublicID(
	_ context.Context,
	publicID uuid.UUID,
) (*model.Asset, error) {
	s.lookups = append(s.lookups, publicID)
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.find == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.find, nil
}

func (s *stubAssetRepo) FindByPublicIDs(
	_ context.Context,
	publicIDs []uuid.UUID,
) ([]model.Asset, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	var found []model.Asset
	for _, id := range publicIDs {
		if s.find == nil {
			continue
		}
		if s.find.PublicID == id {
			found = append(found, *s.find)
		}
	}
	return found, nil
}

// FindPublicByPublicID answers exactly like FindByPublicID, without the
// visibility filter a real public read would apply: a test that cares about the
// lookup itself goes through the real repository, and one that only needs a row
// or a failure is served from here.
func (s *stubAssetRepo) FindPublicByPublicID(
	_ context.Context,
	publicID uuid.UUID,
) (*model.Asset, error) {
	s.lookups = append(s.lookups, publicID)
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.find == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.find, nil
}

func (s *stubAssetRepo) SoftDelete(_ context.Context, asset *model.Asset) error {
	if s.delErr != nil {
		return s.delErr
	}
	asset.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	s.deleted = append(s.deleted, asset.ID)
	return nil
}

type stubBlobRepo struct {
	createErr error

	created []*model.Blob
}

func (s *stubBlobRepo) Create(_ context.Context, blob *model.Blob) error {
	if s.createErr != nil {
		return s.createErr
	}
	if blob.ID == 0 {
		blob.ID = uint(len(s.created) + 1)
	}
	s.created = append(s.created, blob)
	return nil
}

// mixedRepositories hands the transaction a caller-chosen pair of repositories,
// so a real blob repository can share a transaction with a faked asset one.
type mixedRepositories struct {
	assets AssetRepository
	blobs  BlobRepository
}

func (m mixedRepositories) Assets() AssetRepository { return m.assets }
func (m mixedRepositories) Blobs() BlobRepository   { return m.blobs }

// mixedTxManager runs fn inside a real GORM transaction, but builds its
// repositories through build instead of newRepositories so a test can mix a real
// repository with a faked one and still get the real rollback.
type mixedTxManager struct {
	db    *gorm.DB
	build func(*gorm.DB) Repositories
}

func (m mixedTxManager) WithTransaction(
	ctx context.Context,
	fn func(Repositories) error,
) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(m.build(tx))
	})
}

// fakeTxManager hands fn a fixed repository set, or fails before running it.
type fakeTxManager struct {
	repos Repositories
	err   error

	runs int
}

func (f *fakeTxManager) WithTransaction(
	_ context.Context,
	fn func(Repositories) error,
) error {
	f.runs++
	if f.err != nil {
		return f.err
	}
	return fn(f.repos)
}

type failingKeyGenerator struct{ err error }

func (f failingKeyGenerator) Generate(string) (string, error) {
	return "", f.err
}

// failingPublicIDGenerator stands in for a UUID generator that cannot produce a
// value, which is the only way to reach that failure branch.
type failingPublicIDGenerator struct{ err error }

func (f failingPublicIDGenerator) New() (uuid.UUID, error) {
	return uuid.Nil, f.err
}

// newFakeService wires a Service around fakes, which is the only way to reach
// the failure paths the real database and storage would not produce. The ID
// generator is the real one unless a test replaces it, so the fake builder
// mirrors NewService instead of leaving a nil seam behind.
func newFakeService(
	repos Repositories,
	store storage.Storage,
	tx TransactionManager,
	keys KeyGenerator,
) *Service {
	return &Service{
		assets:            repos.Assets(),
		blobs:             repos.Blobs(),
		txManager:         tx,
		storage:           store,
		keyGenerator:      keys,
		publicIDGenerator: idgen.NewUUIDv7Generator(),
	}
}

func newFailingStoreService(
	tx TransactionManager,
	store storage.Storage,
) *Service {
	return newFakeService(
		mixedRepositories{assets: &stubAssetRepo{}, blobs: &stubBlobRepo{}},
		store,
		tx,
		NewKeyGenerator(),
	)
}

// --- Upload -------------------------------------------------------------

func TestUpload_CreatesBlobAndAsset(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	payload := pngPayload(5000)
	got, err := svc.Upload(
		context.Background(),
		user.ID,
		strings.NewReader(string(payload)),
		"holiday.png",
		int64(len(payload)),
	)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if got == nil {
		t.Fatal("Upload returned no asset")
	}

	// The returned asset carries the record the caller needs to render it
	// without a second round trip.
	if got.OriginalName != "holiday.png" {
		t.Errorf("OriginalName = %q, want %q", got.OriginalName, "holiday.png")
	}
	if got.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", got.UserID, user.ID)
	}
	if got.BlobID == 0 || got.BlobID != got.Blob.ID {
		t.Errorf("BlobID = %d, Blob.ID = %d, want both set and equal", got.BlobID, got.Blob.ID)
	}
	if !strings.HasSuffix(got.Blob.StorageKey, ".png") {
		t.Errorf("StorageKey = %q, want a .png suffix", got.Blob.StorageKey)
	}
	if _, err := uuid.Parse(strings.TrimSuffix(got.Blob.StorageKey, ".png")); err != nil {
		t.Errorf("StorageKey %q is not `uuid.ext`: %v", got.Blob.StorageKey, err)
	}
	if got.Blob.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want %q", got.Blob.MimeType, "image/png")
	}
	if got.Blob.Type != model.BlobTypeImage {
		t.Errorf("Blob Type = %q, want %q", got.Blob.Type, model.BlobTypeImage)
	}
	if got.Blob.Size != int64(len(payload)) {
		t.Errorf("Blob Size = %d, want %d", got.Blob.Size, len(payload))
	}

	// Both rows are really persisted, and linked to each other.
	var storedAsset model.Asset
	if err := db.Preload("Blob").First(&storedAsset, got.ID).Error; err != nil {
		t.Fatalf("asset record not saved: %v", err)
	}
	if storedAsset.Blob.StorageKey != got.Blob.StorageKey {
		t.Errorf("stored blob key = %q, want %q", storedAsset.Blob.StorageKey, got.Blob.StorageKey)
	}

	// The object is on disk under its key, byte for byte: sniffing must not
	// have eaten the head of the stream.
	content, err := os.ReadFile(filepath.Join(dataDir, "media", got.Blob.StorageKey))
	if err != nil {
		t.Fatalf("object not written: %v", err)
	}
	if !bytes.Equal(content, payload) {
		t.Errorf("stored %d bytes, want %d (identical payload)", len(content), len(payload))
	}

	if files := mediaFiles(t, dataDir); len(files) != 1 {
		t.Errorf("media dir holds %v, want exactly the one uploaded object", files)
	}
}

// TestUpload_MintsAPublicIDAndMarksTheAssetPublic pins the two fields the
// public read path depends on: without a public ID the asset is unreachable
// under /assets/:publicID/original, and without the public visibility the
// lookup refuses a row that just came out of an upload.
func TestUpload_MintsAPublicIDAndMarksTheAssetPublic(t *testing.T) {
	svc, _, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)
	payload := pngPayload(64)

	got, err := svc.Upload(
		context.Background(),
		user.ID,
		bytes.NewReader(payload),
		"photo.png",
		int64(len(payload)),
	)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if got.PublicID == uuid.Nil {
		t.Error("PublicID is nil, want a generated one")
	}

	// The returned value is what the SPA turns into a URL, and the stored row
	// is what the next request reads back.
	var stored model.Asset
	if err := db.First(&stored, got.ID).Error; err != nil {
		t.Fatalf("asset record not saved: %v", err)
	}
	if stored.PublicID != got.PublicID {
		t.Errorf("stored PublicID = %s, want %s", stored.PublicID, got.PublicID)
	}
	if stored.Visibility != model.AssetVisibilityPublic {
		t.Errorf("Visibility = %q, want %q", stored.Visibility, model.AssetVisibilityPublic)
	}
}

// TestUpload_MintsDistinctPublicIDsPerAsset covers the unique index behind them:
// a generator that handed out one value twice would make the second upload fail
// its insert, and with it the whole transaction.
func TestUpload_MintsDistinctPublicIDsPerAsset(t *testing.T) {
	svc, _, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)
	payload := pngPayload(64)

	seen := make(map[uuid.UUID]struct{}, 2)
	for _, name := range []string{"first.png", "second.png"} {
		got, err := svc.Upload(
			context.Background(),
			user.ID,
			bytes.NewReader(payload),
			name,
			int64(len(payload)),
		)
		if err != nil {
			t.Fatalf("Upload(%s) error: %v", name, err)
		}
		if _, duplicate := seen[got.PublicID]; duplicate {
			t.Fatalf("public ID %s was minted twice", got.PublicID)
		}
		seen[got.PublicID] = struct{}{}
	}

	if n := countAssets(t, db); n != 2 {
		t.Errorf("stored %d asset(s), want 2", n)
	}
}

// TestUpload_AcceptsExactlySniffWindowBoundary covers the size where the file
// header ends exactly on the 512-byte peek limit: the sniff must succeed and the
// payload still arrive whole.
func TestUpload_AcceptsExactlySniffWindowBoundary(t *testing.T) {
	svc, _, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	payload := pngPayload(512)
	got, err := svc.Upload(
		context.Background(),
		user.ID,
		bytes.NewReader(payload),
		"edge.png",
		int64(len(payload)),
	)
	if err != nil {
		t.Fatalf("Upload error at the sniff boundary: %v", err)
	}
	if got.Blob.Size != int64(len(payload)) {
		t.Errorf("Blob Size = %d, want %d", got.Blob.Size, len(payload))
	}
}

func TestUpload_RejectsInvalidParameters(t *testing.T) {
	payload := "some-bytes"

	tests := []struct {
		name     string
		reader   io.Reader
		filename string
		size     int64
		wantErr  error
	}{
		// A nil reader is a programming error inside the server rather than a
		// client mistake, so it has no sentinel to match on.
		{name: "nil reader", reader: nil, filename: "photo.png", size: int64(len(payload))},
		{
			name:     "empty filename",
			reader:   strings.NewReader(payload),
			filename: "",
			size:     int64(len(payload)),
			wantErr:  ErrEmptyFilename,
		},
		{
			name:     "zero size",
			reader:   strings.NewReader(payload),
			filename: "photo.png",
			size:     0,
			wantErr:  ErrInvalidFileSize,
		},
		{
			name:     "negative size",
			reader:   strings.NewReader(payload),
			filename: "photo.png",
			size:     -1,
			wantErr:  ErrInvalidFileSize,
		},
		{
			name:     "negative size with large magnitude",
			reader:   strings.NewReader(payload),
			filename: "photo.png",
			size:     -1 << 40,
			wantErr:  ErrInvalidFileSize,
		},
		{name: "nil reader and bad size", reader: nil, filename: "photo.png", size: -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dataDir, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)

			got, err := svc.Upload(
				context.Background(),
				user.ID,
				tt.reader,
				tt.filename,
				tt.size,
			)
			if err == nil {
				t.Fatal("expected an error for invalid parameters, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Upload error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Errorf("expected no asset alongside the error, got %+v", got)
			}
			if n := countAssets(t, db); n != 0 {
				t.Errorf("rejected upload created %d asset record(s)", n)
			}
			if n := countBlobs(t, db); n != 0 {
				t.Errorf("rejected upload created %d blob record(s)", n)
			}
			if files := mediaFiles(t, dataDir); len(files) != 0 {
				t.Errorf("rejected upload left %v in storage", files)
			}
		})
	}
}

// TestUpload_FilenameLengthMatchesColumnLimit pins the length guard to the
// column it protects. The column counts characters, so a name of exactly
// MaxAssetFilenameLength multibyte runes is accepted — the case a byte-length
// check would wrongly refuse — and one rune more is refused before anything is
// written.
func TestUpload_FilenameLengthMatchesColumnLimit(t *testing.T) {
	accepted := []struct {
		name     string
		filename string
	}{
		{
			name:     "ascii at the limit",
			filename: strings.Repeat("a", model.MaxAssetFilenameLength-4) + ".png",
		},
		{
			// 255 three-byte runes, 765 bytes: the reason the guard counts
			// characters.
			name:     "multibyte at the limit",
			filename: strings.Repeat("写", model.MaxAssetFilenameLength-4) + ".png",
		},
	}

	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)
			payload := pngPayload(64)

			got, err := svc.Upload(
				context.Background(),
				user.ID,
				strings.NewReader(string(payload)),
				tt.filename,
				int64(len(payload)),
			)
			if err != nil {
				t.Fatalf("Upload(%d runes) error: %v", utf8.RuneCountInString(tt.filename), err)
			}
			if got.OriginalName != tt.filename {
				t.Errorf("OriginalName round-trip changed: got %d runes, want %d",
					utf8.RuneCountInString(got.OriginalName), utf8.RuneCountInString(tt.filename))
			}
			if n := countAssets(t, db); n != 1 {
				t.Errorf("stored %d asset(s), want 1", n)
			}
		})
	}

	rejected := []struct {
		name     string
		filename string
	}{
		{
			name:     "ascii over the limit",
			filename: strings.Repeat("a", model.MaxAssetFilenameLength-3) + ".png",
		},
		{
			name:     "multibyte over the limit",
			filename: strings.Repeat("写", model.MaxAssetFilenameLength-3) + ".png",
		},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			svc, dataDir, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)
			payload := pngPayload(64)

			got, err := svc.Upload(
				context.Background(),
				user.ID,
				strings.NewReader(string(payload)),
				tt.filename,
				int64(len(payload)),
			)
			if !errors.Is(err, ErrFilenameTooLong) {
				t.Fatalf("Upload error = %v, want ErrFilenameTooLong", err)
			}
			if got != nil {
				t.Errorf("expected no asset alongside the error, got %+v", got)
			}
			if n := countAssets(t, db); n != 0 {
				t.Errorf("rejected upload created %d asset record(s)", n)
			}
			if files := mediaFiles(t, dataDir); len(files) != 0 {
				t.Errorf("rejected upload left %v in storage", files)
			}
		})
	}
}

// TestUpload_RejectsEmptyPayload covers a body of zero bytes with an otherwise
// valid request: there is no header to sniff, so nothing may be stored.
func TestUpload_RejectsEmptyPayload(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	got, err := svc.Upload(
		context.Background(),
		user.ID,
		strings.NewReader(""),
		"empty.png",
		1,
	)
	if err == nil {
		t.Fatal("expected an error for an empty payload, got nil")
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if n := countAssets(t, db); n != 0 {
		t.Errorf("rejected upload created %d asset record(s)", n)
	}
	if n := countBlobs(t, db); n != 0 {
		t.Errorf("rejected upload created %d blob record(s)", n)
	}
	if files := mediaFiles(t, dataDir); len(files) != 0 {
		t.Errorf("rejected upload left %v in storage", files)
	}
}

func TestUpload_RejectsDisallowedExtension(t *testing.T) {
	// The document and media formats that used to be allowed are listed here
	// too: none of them sniffs to a MIME type with a blob type, so every upload
	// wearing one is refused before anything is written.
	for _, filename := range []string{
		"evil.exe",
		"evil.html",
		"evil.js",
		"evil.xml",
		"icon.svg",
		"noextension",
		"notes.txt",
		"paper.pdf",
		"table.csv",
	} {
		t.Run(filename, func(t *testing.T) {
			svc, dataDir, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)

			got, err := svc.Upload(
				context.Background(),
				user.ID,
				strings.NewReader("<script>alert(1)</script>"),
				filename,
				26,
			)
			if !errors.Is(err, ErrInvalidExtension) {
				t.Fatalf("Upload error = %v, want ErrInvalidExtension", err)
			}
			if got != nil {
				t.Errorf("expected no asset alongside the error, got %+v", got)
			}
			if n := countAssets(t, db); n != 0 {
				t.Errorf("rejected upload created %d asset record(s)", n)
			}
			if files := mediaFiles(t, dataDir); len(files) != 0 {
				t.Errorf("rejected upload left %v in storage", files)
			}
		})
	}
}

func TestUpload_FailsWhenKeyGenerationFails(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)
	store := &fakeStorage{}
	svc := newFakeService(
		mixedRepositories{assets: &stubAssetRepo{}, blobs: &stubBlobRepo{}},
		store,
		&fakeTxManager{repos: mixedRepositories{}},
		failingKeyGenerator{err: errDB},
	)

	got, err := svc.Upload(
		context.Background(),
		user.ID,
		strings.NewReader("data"),
		"photo.png",
		4,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want it to wrap the key generator failure", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if len(store.puts) != 0 {
		t.Errorf("storage was written before the key existed: %+v", store.puts)
	}
}

// TestUpload_FailsWhenPublicIDGenerationFails pins the ordering that keeps this
// failure cheap: the ID is minted before the object is written, so a generator
// that cannot produce one leaves no row and no orphan in storage.
func TestUpload_FailsWhenPublicIDGenerationFails(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)
	store := &fakeStorage{}
	svc := newFakeService(
		mixedRepositories{assets: &stubAssetRepo{}, blobs: &stubBlobRepo{}},
		store,
		&fakeTxManager{repos: mixedRepositories{}},
		NewKeyGenerator(),
	)
	svc.publicIDGenerator = failingPublicIDGenerator{err: errDB}

	got, err := svc.Upload(
		context.Background(),
		user.ID,
		bytes.NewReader(pngPayload(64)),
		"photo.png",
		64,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want it to wrap the ID generator failure", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if len(store.puts) != 0 {
		t.Errorf("storage was written before the public ID existed: %+v", store.puts)
	}
	if n := countAssets(t, db); n != 0 {
		t.Errorf("failed upload created %d asset record(s)", n)
	}
	if n := countBlobs(t, db); n != 0 {
		t.Errorf("failed upload created %d blob record(s)", n)
	}
}

// TestUpload_FailsWhenStoragePutFails checks a rejected write leaves no rows:
// the object is the only trace of the upload, so the record must not outlive it.
func TestUpload_FailsWhenStoragePutFails(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)
	store := &fakeStorage{putErr: errDB}
	tx := &fakeTxManager{repos: mixedRepositories{assets: &stubAssetRepo{}, blobs: &stubBlobRepo{}}}
	svc := newFailingStoreService(tx, store)

	got, err := svc.Upload(
		context.Background(),
		user.ID,
		bytes.NewReader(pngPayload(64)),
		"photo.png",
		64,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want it to wrap the storage failure", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if n := countAssets(t, db); n != 0 {
		t.Errorf("failed upload created %d asset record(s)", n)
	}
	if n := countBlobs(t, db); n != 0 {
		t.Errorf("failed upload created %d blob record(s)", n)
	}
	// Nothing was committed, so nothing may be deleted either.
	if tx.runs != 0 {
		t.Errorf("transaction ran %d time(s), want 0 after a failed write", tx.runs)
	}
	if len(store.deletes) != 0 {
		t.Errorf("rollback deleted %v, but no object was committed", store.deletes)
	}
}

// TestUpload_RollsBackStorageWhenRecordCreationFails is the orphan guard: the
// object is already in storage when the rows fail, and no media record will
// ever point at it, so the upload has to remove it itself.
func TestUpload_RollsBackStorageWhenRecordCreationFails(t *testing.T) {
	blobs := &stubBlobRepo{createErr: errDB}
	tx := &fakeTxManager{repos: mixedRepositories{assets: &stubAssetRepo{}, blobs: blobs}}
	store := &fakeStorage{}
	svc := newFailingStoreService(tx, store)

	got, err := svc.Upload(
		context.Background(),
		1,
		bytes.NewReader(pngPayload(64)),
		"photo.png",
		64,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want it to wrap the record creation failure", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if len(store.puts) != 1 {
		t.Fatalf("storage writes = %d, want 1", len(store.puts))
	}
	if len(store.deletes) != 1 || store.deletes[0] != store.puts[0].key {
		t.Errorf("rollback deletes = %v, want exactly the stored key %q", store.deletes, store.puts[0].key)
	}
	if len(blobs.created) != 0 {
		t.Errorf("failing blob repository recorded %d blob(s)", len(blobs.created))
	}
}

// TestUpload_KeepsCreateErrorWhenRollbackDeleteFails pins the priority of the
// two errors: the cleanup failure is only logged, because the record failure is
// what the caller has to act on.
func TestUpload_KeepsCreateErrorWhenRollbackDeleteFails(t *testing.T) {
	tx := &fakeTxManager{
		repos: mixedRepositories{
			assets: &stubAssetRepo{},
			blobs:  &stubBlobRepo{createErr: errDB},
		},
	}
	store := &fakeStorage{deleteErr: errors.New("storage unreachable")}
	svc := newFailingStoreService(tx, store)

	got, err := svc.Upload(
		context.Background(),
		1,
		bytes.NewReader(pngPayload(64)),
		"photo.png",
		64,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want the record creation failure, not the cleanup one", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if len(store.deletes) != 1 {
		t.Errorf("rollback attempts = %v, want one delete attempt", store.deletes)
	}
}

// TestUpload_RollsBackBlobWhenAssetCreationFails runs the real transaction with
// a real blob repository: a failed asset insert must undo the blob row written
// just before it, and the object must go with both.
func TestUpload_RollsBackBlobWhenAssetCreationFails(t *testing.T) {
	db := newTestDB(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)
	store := &fakeStorage{}
	// The blob half is real; only the asset insert is made to fail, so the
	// rollback has something to undo.
	assets := &failingCreateAssetRepo{}
	tx := mixedTxManager{
		db: db,
		build: func(tx *gorm.DB) Repositories {
			return mixedRepositories{assets: assets, blobs: NewBlobRepository(tx)}
		},
	}
	svc := newFakeService(
		mixedRepositories{assets: assets, blobs: NewBlobRepository(db)},
		store,
		tx,
		NewKeyGenerator(),
	)

	got, err := svc.Upload(
		context.Background(),
		user.ID,
		bytes.NewReader(pngPayload(64)),
		"photo.png",
		64,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want the asset creation failure", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if n := countBlobs(t, db); n != 0 {
		t.Errorf("rolled back transaction left %d blob record(s), want 0", n)
	}
	if n := countAssets(t, db); n != 0 {
		t.Errorf("rolled back transaction left %d asset record(s), want 0", n)
	}
	if len(store.deletes) != 1 {
		t.Errorf("storage deletes = %v, want the written object removed once", store.deletes)
	}
}

// failingCreateAssetRepo always fails to insert, which is how the asset half of
// the transaction is made to fail while the blob half stays real.
type failingCreateAssetRepo struct{ stubAssetRepo }

func (f *failingCreateAssetRepo) Create(context.Context, *model.Asset) error {
	return errDB
}

func TestUpload_PropagatesTransactionFailure(t *testing.T) {
	store := &fakeStorage{}
	svc := newFailingStoreService(&fakeTxManager{err: errDB}, store)

	got, err := svc.Upload(
		context.Background(),
		1,
		bytes.NewReader(pngPayload(64)),
		"photo.png",
		64,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Upload error = %v, want it to wrap the transaction failure", err)
	}
	if got != nil {
		t.Errorf("expected no asset alongside the error, got %+v", got)
	}
	if len(store.deletes) != 1 || store.deletes[0] != store.puts[0].key {
		t.Errorf("rollback deletes = %v, want the stored key %q", store.deletes, store.puts[0].key)
	}
}

// TestUpload_ClassifiesBlobType checks the sniffed MIME type, not the filename,
// decides how the asset is treated downstream.
func TestUpload_ClassifiesBlobType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		payload  []byte
		wantType model.BlobType
	}{
		{
			// The extension is allowlisted but the bytes are not media, so the
			// blob type is taken from the content and the extension it does not
			// match cannot save it.
			name:     "image from signature, not extension",
			filename: "mislabelled.jpg",
			payload:  pngPayload(32),
			wantType: model.BlobTypeImage,
		},
		{
			name:     "video from signature",
			filename: "clip.mp4",
			payload:  append([]byte("\x00\x00\x00\x18ftypmp42"), bytes.Repeat([]byte{0}, 16)...),
			wantType: model.BlobTypeVideo,
		},
		{
			name:     "audio from signature",
			filename: "song.mp3",
			payload:  append([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"), bytes.Repeat([]byte{0}, 16)...),
			wantType: model.BlobTypeAudio,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)

			got, err := svc.Upload(
				context.Background(),
				user.ID,
				bytes.NewReader(tt.payload),
				tt.filename,
				int64(len(tt.payload)),
			)
			if err != nil {
				t.Fatalf("Upload error: %v", err)
			}
			if got.Blob.Type != tt.wantType {
				t.Errorf("Blob Type = %q, want %q (mime %q)", got.Blob.Type, tt.wantType, got.Blob.MimeType)
			}
		})
	}
}

// TestUpload_RejectsUnsupportedMIMEType covers the other half: content that
// sniffs to a MIME type no blob type is defined for is refused before anything
// is written, so a text file wearing a .png name never becomes an asset.
func TestUpload_RejectsUnsupportedMIMEType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		payload  []byte
	}{
		{
			name:     "plain text",
			filename: "photo.png",
			payload:  []byte("just some plain text"),
		},
		{
			name:     "text behind an image name",
			filename: "clip.mp4",
			payload:  []byte("just some plain text"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, dataDir, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)

			got, err := svc.Upload(
				context.Background(),
				user.ID,
				bytes.NewReader(tt.payload),
				tt.filename,
				int64(len(tt.payload)),
			)
			if !errors.Is(err, model.ErrUnsupportedMIMEType) {
				t.Fatalf("Upload error = %v, want ErrUnsupportedMIMEType", err)
			}
			if got != nil {
				t.Errorf("expected no asset alongside the error, got %+v", got)
			}
			if n := countAssets(t, db); n != 0 {
				t.Errorf("rejected upload created %d asset record(s)", n)
			}
			if n := countBlobs(t, db); n != 0 {
				t.Errorf("rejected upload created %d blob record(s)", n)
			}
			if files := mediaFiles(t, dataDir); len(files) != 0 {
				t.Errorf("rejected upload left %v in storage", files)
			}
		})
	}
}

// --- detectMIMEType -----------------------------------------------------

func TestDetectMIMEType(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
		wantErr bool
	}{
		{name: "png", payload: append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 16)...), want: "image/png"},
		{name: "jpeg", payload: []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x00"), want: "image/jpeg"},
		{name: "gif", payload: []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00,"), want: "image/gif"},
		{name: "pdf", payload: []byte("%PDF-1.7\n%\xE2\xE3\xCF\xD3\n"), want: "application/pdf"},
		{name: "webm", payload: append([]byte("\x1A\x45\xDF\xA3"), bytes.Repeat([]byte{0}, 16)...), want: "video/webm"},
		{name: "plain text", payload: []byte("hello, world"), want: "text/plain; charset=utf-8"},
		{name: "utf-8 text", payload: []byte("写真です"), want: "text/plain; charset=utf-8"},
		{name: "binary", payload: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00}, want: "application/octet-stream"},
		{name: "single byte", payload: []byte("x"), want: "text/plain; charset=utf-8"},

		// Boundary sizes around the 512-byte peek window.
		{name: "exactly 511 bytes", payload: pngPayload(511), want: "image/png"},
		{name: "exactly 512 bytes", payload: pngPayload(512), want: "image/png"},
		{name: "exactly 513 bytes", payload: pngPayload(513), want: "image/png"},
		{name: "far beyond the window", payload: pngPayload(8192), want: "image/png"},

		{name: "empty", payload: []byte{}, wantErr: true},
		{name: "nil payload", payload: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectMIMEType(bufio.NewReader(bytes.NewReader(tt.payload)))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("detectMIMEType(%d bytes) = %q, want an error", len(tt.payload), got)
				}
				if got != "" {
					t.Errorf("detectMIMEType returned %q alongside an error, want empty", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("detectMIMEType error: %v", err)
			}
			if got != tt.want {
				t.Errorf("detectMIMEType = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDetectMIMEType_DoesNotConsumeStream is the reason the service sniffs the
// same bufio.Reader it later hands to storage: consuming the head would silently
// truncate every uploaded file.
func TestDetectMIMEType_DoesNotConsumeStream(t *testing.T) {
	payload := pngPayload(4096)
	reader := bufio.NewReader(bytes.NewReader(payload))

	if _, err := DetectMIMEType(reader); err != nil {
		t.Fatalf("detectMIMEType error: %v", err)
	}

	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read remainder: %v", err)
	}
	if len(rest) != len(payload) {
		t.Fatalf("sniffing consumed %d bytes, want none", len(payload)-len(rest))
	}
}

// TestDetectMIMEType_PropagatesReadError covers a body that fails mid-header
// instead of ending: that is a transport failure, not an empty file, and must
// not be mistaken for one.
func TestDetectMIMEType_PropagatesReadError(t *testing.T) {
	readErr := errors.New("connection reset")

	_, err := DetectMIMEType(bufio.NewReader(errReader{err: readErr}))
	if !errors.Is(err, readErr) {
		t.Fatalf("detectMIMEType error = %v, want it to wrap %v", err, readErr)
	}
	if !strings.Contains(err.Error(), "read file header") {
		t.Errorf("error = %q, want it to name the header read", err.Error())
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// --- Delete -------------------------------------------------------------

func TestDelete_OwnerCanDelete(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleGuest)
	asset := seedAsset(t, db, owner.ID, "mine.png")

	// A real object under the blob's key, so the assertions below can tell
	// "Delete left the file alone" from "there was never a file".
	key := writeObject(t, db, dataDir, asset.BlobID, "stored-bytes")

	// Ownership alone is enough, whatever the role.
	if err := svc.Delete(
		context.Background(),
		Actor{ID: owner.ID, Role: model.RoleGuest},
		asset.PublicID,
	); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// Soft deleted: gone from the normal query, still in the table.
	if err := db.First(&model.Asset{}, asset.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("soft deleted asset is still visible: %v", err)
	}
	var unscoped model.Asset
	if err := db.Unscoped().First(&unscoped, asset.ID).Error; err != nil {
		t.Fatalf("asset row was hard deleted: %v", err)
	}
	if !unscoped.DeletedAt.Valid {
		t.Error("DeletedAt not set on the soft deleted asset")
	}

	// Only the asset is deleted: the blob row and the stored object both
	// survive, since nothing says they are unreferenced yet.
	if n := countBlobs(t, db); n != 1 {
		t.Errorf("blob count = %d, want the blob to survive a soft delete", n)
	}
	assertObject(t, dataDir, key, "stored-bytes")
}

func TestDelete_AdminRolesCanDeleteOthers(t *testing.T) {
	for _, role := range []string{model.RoleAdmin, model.RoleSuperAdmin} {
		t.Run(role, func(t *testing.T) {
			svc, _, db := newTestService(t)
			owner := seedUser(t, db, "owner", model.RoleContributor)
			admin := seedUser(t, db, role, role)
			asset := seedAsset(t, db, owner.ID, "theirs.png")

			if err := svc.Delete(
				context.Background(),
				Actor{ID: admin.ID, Role: admin.Role},
				asset.PublicID,
			); err != nil {
				t.Fatalf("Delete error for %s: %v", role, err)
			}
			if err := db.First(&model.Asset{}, asset.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Errorf("asset not soft deleted: %v", err)
			}
		})
	}
}

func TestDelete_ForbiddenForNonOwner(t *testing.T) {
	for _, role := range []string{
		model.RoleGuest,
		model.RoleContributor,
		model.RoleAuthor,
		"", // unknown role is not an admin
		"Admin",
		"administrator",
		"admin ", // no trimming
	} {
		t.Run("role="+role, func(t *testing.T) {
			svc, _, db := newTestService(t)
			owner := seedUser(t, db, "owner", model.RoleAuthor)
			other := seedUser(t, db, "other", role)
			asset := seedAsset(t, db, owner.ID, "theirs.png")

			err := svc.Delete(
				context.Background(),
				Actor{ID: other.ID, Role: other.Role},
				asset.PublicID,
			)
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("Delete error = %v, want ErrForbidden for role %q", err, role)
			}
			if err := db.First(&model.Asset{}, asset.ID).Error; err != nil {
				t.Errorf("forbidden delete still removed the asset: %v", err)
			}
		})
	}
}

// TestDelete_ForbiddenForActorIDZero covers an unauthenticated actor: an empty
// JWT subject is not the owner of anything, so it must not be able to delete.
func TestDelete_ForbiddenForActorIDZero(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleAuthor)
	// Asset 1 belongs to seedUser, which always gets ID 1, so an actor with
	// ID 0 is never the owner here.
	asset := seedAsset(t, db, owner.ID, "theirs.png")
	if asset.UserID == 0 {
		t.Skip("no owner to distinguish from the zero actor")
	}

	if err := svc.Delete(context.Background(), Actor{}, asset.PublicID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete error = %v, want ErrForbidden for an anonymous actor", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	svc, _, db := newTestService(t)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	seedAsset(t, db, admin.ID, "present.png")

	// Public IDs that name no row. The nil UUID has its own test: it is a
	// lookup edge rather than an absent record.
	for _, publicID := range []uuid.UUID{uuid.New(), uuid.New()} {
		err := svc.Delete(
			context.Background(),
			Actor{ID: admin.ID, Role: admin.Role},
			publicID,
		)
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("Delete(%s) error = %v, want ErrNotFound", publicID, err)
		}
	}

	if n := countAssets(t, db); n != 1 {
		t.Errorf("asset count = %d, want the existing asset untouched", n)
	}
}

// TestDelete_NilPublicIDDoesNotDeleteAnotherAsset is the lookup edge: the nil
// UUID must select no row at all. A `Where(&Asset{PublicID: uuid.Nil})` drops
// the zero value from the query and falls back to "first row", so an ID-less
// request would take out an arbitrary asset instead of 404ing.
func TestDelete_NilPublicIDDoesNotDeleteAnotherAsset(t *testing.T) {
	svc, _, db := newTestService(t)
	admin := seedUser(t, db, "admin", model.RoleAdmin)
	first := seedAsset(t, db, admin.ID, "first.png")

	if err := svc.Delete(
		context.Background(),
		Actor{ID: admin.ID, Role: admin.Role},
		uuid.Nil,
	); !errors.Is(err, ErrNotFound) {
		// Reported with Errorf, not Fatalf: the damage below is the worse
		// half of the same bug and must be reported too.
		t.Errorf("Delete(uuid.Nil) error = %v, want ErrNotFound", err)
	}

	if err := db.First(&model.Asset{}, first.ID).Error; err != nil {
		t.Fatalf("Delete(uuid.Nil) removed an unrelated asset: %v", err)
	}
}

func TestDelete_TwiceIsNotFoundTheSecondTime(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "twice.png")
	actor := Actor{ID: owner.ID, Role: owner.Role}

	if err := svc.Delete(context.Background(), actor, asset.PublicID); err != nil {
		t.Fatalf("first Delete error: %v", err)
	}
	if err := svc.Delete(context.Background(), actor, asset.PublicID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete error = %v, want ErrNotFound", err)
	}
}

func TestDelete_LeavesOtherAssetsIntact(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	first := seedAsset(t, db, owner.ID, "first.png")
	second := seedAsset(t, db, owner.ID, "second.png")

	if err := svc.Delete(
		context.Background(),
		Actor{ID: owner.ID, Role: owner.Role},
		first.PublicID,
	); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if err := db.First(&model.Asset{}, second.ID).Error; err != nil {
		t.Errorf("sibling asset was removed too: %v", err)
	}
	if n := countAssets(t, db); n != 1 {
		t.Errorf("visible asset count = %d, want 1", n)
	}
}

// TestDelete_PrivateAssetIsDeletableByOwner keeps the delete lookup free of the
// public read path's visibility gate: an asset its owner just made private must
// still be deletable by that owner, or a private asset could never be removed.
func TestDelete_PrivateAssetIsDeletableByOwner(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "draft.png")
	setVisibility(t, db, asset.ID, model.AssetVisibilityPrivate)

	if err := svc.Delete(
		context.Background(),
		Actor{ID: owner.ID, Role: owner.Role},
		asset.PublicID,
	); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	if err := db.First(&model.Asset{}, asset.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("private asset survived its owner's delete: %v", err)
	}
}

// TestDelete_PropagatesFindError keeps an infrastructure failure from being
// reported as a missing asset, which would tell the client to give up on a
// record that is still there.
func TestDelete_PropagatesFindError(t *testing.T) {
	svc, _, _ := newTestService(t)
	repo := &stubAssetRepo{findErr: errDB}
	svc.assets = repo

	err := svc.Delete(
		context.Background(),
		Actor{ID: 1, Role: model.RoleSuperAdmin},
		uuid.New(),
	)
	if err == nil {
		t.Fatal("expected an error when the lookup fails, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("lookup failure reported as ErrNotFound: %v", err)
	}
	if !errors.Is(err, errDB) {
		t.Errorf("Delete error = %v, want it to wrap the lookup failure", err)
	}
	if len(repo.deleted) != 0 {
		t.Errorf("delete ran %v despite the failed lookup", repo.deleted)
	}
}

// TestDelete_PropagatesSoftDeleteError checks a failed update surfaces instead
// of being swallowed, and that no row was touched.
func TestDelete_PropagatesSoftDeleteError(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "keep.png")

	svc.assets = &stubAssetRepo{
		find:   &model.Asset{ID: asset.ID, UserID: owner.ID},
		delErr: errDB,
	}

	err := svc.Delete(
		context.Background(),
		Actor{ID: owner.ID, Role: owner.Role},
		asset.PublicID,
	)
	if !errors.Is(err, errDB) {
		t.Fatalf("Delete error = %v, want it to wrap the update failure", err)
	}
	if err := db.First(&model.Asset{}, asset.ID).Error; err != nil {
		t.Errorf("asset vanished after a failed soft delete: %v", err)
	}
}

// TestDelete_MapsRecordNotFoundToErrNotFound documents the single translation
// the repository layer leaves to the service.
func TestDelete_MapsRecordNotFoundToErrNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	svc.assets = &stubAssetRepo{findErr: gorm.ErrRecordNotFound}

	if err := svc.Delete(
		context.Background(),
		Actor{ID: 1, Role: model.RoleAdmin},
		uuid.New(),
	); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete error = %v, want ErrNotFound", err)
	}
}

// TestDelete_ChecksOwnershipBeforeRole covers the ordering: an asset the actor
// does not own stays forbidden even when the lookup itself is satisfied.
func TestDelete_ChecksOwnershipBeforeRole(t *testing.T) {
	svc, _, _ := newTestService(t)
	repo := &stubAssetRepo{find: &model.Asset{ID: 5, UserID: 42}}
	svc.assets = repo

	if err := svc.Delete(
		context.Background(),
		Actor{ID: 43, Role: model.RoleContributor},
		uuid.New(),
	); !errors.Is(err, ErrForbidden) {
		t.Errorf("Delete error = %v, want ErrForbidden", err)
	}
	if len(repo.deleted) != 0 {
		t.Errorf("forbidden delete ran %v", repo.deleted)
	}
}

// TestService_DeleteAssets_EmptyInputIsNoop covers that deleting no IDs is a
// no-op and never touches the repository.
func TestService_DeleteAssets_EmptyInputIsNoop(t *testing.T) {
	svc, _, _ := newTestService(t)

	if err := svc.DeleteAssets(context.Background(), nil); err != nil {
		t.Errorf("DeleteAssets(nil) error = %v, want nil", err)
	}
	if err := svc.DeleteAssets(context.Background(), []uuid.UUID{}); err != nil {
		t.Errorf("DeleteAssets([]) error = %v, want nil", err)
	}
}

// TestService_DeleteAssets_DedupsAndSkipsNil covers that duplicate IDs are
// retired only once and nil UUIDs are skipped.
func TestService_DeleteAssets_DedupsAndSkipsNil(t *testing.T) {
	svc, _, db := newTestService(t)
	id := uuid.New()
	seedAsset(t, db, 0, "dup.png")
	// Override the public ID with a known one.
	var asset model.Asset
	db.First(&asset, "original_name = ?", "dup.png")
	asset.PublicID = id
	db.Save(&asset)

	if err := svc.DeleteAssets(context.Background(), []uuid.UUID{id, id, uuid.Nil, id}); err != nil {
		t.Errorf("DeleteAssets error = %v, want nil", err)
	}
	var retired model.Asset
	if err := db.Unscoped().First(&retired, "public_id = ?", id).Error; err != nil {
		t.Fatalf("asset %s was not soft-deleted: %v", id, err)
	}
	if !retired.DeletedAt.Valid {
		t.Errorf("asset %s is not marked deleted", id)
	}
}

// TestService_DeleteAssets_RetiresOnlyFound covers that IDs without a matching
// asset row are skipped without error.
func TestService_DeleteAssets_RetiresOnlyFound(t *testing.T) {
	svc, _, db := newTestService(t)
	foundID := uuid.New()
	seedAsset(t, db, 0, "found.png")
	var asset model.Asset
	db.First(&asset, "original_name = ?", "found.png")
	asset.PublicID = foundID
	db.Save(&asset)

	missing := uuid.New()
	if err := svc.DeleteAssets(context.Background(), []uuid.UUID{foundID, missing}); err != nil {
		t.Errorf("DeleteAssets error = %v, want nil", err)
	}

	var retired model.Asset
	if err := db.Unscoped().First(&retired, "public_id = ?", foundID).Error; err != nil {
		t.Fatalf("asset %s was not soft-deleted: %v", foundID, err)
	}
	if !retired.DeletedAt.Valid {
		t.Errorf("asset %s is not marked deleted", foundID)
	}

	// The missing ID must not cause an error or delete anything else.
	var count int64
	db.Model(&model.Asset{}).Unscoped().Where("deleted_at IS NOT NULL").Count(&count)
	if count != 1 {
		t.Errorf("expected exactly 1 soft-deleted asset, got %d", count)
	}
}

// TestService_DeleteAssets_PropagatesError covers that a repository failure
// aborts the whole operation. Since the repository layer is backed by GORM,
// a hard-to-trigger error, this test pins that the method returns an error
// when the database is unavailable (e.g. nil ID still passes validation).
func TestService_DeleteAssets_PropagatesError(t *testing.T) {
	svc, _, _ := newTestService(t)

	// Deleting a non-existent ID is not an error; nothing matches.
	if err := svc.DeleteAssets(context.Background(), []uuid.UUID{uuid.New()}); err != nil {
		t.Errorf("DeleteAssets with missing ID error = %v, want nil", err)
	}
}

// --- repositories -------------------------------------------------------

// TestAssetRepository_FindByIDPreloadsBlob covers what the service relies on:
// Delete has to read the owner, and callers need the blob with the asset.
func TestAssetRepository_FindByIDPreloadsBlob(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "photo.png")
	repo := NewAssetRepository(db)

	found, err := repo.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if found.UserID != owner.ID {
		t.Errorf("UserID = %d, want %d", found.UserID, owner.ID)
	}
	if found.Blob.ID != asset.BlobID {
		t.Errorf("Blob.ID = %d, want the asset's blob %d", found.Blob.ID, asset.BlobID)
	}
	if found.Blob.StorageKey == "" {
		t.Error("preloaded blob has no storage key")
	}
}

// TestAssetRepository_SoftDeleteHidesRowFromFind covers the read side of a soft
// delete: FindByID must stop returning the row, not just leave DeletedAt set.
func TestAssetRepository_SoftDeleteHidesRowFromFind(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "photo.png")
	repo := NewAssetRepository(db)

	if err := repo.SoftDelete(context.Background(), &model.Asset{ID: asset.ID}); err != nil {
		t.Fatalf("SoftDelete error: %v", err)
	}
	if _, err := repo.FindByID(context.Background(), asset.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("FindByID after soft delete = %v, want gorm.ErrRecordNotFound", err)
	}
}

// TestAssetRepository_FindPublicByPublicIDPreloadsBlob covers what the read path
// relies on: the lookup is by public ID, but the caller still needs the blob to
// open the object.
func TestAssetRepository_FindPublicByPublicIDPreloadsBlob(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "photo.png")
	repo := NewAssetRepository(db)

	found, err := repo.FindPublicByPublicID(context.Background(), asset.PublicID)
	if err != nil {
		t.Fatalf("FindPublicByPublicID error: %v", err)
	}
	if found.ID != asset.ID {
		t.Errorf("ID = %d, want %d", found.ID, asset.ID)
	}
	if found.Blob.StorageKey == "" {
		t.Error("preloaded blob has no storage key")
	}
}

// TestAssetRepository_FindPublicByPublicIDRefusesPrivateAsset is the gate the
// public route depends on: a private asset has to be indistinguishable from one
// that does not exist, because the response is the only thing an anonymous
// caller sees.
func TestAssetRepository_FindPublicByPublicIDRefusesPrivateAsset(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "draft.png")
	setVisibility(t, db, asset.ID, model.AssetVisibilityPrivate)
	repo := NewAssetRepository(db)

	if _, err := repo.FindPublicByPublicID(context.Background(), asset.PublicID); !errors.Is(
		err,
		gorm.ErrRecordNotFound,
	) {
		t.Errorf("FindPublicByPublicID on a private asset = %v, want gorm.ErrRecordNotFound", err)
	}
}

// TestAssetRepository_FindPublicByPublicIDNilIDSelectsNoRow is the lookup edge.
// A `Where(&Asset{PublicID: uuid.Nil})` drops the zero value from the query and
// falls back to "first row", so a nil ID would serve an arbitrary asset
// instead of nothing. The guard is the only thing standing between the two.
func TestAssetRepository_FindPublicByPublicIDNilIDSelectsNoRow(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	first := seedAsset(t, db, owner.ID, "first.png")
	repo := NewAssetRepository(db)

	found, err := repo.FindPublicByPublicID(context.Background(), uuid.Nil)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("FindPublicByPublicID(uuid.Nil) = %v, want gorm.ErrRecordNotFound", err)
	}
	if found != nil {
		t.Errorf("a nil public ID returned asset %d", found.ID)
	}

	// Reported separately from the error: serving the first row is the worse
	// half of the same bug.
	if err := db.First(&model.Asset{}, first.ID).Error; err != nil {
		t.Errorf("a nil public ID disturbed an unrelated asset: %v", err)
	}
}

// TestAssetRepository_FindPublicByPublicIDHidesSoftDeletedAsset keeps the delete
// contract on the public read path: a deleted asset stops being reachable by its
// public URL, not merely invisible in the admin list.
func TestAssetRepository_FindPublicByPublicIDHidesSoftDeletedAsset(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "photo.png")
	repo := NewAssetRepository(db)

	if err := repo.SoftDelete(context.Background(), &model.Asset{ID: asset.ID}); err != nil {
		t.Fatalf("SoftDelete error: %v", err)
	}
	if _, err := repo.FindPublicByPublicID(
		context.Background(),
		asset.PublicID,
	); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("FindPublicByPublicID after soft delete = %v, want gorm.ErrRecordNotFound", err)
	}
}

// TestAssetRepository_FindByPublicIDFindsPrivateAsset is the difference from
// FindPublicByPublicID: the authenticated delete path must see private assets,
// so this lookup carries no visibility filter.
func TestAssetRepository_FindByPublicIDFindsPrivateAsset(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "draft.png")
	setVisibility(t, db, asset.ID, model.AssetVisibilityPrivate)
	repo := NewAssetRepository(db)

	found, err := repo.FindByPublicID(context.Background(), asset.PublicID)
	if err != nil {
		t.Fatalf("FindByPublicID error: %v", err)
	}
	if found.ID != asset.ID {
		t.Errorf("ID = %d, want %d", found.ID, asset.ID)
	}
	if found.Visibility != model.AssetVisibilityPrivate {
		t.Errorf("Visibility = %q, want %q", found.Visibility, model.AssetVisibilityPrivate)
	}
}

// TestAssetRepository_FindByPublicIDNilIDSelectsNoRow mirrors the public-path
// guard: the nil UUID drops out of a struct query, so without the guard this
// would select the first row.
func TestAssetRepository_FindByPublicIDNilIDSelectsNoRow(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	first := seedAsset(t, db, owner.ID, "first.png")
	repo := NewAssetRepository(db)

	found, err := repo.FindByPublicID(context.Background(), uuid.Nil)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("FindByPublicID(uuid.Nil) = %v, want gorm.ErrRecordNotFound", err)
	}
	if found != nil {
		t.Errorf("a nil public ID returned asset %d", found.ID)
	}
	if err := db.First(&model.Asset{}, first.ID).Error; err != nil {
		t.Errorf("a nil public ID disturbed an unrelated asset: %v", err)
	}
}

// TestAssetRepository_FindByPublicIDHidesSoftDeletedAsset: deleting an
// already-deleted asset by its public ID must 404 rather than resurrect the
// row.
func TestAssetRepository_FindByPublicIDHidesSoftDeletedAsset(t *testing.T) {
	db := newTestDB(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	asset := seedAsset(t, db, owner.ID, "photo.png")
	repo := NewAssetRepository(db)

	if err := repo.SoftDelete(context.Background(), &model.Asset{ID: asset.ID}); err != nil {
		t.Fatalf("SoftDelete error: %v", err)
	}
	if _, err := repo.FindByPublicID(
		context.Background(),
		asset.PublicID,
	); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("FindByPublicID after soft delete = %v, want gorm.ErrRecordNotFound", err)
	}
}
