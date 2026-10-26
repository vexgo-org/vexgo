package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/asset"
	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// seedSiteIconAsset creates the blob and asset rows a real upload would leave
// behind. visibility and blobType let tests cover the values the icon lookup
// must reject.
func seedSiteIconAsset(t *testing.T, db *gorm.DB, mimeType string, blobType model.BlobType, visibility model.AssetVisibility) model.Asset {
	t.Helper()

	blob := model.Blob{
		StorageKey: uuid.NewString(),
		MimeType:   mimeType,
		Type:       blobType,
		Size:       1,
	}
	if err := db.Create(&blob).Error; err != nil {
		t.Fatalf("seed blob: %v", err)
	}

	icon := model.Asset{
		OriginalName: "icon",
		UserID:       1,
		BlobID:       blob.ID,
		PublicID:     uuid.New(),
		Visibility:   visibility,
	}
	if err := db.Create(&icon).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return icon
}

func TestUpdateGeneralSettings_SiteIconRoundTrip(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// Seed the existing row first so the icon update exercises the save path,
	// not only the create path.
	if _, err := svc.UpdateGeneralSettings(ctx, GeneralSettingsRequest{
		SiteName:     "My Blog",
		ItemsPerPage: 12,
	}); err != nil {
		t.Fatalf("create general settings: %v", err)
	}

	icon := seedSiteIconAsset(t, db, "image/png", model.BlobTypeImage, model.AssetVisibilityPublic)
	config, err := svc.UpdateGeneralSettings(ctx, GeneralSettingsRequest{
		SiteName:         "My Blog",
		ItemsPerPage:     12,
		SiteIconPublicID: &icon.PublicID,
	})
	if err != nil {
		t.Fatalf("UpdateGeneralSettings error: %v", err)
	}
	if config.SiteIconID == nil || *config.SiteIconID != icon.ID {
		t.Fatalf("SiteIconID = %v, want %d", config.SiteIconID, icon.ID)
	}
	// The returned config is re-read after saving, so the preloaded relation
	// must reflect the icon too. This guards the stale PUT response.
	if config.SiteIcon == nil || config.SiteIcon.PublicID != icon.PublicID {
		t.Fatalf("SiteIcon = %+v, want public ID %s", config.SiteIcon, icon.PublicID)
	}

	// Clearing the icon removes both the ID and the preloaded relation.
	config, err = svc.UpdateGeneralSettings(ctx, GeneralSettingsRequest{
		SiteName:     "My Blog",
		ItemsPerPage: 12,
	})
	if err != nil {
		t.Fatalf("clear site icon: %v", err)
	}
	if config.SiteIconID != nil || config.SiteIcon != nil {
		t.Fatalf("cleared icon still present: ID=%v icon=%+v", config.SiteIconID, config.SiteIcon)
	}

	var stored model.GeneralSettings
	if err := db.Preload("SiteIcon").First(&stored).Error; err != nil {
		t.Fatalf("load stored settings: %v", err)
	}
	if stored.SiteIconID != nil || stored.SiteIcon != nil {
		t.Fatalf("stored cleared icon still present: ID=%v icon=%+v", stored.SiteIconID, stored.SiteIcon)
	}
}

func TestUpdateGeneralSettings_SiteIconRejectsUnknownPublicID(t *testing.T) {
	svc, _ := newTestService(t)
	missing := uuid.New()

	_, err := svc.UpdateGeneralSettings(context.Background(), GeneralSettingsRequest{
		SiteName:         "My Blog",
		SiteIconPublicID: &missing,
	})
	if !errors.Is(err, ErrInvalidSiteIcon) {
		t.Fatalf("error = %v, want ErrInvalidSiteIcon", err)
	}
}

func TestUpdateGeneralSettings_SiteIconRejectsUnusableAsset(t *testing.T) {
	tests := []struct {
		name       string
		mimeType   string
		blobType   model.BlobType
		visibility model.AssetVisibility
	}{
		{
			name:       "non-image",
			mimeType:   "audio/mpeg",
			blobType:   model.BlobTypeAudio,
			visibility: model.AssetVisibilityPublic,
		},
		{
			name:       "private image",
			mimeType:   "image/png",
			blobType:   model.BlobTypeImage,
			visibility: model.AssetVisibilityPrivate,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := newTestService(t)
			icon := seedSiteIconAsset(t, db, tc.mimeType, tc.blobType, tc.visibility)

			_, err := svc.UpdateGeneralSettings(context.Background(), GeneralSettingsRequest{
				SiteName:         "My Blog",
				SiteIconPublicID: &icon.PublicID,
			})
			if !errors.Is(err, ErrInvalidSiteIcon) {
				t.Fatalf("error = %v, want ErrInvalidSiteIcon", err)
			}
		})
	}
}

func doGeneralSettingsRequest(t *testing.T, r *gin.Engine, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/config/general", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGeneralSettingsRoute_SiteIconRoundTrip(t *testing.T) {
	r, db, _ := newTestAdminRouter(t)
	token := mintAdminToken(t)
	icon := seedSiteIconAsset(t, db, "image/png", model.BlobTypeImage, model.AssetVisibilityPublic)

	body := fmt.Sprintf(
		`{"siteName":"Icon Site","itemsPerPage":12,"siteIconPublicID":%q}`,
		icon.PublicID,
	)
	w := doGeneralSettingsRequest(t, r, token, body)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /config/general status = %d, body: %s", w.Code, w.Body.String())
	}

	var updated GeneralSettingsUpdateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if updated.GeneralSettingsResponse == nil {
		t.Fatal("PUT response is missing generalSettings")
	}
	if updated.GeneralSettingsResponse.SiteIconPublicID == nil ||
		*updated.GeneralSettingsResponse.SiteIconPublicID != icon.PublicID {
		t.Errorf("PUT siteIconPublicID = %v, want %s", updated.GeneralSettingsResponse.SiteIconPublicID, icon.PublicID)
	}
	if got, want := updated.GeneralSettingsResponse.SiteIconURL, asset.OriginalURL(icon.PublicID); got != want {
		t.Errorf("PUT siteIconURL = %q, want %q", got, want)
	}

	// GET must expose the same icon after the response has been persisted.
	req := httptest.NewRequest(http.MethodGet, "/api/config/general", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /config/general status = %d, body: %s", w.Code, w.Body.String())
	}
	var got GeneralSettingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if got.SiteIconPublicID == nil || *got.SiteIconPublicID != icon.PublicID {
		t.Errorf("GET siteIconPublicID = %v, want %s", got.SiteIconPublicID, icon.PublicID)
	}
	if want := asset.OriginalURL(icon.PublicID); got.SiteIconURL != want {
		t.Errorf("GET siteIconURL = %q, want %q", got.SiteIconURL, want)
	}

	// An update without siteIconPublicID clears the icon, and the response must
	// not keep advertising the old URL.
	w = doGeneralSettingsRequest(t, r, token, `{"siteName":"Icon Site","itemsPerPage":12}`)
	if w.Code != http.StatusOK {
		t.Fatalf("clear PUT /config/general status = %d, body: %s", w.Code, w.Body.String())
	}
	var cleared GeneralSettingsUpdateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("decode clear PUT response: %v", err)
	}
	if cleared.GeneralSettingsResponse == nil {
		t.Fatal("clear PUT response is missing generalSettings")
	}
	if cleared.GeneralSettingsResponse.SiteIconPublicID != nil || cleared.GeneralSettingsResponse.SiteIconURL != "" {
		t.Errorf(
			"cleared site icon still advertised: publicID=%v URL=%q",
			cleared.GeneralSettingsResponse.SiteIconPublicID,
			cleared.GeneralSettingsResponse.SiteIconURL,
		)
	}
}

func TestGeneralSettingsRoute_SiteIconInvalidReturnsBadRequest(t *testing.T) {
	r, db, _ := newTestAdminRouter(t)
	token := mintAdminToken(t)
	audio := seedSiteIconAsset(t, db, "audio/mpeg", model.BlobTypeAudio, model.AssetVisibilityPublic)
	missing := uuid.New()

	tests := []struct {
		name string
		id   uuid.UUID
	}{
		{name: "unknown public ID", id: missing},
		{name: "non-image asset", id: audio.PublicID},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"siteName":"Icon Site","siteIconPublicID":%q}`, tc.id)
			w := doGeneralSettingsRequest(t, r, token, body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("PUT /config/general status = %d, want 400, body: %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "Invalid site icon") {
				t.Errorf("PUT response body = %s, want Invalid site icon", w.Body.String())
			}
		})
	}
}
