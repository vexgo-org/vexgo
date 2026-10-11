package asset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Asset{}, &model.User{}); err != nil {
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

// storedFiles lists the names the service actually left in storage.
func storedFiles(t *testing.T, dataDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataDir, "media"))
	if os.IsNotExist(err) {
		// Nothing was ever written, so storage never created the directory.
		return nil
	}
	if err != nil {
		t.Fatalf("read media dir: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestUpload_StoresFileAndRecord(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	// Storage verifies the declared size against the bytes it received, so the
	// size has to be the payload's real length rather than an arbitrary number.
	payload := "jpeg-data"
	asset, err := svc.Upload(
		context.Background(),
		user.ID,
		strings.NewReader(payload),
		"photo.jpg",
		int64(len(payload)),
	)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}

	// Key is a fresh UUID plus the allowlisted extension, never the client's
	// filename, which would otherwise be a path and XSS vector.
	if !strings.HasSuffix(asset.StorageKey, ".jpg") {
		t.Errorf("storage key %q must keep the .jpg extension", asset.StorageKey)
	}
	if _, err := uuid.Parse(strings.TrimSuffix(asset.StorageKey, ".jpg")); err != nil {
		t.Errorf("storage key %q must be UUID-based, got %v", asset.StorageKey, err)
	}
	if asset.OriginalName != "photo.jpg" {
		t.Errorf("original name = %q, want photo.jpg", asset.OriginalName)
	}
	if asset.URL != "/uploads/"+asset.StorageKey {
		t.Errorf("URL = %q, want /uploads/%s", asset.URL, asset.StorageKey)
	}
	if asset.MimeType != "image/jpeg" || asset.Type != model.AssetTypeImage {
		t.Errorf("unexpected type: mime=%q asset=%q", asset.MimeType, asset.Type)
	}
	if asset.Size != int64(len(payload)) || asset.UserID != user.ID {
		t.Errorf("unexpected asset record: %+v", asset)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "media", asset.StorageKey))
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(content) != payload {
		t.Errorf("unexpected file content %q", string(content))
	}

	var stored model.Asset
	if err := db.First(&stored, asset.ID).Error; err != nil {
		t.Fatalf("asset record not saved: %v", err)
	}
}

// TestUpload_RejectsExtensionOutsideAllowlist covers the stored-XSS guard: a
// stored file is served from this origin, so an extension the browser would
// render as an executable document (html, js, xhtml, xml) or that is not a
// media type at all must never reach storage, not even as an extensionless
// object. Case is normalized, because camera uploads arrive as "IMG_1.JPG".
func TestUpload_RejectsExtensionOutsideAllowlist(t *testing.T) {
	for _, name := range []string{
		"evil.html", "evil.htm", "evil.xhtml", "evil.xml", "evil.js",
		"payload.svgz", "shell.exe", "archive.tar.gz", "notes",
		"x.", "x.<script>", `x." onload="alert(1)`,
	} {
		t.Run(name, func(t *testing.T) {
			svc, dataDir, db := newTestService(t)
			user := seedUser(t, db, "uploader", model.RoleContributor)

			_, err := svc.Upload(context.Background(), user.ID, strings.NewReader("x"), name, 1)
			if !errors.Is(err, ErrInvalidExtension) {
				t.Fatalf("Upload(%q) error = %v, want ErrInvalidExtension", name, err)
			}

			var count int64
			if err := db.Model(&model.Asset{}).Count(&count).Error; err != nil {
				t.Fatalf("count error: %v", err)
			}
			if count != 0 {
				t.Errorf("rejected upload created %d asset record(s)", count)
			}
			if names := storedFiles(t, dataDir); len(names) != 0 {
				t.Errorf("rejected upload left %v in storage", names)
			}
		})
	}
}

func TestUpload_NormalizesExtensionCase(t *testing.T) {
	svc, _, _ := newTestService(t)

	asset, err := svc.Upload(context.Background(), 1, strings.NewReader("x"), "IMG_0042.JPG", 1)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}
	if !strings.HasSuffix(asset.StorageKey, ".jpg") {
		t.Errorf("storage key = %q, want a .jpg suffix", asset.StorageKey)
	}
}

// failingCreateRepo forces CreateAsset to fail so the storage rollback path can
// be exercised: a row that fails to persist must not leave an orphan object
// behind, which would be invisible in the DB and unlistable in storage.
type failingCreateRepo struct{ Repository }

func (failingCreateRepo) CreateAsset(context.Context, *model.Asset) error {
	return errors.New("unique constraint failed: assets.storage_key")
}

func TestUpload_RollsBackStorageWhenRecordFails(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	user := seedUser(t, db, "uploader", model.RoleContributor)

	svc.repo = failingCreateRepo{Repository: svc.repo}

	_, err := svc.Upload(context.Background(), user.ID, strings.NewReader("x"), "photo.png", 1)
	if err == nil {
		t.Fatal("expected an error when the record cannot be persisted, got nil")
	}
	if names := storedFiles(t, dataDir); len(names) != 0 {
		t.Errorf("failed upload left %v in storage, want the object rolled back", names)
	}
	if err := db.Model(&model.Asset{}).Count(new(int64)).Error; err != nil {
		t.Fatalf("count error: %v", err)
	}
}

// TestDelete_Permissions pins the ownership rule: the uploader and admins may
// delete, nobody else. Deleting is a soft delete, so the object stays in
// storage and the row stays reachable unscoped.
func TestDelete_Permissions(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	stranger := seedUser(t, db, "stranger", model.RoleContributor)
	admin := seedUser(t, db, "admin", model.RoleAdmin)

	asset, err := svc.Upload(context.Background(), owner.ID, strings.NewReader("x"), "mine.jpg", 1)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}

	if err := svc.Delete(context.Background(), stranger.ID, asset.StorageKey); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger Delete = %v, want ErrForbidden", err)
	}

	if err := svc.Delete(context.Background(), admin.ID, asset.StorageKey); err != nil {
		t.Errorf("admin Delete of another user's asset = %v, want nil", err)
	}

	// A soft-deleted asset is gone for normal queries but still in the table.
	var visible int64
	if err := db.Model(&model.Asset{}).Count(&visible).Error; err != nil {
		t.Fatalf("count error: %v", err)
	}
	if visible != 0 {
		t.Errorf("soft-deleted asset still visible to normal queries: %d row(s)", visible)
	}
	var softDeleted model.Asset
	if err := db.Unscoped().First(&softDeleted, asset.ID).Error; err != nil {
		t.Errorf("expected a soft-deleted row to remain: %v", err)
	}
	if names := storedFiles(t, dataDir); len(names) != 1 {
		t.Errorf("soft delete must keep the object in storage, got %v", names)
	}

	if err := svc.Delete(context.Background(), owner.ID, asset.StorageKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(context.Background(), owner.ID, "no-such-key.jpg"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown key = %v, want ErrNotFound", err)
	}
	// An empty key is a caller bug, not a missing asset: it must not be
	// reported as ErrNotFound, or the handler would answer 404 for it.
	if err := svc.Delete(context.Background(), owner.ID, ""); errors.Is(err, ErrNotFound) {
		t.Error("empty key reported as ErrNotFound, want an internal error")
	}
}

// TestFindByID covers the ID-addressed lookup: it resolves an asset the caller
// already knows the ID of, and reports a miss as the domain's ErrNotFound so
// callers can map it to 404 without leaking the storage layer's sentinel.
func TestFindByID(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)

	asset, err := svc.Upload(context.Background(), owner.ID, strings.NewReader("x"), "mine.jpg", 1)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}

	found, err := svc.FindByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("FindByID error: %v", err)
	}
	if found.ID != asset.ID || found.StorageKey != asset.StorageKey || found.UserID != owner.ID {
		t.Errorf("FindByID returned %+v, want asset %d", found, asset.ID)
	}

	if _, err := svc.FindByID(context.Background(), asset.ID+999); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID(unknown id) = %v, want ErrNotFound", err)
	}
}

// TestDeleteByID_Permissions pins the same ownership rule as
// TestDelete_Permissions, for the ID-addressed route: the uploader and admins
// may delete, nobody else, and the delete stays soft.
func TestDeleteByID_Permissions(t *testing.T) {
	svc, dataDir, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)
	stranger := seedUser(t, db, "stranger", model.RoleContributor)
	admin := seedUser(t, db, "admin", model.RoleAdmin)

	asset, err := svc.Upload(context.Background(), owner.ID, strings.NewReader("x"), "mine.jpg", 1)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}

	if err := svc.DeleteByID(context.Background(), stranger.ID, asset.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("stranger DeleteByID = %v, want ErrForbidden", err)
	}

	if err := svc.DeleteByID(context.Background(), admin.ID, asset.ID); err != nil {
		t.Errorf("admin DeleteByID of another user's asset = %v, want nil", err)
	}

	var visible int64
	if err := db.Model(&model.Asset{}).Count(&visible).Error; err != nil {
		t.Fatalf("count error: %v", err)
	}
	if visible != 0 {
		t.Errorf("soft-deleted asset still visible to normal queries: %d row(s)", visible)
	}
	var softDeleted model.Asset
	if err := db.Unscoped().First(&softDeleted, asset.ID).Error; err != nil {
		t.Errorf("expected a soft-deleted row to remain: %v", err)
	}
	if names := storedFiles(t, dataDir); len(names) != 1 {
		t.Errorf("soft delete must keep the object in storage, got %v", names)
	}

	// Soft-deleted rows are scoped out, so a repeat delete looks like a miss.
	if err := svc.DeleteByID(context.Background(), owner.ID, asset.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second DeleteByID = %v, want ErrNotFound", err)
	}
	if err := svc.DeleteByID(context.Background(), owner.ID, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id = %v, want ErrNotFound", err)
	}
}

// failingUserLookupRepo forces FindUserByID to fail with an unexpected error
// to exercise the fail-closed authorization path in Delete.
type failingUserLookupRepo struct{ Repository }

func (failingUserLookupRepo) FindUserByID(context.Context, uint) (*model.User, error) {
	return nil, errors.New("database is unavailable")
}

// TestDelete_UserLookupFailureFailsClosed ensures a transient failure while
// looking up the acting user can never bypass the ownership check: the delete
// must fail, not proceed.
func TestDelete_UserLookupFailureFailsClosed(t *testing.T) {
	svc, _, db := newTestService(t)
	owner := seedUser(t, db, "owner", model.RoleContributor)

	asset, err := svc.Upload(context.Background(), owner.ID, strings.NewReader("x"), "keep.jpg", 1)
	if err != nil {
		t.Fatalf("Upload error: %v", err)
	}

	svc.repo = failingUserLookupRepo{Repository: svc.repo}

	if err := svc.Delete(context.Background(), owner.ID, asset.StorageKey); err == nil || errors.Is(err, ErrForbidden) {
		t.Errorf("expected a non-forbidden error (fail closed), got %v", err)
	}

	var count int64
	if err := db.Model(&model.Asset{}).Count(&count).Error; err != nil {
		t.Fatalf("count error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected asset preserved after failed delete, got %d", count)
	}
}
