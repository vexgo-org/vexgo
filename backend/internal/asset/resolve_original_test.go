package asset

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/storage"

	"gorm.io/gorm"
)

// Tests for Service.ResolveOriginal, the read path behind GET /assets/:id/
// original. It is the one asset path that serves bytes to an unauthenticated
// caller, so the assertions are about what it hands out (content and blob
// metadata) and, mostly, about when it refuses: a deleted row, a missing
// object and a broken row must all fail closed.

// errStore stands in for any storage backend failure that is not a missing key.
var errStore = errors.New("object store is unreachable")

// seedStoredAsset seeds an asset whose blob metadata matches the object actually
// written to storage, so a test can assert the metadata ResolveOriginal returns
// instead of the placeholders seedAsset installs.
func seedStoredAsset(
	t *testing.T,
	db *gorm.DB,
	dataDir string,
	ownerID uint,
	name string,
	mimeType string,
	content []byte,
) model.Asset {
	t.Helper()
	asset := seedAsset(t, db, ownerID, name)
	if err := db.Model(&model.Blob{}).
		Where("id = ?", asset.BlobID).
		Updates(map[string]any{"mime_type": mimeType, "size": len(content)}).
		Error; err != nil {
		t.Fatalf("failed to align blob %d metadata: %v", asset.BlobID, err)
	}
	writeObject(t, db, dataDir, asset.BlobID, string(content))
	return asset
}

// stubOpenStorage answers every Open with a fixed error and records the keys it
// was asked for, so a test can prove a lookup never reached the backend at all.
type stubOpenStorage struct {
	storage.Storage
	err   error
	opens []string
}

func (s *stubOpenStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	s.opens = append(s.opens, key)
	return nil, s.err
}

func TestResolveOriginal_ReturnsStoredContentAndBlobMetadata(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	content := pngPayload(64)
	asset := seedStoredAsset(t, db, dataDir, owner.ID, "holiday photo.png", "image/png", content)

	got, err := svc.ResolveOriginal(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("ResolveOriginal(%d) = %v, want the stored object", asset.ID, err)
	}
	t.Cleanup(func() { _ = got.Reader.Close() })

	if got.MIMEType != "image/png" {
		t.Errorf("MIMEType = %q, want %q", got.MIMEType, "image/png")
	}
	if got.Filename != "holiday photo.png" {
		t.Errorf("Filename = %q, want %q", got.Filename, "holiday photo.png")
	}
	if got.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", got.Size, len(content))
	}

	body, err := io.ReadAll(got.Reader)
	if err != nil {
		t.Fatalf("failed to read resolved content: %v", err)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("content = %d bytes, want the %d stored bytes", len(body), len(content))
	}
}

// TestResolveOriginal_ServesEachAssetsOwnObject guards the one mixup this read
// path can make: two live assets, each resolving to its own bytes.
func TestResolveOriginal_ServesEachAssetsOwnObject(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	first := pngPayload(32)
	second := pngPayload(48)
	firstAsset := seedStoredAsset(t, db, dataDir, owner.ID, "first.png", "image/png", first)
	secondAsset := seedStoredAsset(t, db, dataDir, owner.ID, "second.png", "image/png", second)

	for _, tc := range []struct {
		asset model.Asset
		want  []byte
	}{
		{firstAsset, first},
		{secondAsset, second},
	} {
		got, err := svc.ResolveOriginal(context.Background(), tc.asset.ID)
		if err != nil {
			t.Fatalf("ResolveOriginal(%d) = %v", tc.asset.ID, err)
		}
		body, err := io.ReadAll(got.Reader)
		_ = got.Reader.Close()
		if err != nil {
			t.Fatalf("failed to read asset %d: %v", tc.asset.ID, err)
		}
		if !bytes.Equal(body, tc.want) {
			t.Errorf("asset %d served %d bytes, want %d", tc.asset.ID, len(body), len(tc.want))
		}
	}
}

// TestResolveOriginal_ZeroIDIsNotFound pins that id 0 is refused before the
// repository is consulted at all. The repository rejects zero on its own, so
// the handler-facing contract is only enforced here if a repository that answers
// zero with a row still cannot make the service serve one.
func TestResolveOriginal_ZeroIDIsNotFound(t *testing.T) {
	store := &stubOpenStorage{
		Storage: storage.NewLocalStorage(t.TempDir()),
		err:     storage.ErrNotFound,
	}
	repo := &stubAssetRepo{find: &model.Asset{
		ID:           99,
		OriginalName: "photo.png",
		Blob:         model.Blob{StorageKey: "someone-elses.png"},
	}}
	svc := newFakeService(mixedRepositories{assets: repo}, store, nil, nil)

	content, err := svc.ResolveOriginal(context.Background(), 0)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ResolveOriginal(0) = %v, want ErrNotFound", err)
	}
	if content != nil {
		t.Errorf("ResolveOriginal(0) returned content for a zero id")
	}
	if len(store.opens) != 0 {
		t.Errorf("a zero id reached storage with %v", store.opens)
	}
}

func TestResolveOriginal_MissingAssetIsNotFound(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	seedStoredAsset(t, db, dataDir, owner.ID, "photo.png", "image/png", pngPayload(16))
	seedStoredAsset(t, db, dataDir, owner.ID, "other.png", "image/png", pngPayload(16))

	content, err := svc.ResolveOriginal(context.Background(), 4242)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ResolveOriginal(4242) = %v, want ErrNotFound", err)
	}
	if content != nil {
		t.Errorf("ResolveOriginal(4242) returned content for an unknown id")
	}
}

// TestResolveOriginal_SoftDeletedAssetIsNotFound is the delete contract seen
// from the read side: soft deleting an asset must take its bytes off the public
// route, not merely hide the row from the admin UI.
func TestResolveOriginal_SoftDeletedAssetIsNotFound(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	asset := seedStoredAsset(t, db, dataDir, owner.ID, "photo.png", "image/png", pngPayload(16))

	if err := svc.Delete(
		context.Background(),
		Actor{ID: owner.ID, Role: owner.Role},
		asset.ID,
	); err != nil {
		t.Fatalf("failed to delete asset: %v", err)
	}

	content, err := svc.ResolveOriginal(context.Background(), asset.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ResolveOriginal(%d) after delete = %v, want ErrNotFound", asset.ID, err)
	}
	if content != nil {
		t.Errorf("a deleted asset still served content")
	}
}

// TestResolveOriginal_MissingObjectIsContentUnavailable covers the row/object
// split that ErrContentUnavailable exists for: the records are intact, so this
// is not a client error, but the bytes are gone and must not be reported as a
// plain missing asset.
func TestResolveOriginal_MissingObjectIsContentUnavailable(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	// Seeded without writing the object: the row exists, the file does not.
	asset := seedAsset(t, db, owner.ID, "photo.png")

	content, err := svc.ResolveOriginal(context.Background(), asset.ID)
	if !errors.Is(err, ErrContentUnavailable) {
		t.Errorf("ResolveOriginal(%d) = %v, want ErrContentUnavailable", asset.ID, err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("a missing object must not be reported as a missing asset")
	}
	if content != nil {
		t.Errorf("a missing object returned content")
	}
}

// TestResolveOriginal_BrokenBlobRowNeverServesContent covers an asset whose
// blob row is gone: the preload leaves an empty storage key, which the backend
// rejects. Whatever the exact error is, this must not degrade into content.
func TestResolveOriginal_BrokenBlobRowNeverServesContent(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	asset := seedStoredAsset(t, db, dataDir, owner.ID, "photo.png", "image/png", pngPayload(16))

	if err := db.Delete(&model.Blob{}, asset.BlobID).Error; err != nil {
		t.Fatalf("failed to soft delete blob: %v", err)
	}

	content, err := svc.ResolveOriginal(context.Background(), asset.ID)
	if err == nil {
		t.Fatalf("ResolveOriginal(%d) served an asset whose blob row is gone", asset.ID)
	}
	if content != nil {
		t.Errorf("ResolveOriginal(%d) returned content for a broken blob row", asset.ID)
	}
}

// TestResolveOriginal_PropagatesStorageFailure pins the third outcome: a
// backend that is broken rather than empty stays distinguishable from a missing
// asset, which is what lets the handler answer 503 and 500 differently.
func TestResolveOriginal_PropagatesStorageFailure(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "author", model.RoleAuthor)
	asset := seedStoredAsset(t, db, dataDir, owner.ID, "photo.png", "image/png", pngPayload(16))

	svc.storage = &stubOpenStorage{
		Storage: storage.NewLocalStorage(dataDir),
		err:     errStore,
	}

	content, err := svc.ResolveOriginal(context.Background(), asset.ID)
	if !errors.Is(err, errStore) {
		t.Errorf("ResolveOriginal(%d) = %v, want it to wrap the storage failure", asset.ID, err)
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrContentUnavailable) {
		t.Errorf("a broken backend was reported as %v, want a plain error", err)
	}
	if content != nil {
		t.Errorf("a failed open returned content")
	}
	if opens := svc.storage.(*stubOpenStorage).opens; len(opens) != 1 {
		t.Errorf("storage was asked %d times, want 1", len(opens))
	}
}

// TestResolveOriginal_PropagatesFindFailure keeps a database outage from being
// reported as a 404: the handler would tell a caller its asset does not exist
// when the truth is that nobody could ask.
func TestResolveOriginal_PropagatesFindFailure(t *testing.T) {
	store := &stubOpenStorage{
		Storage: storage.NewLocalStorage(t.TempDir()),
		err:     storage.ErrNotFound,
	}
	svc := newFakeService(
		mixedRepositories{assets: &stubAssetRepo{findErr: errDB}},
		store,
		nil,
		nil,
	)

	content, err := svc.ResolveOriginal(context.Background(), 1)
	if !errors.Is(err, errDB) {
		t.Errorf("ResolveOriginal = %v, want it to wrap the database failure", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Errorf("a database outage was reported as a missing asset")
	}
	if content != nil {
		t.Errorf("a failed lookup returned content")
	}
	if len(store.opens) != 0 {
		t.Errorf("storage was reached without a row: %v", store.opens)
	}
}
