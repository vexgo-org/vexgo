package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var errAssetConfigDB = errors.New("database is unavailable")

// fakeAssetPruner records reschedule calls and can fail on demand.
type fakeAssetPruner struct {
	err   error
	calls int
}

func (f *fakeAssetPruner) ReschedulePruneJob(context.Context) error {
	f.calls++
	return f.err
}

// stubAssetConfigRepo embeds Repository so only the asset-config methods the
// service calls need an implementation. A nil config means "no row stored".
type stubAssetConfigRepo struct {
	Repository

	config    *model.AssetConfig
	getErr    error
	createErr error
	saveErr   error

	created *model.AssetConfig
	saved   *model.AssetConfig
}

func (r *stubAssetConfigRepo) GetAssetConfig(context.Context) (*model.AssetConfig, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.config == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.config, nil
}

func (r *stubAssetConfigRepo) CreateAssetConfig(_ context.Context, config *model.AssetConfig) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = config
	return nil
}

func (r *stubAssetConfigRepo) SaveAssetConfig(_ context.Context, config *model.AssetConfig) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = config
	return nil
}

func newAssetConfigService(t *testing.T) (*Service, *gorm.DB, *fakeAssetPruner) {
	t.Helper()
	db := newTestDB(t)
	pruner := &fakeAssetPruner{}
	return NewService(Deps{DB: db, Assets: pruner}), db, pruner
}

// --- UpdateAssetConfig --------------------------------------------------

func TestUpdateAssetConfig_SavesAndReschedules(t *testing.T) {
	svc, db, pruner := newAssetConfigService(t)
	if err := db.Create(&model.AssetConfig{
		PruneInterval:      "@monthly",
		PruneRetentionDays: 30,
	}).Error; err != nil {
		t.Fatalf("seed asset config: %v", err)
	}

	got, err := svc.UpdateAssetConfig(context.Background(), AssetConfigRequest{
		PruneInterval:      "@every 6h",
		PruneRetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("UpdateAssetConfig error: %v", err)
	}
	if got.PruneInterval != "@every 6h" || got.PruneRetentionDays != 7 {
		t.Errorf("returned config = %+v, want the request values", got)
	}

	var stored model.AssetConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load stored config: %v", err)
	}
	if stored.PruneInterval != "@every 6h" || stored.PruneRetentionDays != 7 {
		t.Errorf("stored config = %+v, want the request values", stored)
	}
	if pruner.calls != 1 {
		t.Errorf("reschedule calls = %d, want 1", pruner.calls)
	}
}

func TestUpdateAssetConfig_CreatesMissingRowAndReschedules(t *testing.T) {
	svc, db, pruner := newAssetConfigService(t)

	if _, err := svc.UpdateAssetConfig(context.Background(), AssetConfigRequest{
		PruneInterval:      "@daily",
		PruneRetentionDays: 14,
	}); err != nil {
		t.Fatalf("UpdateAssetConfig error: %v", err)
	}

	var count int64
	db.Model(&model.AssetConfig{}).Count(&count)
	if count != 1 {
		t.Errorf("stored config rows = %d, want 1", count)
	}
	if pruner.calls != 1 {
		t.Errorf("reschedule calls = %d, want 1", pruner.calls)
	}
}

func TestUpdateAssetConfig_RejectsInvalidCron(t *testing.T) {
	for _, spec := range []string{"", "not-a-cron", "* * *"} {
		t.Run("spec="+spec, func(t *testing.T) {
			repo := &stubAssetConfigRepo{}
			pruner := &fakeAssetPruner{}
			svc := NewService(Deps{Assets: pruner})
			svc.repo = repo

			_, err := svc.UpdateAssetConfig(context.Background(), AssetConfigRequest{
				PruneInterval:      spec,
				PruneRetentionDays: 30,
			})
			if !errors.Is(err, ErrInvalidCron) {
				t.Fatalf("error = %v, want ErrInvalidCron", err)
			}
			if repo.created != nil || repo.saved != nil {
				t.Error("invalid cron must not write the configuration")
			}
			if pruner.calls != 0 {
				t.Errorf("reschedule calls = %d, want 0", pruner.calls)
			}
		})
	}
}

func TestUpdateAssetConfig_PropagatesRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo *stubAssetConfigRepo
	}{
		{"get", &stubAssetConfigRepo{getErr: errAssetConfigDB}},
		{"create", &stubAssetConfigRepo{createErr: errAssetConfigDB}},
		{
			"save",
			&stubAssetConfigRepo{
				config:  &model.AssetConfig{PruneInterval: "@monthly", PruneRetentionDays: 30},
				saveErr: errAssetConfigDB,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pruner := &fakeAssetPruner{}
			svc := NewService(Deps{Assets: pruner})
			svc.repo = tc.repo

			_, err := svc.UpdateAssetConfig(context.Background(), AssetConfigRequest{
				PruneInterval:      "@daily",
				PruneRetentionDays: 30,
			})
			if !errors.Is(err, errAssetConfigDB) {
				t.Fatalf("error = %v, want it to wrap %v", err, errAssetConfigDB)
			}
			if pruner.calls != 0 {
				t.Errorf("reschedule calls = %d, want 0 after a failed write", pruner.calls)
			}
		})
	}
}

func TestUpdateAssetConfig_PropagatesRescheduleError(t *testing.T) {
	repo := &stubAssetConfigRepo{
		config: &model.AssetConfig{PruneInterval: "@monthly", PruneRetentionDays: 30},
	}
	pruner := &fakeAssetPruner{err: errAssetConfigDB}
	svc := NewService(Deps{Assets: pruner})
	svc.repo = repo

	_, err := svc.UpdateAssetConfig(context.Background(), AssetConfigRequest{
		PruneInterval:      "@daily",
		PruneRetentionDays: 30,
	})
	if !errors.Is(err, errAssetConfigDB) {
		t.Fatalf("error = %v, want it to wrap %v", err, errAssetConfigDB)
	}
	if repo.saved == nil {
		t.Error("the configuration must be saved before the reschedule is attempted")
	}
}

// --- GetAssetConfig -----------------------------------------------------

func TestGetAssetConfig_ReturnsStored(t *testing.T) {
	svc, db, _ := newAssetConfigService(t)
	if err := db.Create(&model.AssetConfig{
		PruneInterval:      "@every 6h",
		PruneRetentionDays: 7,
	}).Error; err != nil {
		t.Fatalf("seed asset config: %v", err)
	}

	got, err := svc.GetAssetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetAssetConfig error: %v", err)
	}
	if got.PruneInterval != "@every 6h" || got.PruneRetentionDays != 7 {
		t.Errorf("config = %+v, want the stored values", got)
	}
}

func TestGetAssetConfig_DefaultsWhenMissing(t *testing.T) {
	svc, _, _ := newAssetConfigService(t)

	got, err := svc.GetAssetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetAssetConfig error: %v", err)
	}
	if got.PruneInterval != "@monthly" || got.PruneRetentionDays != 30 {
		t.Errorf("config = %+v, want the first-run defaults", got)
	}
}

func TestGetAssetConfig_PropagatesError(t *testing.T) {
	svc := NewService(Deps{})
	svc.repo = &stubAssetConfigRepo{getErr: errAssetConfigDB}

	_, err := svc.GetAssetConfig(context.Background())
	if !errors.Is(err, errAssetConfigDB) {
		t.Fatalf("error = %v, want it to wrap %v", err, errAssetConfigDB)
	}
}

// --- routes -------------------------------------------------------------

func doAssetConfigRequest(
	t *testing.T,
	r *gin.Engine,
	method, token, body string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/config/asset", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAssetConfigRoute_GetDefaults(t *testing.T) {
	r, _, _ := newTestAdminRouter(t)

	w := doAssetConfigRequest(t, r, http.MethodGet, mintAdminToken(t), "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s), want 200", w.Code, w.Body.String())
	}

	// Decode into a map: the wire must use the lower-camel keys the generated
	// client expects, not the Go field names.
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["pruneInterval"] != "@monthly" || body["pruneRetentionDays"] != float64(30) {
		t.Errorf("body = %v, want the first-run defaults", body)
	}
	if _, ok := body["PruneInterval"]; ok {
		t.Errorf("response uses Go field names: %s", w.Body.String())
	}
}

func TestAssetConfigRoute_UpdatePersistsAndReturns(t *testing.T) {
	r, db, _ := newTestAdminRouter(t)

	w := doAssetConfigRequest(
		t,
		r,
		http.MethodPut,
		mintAdminToken(t),
		`{"pruneInterval":"@every 6h","pruneRetentionDays":7}`,
	)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (body %s), want 200", w.Code, w.Body.String())
	}

	var body AssetConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PruneInterval != "@every 6h" || body.PruneRetentionDays != 7 {
		t.Errorf("body = %+v, want the request values", body)
	}

	var stored model.AssetConfig
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load stored config: %v", err)
	}
	if stored.PruneInterval != "@every 6h" || stored.PruneRetentionDays != 7 {
		t.Errorf("stored config = %+v, want the request values", stored)
	}
}

func TestAssetConfigRoute_UpdateRejectsInvalidCron(t *testing.T) {
	r, db, _ := newTestAdminRouter(t)

	w := doAssetConfigRequest(
		t,
		r,
		http.MethodPut,
		mintAdminToken(t),
		`{"pruneInterval":"not-a-cron","pruneRetentionDays":30}`,
	)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (body %s), want 400", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Invalid cron expression") {
		t.Errorf("body = %s, want the invalid-cron error", w.Body.String())
	}

	var count int64
	db.Model(&model.AssetConfig{}).Count(&count)
	if count != 0 {
		t.Errorf("stored config rows = %d, want 0 after a rejected update", count)
	}
}

func TestAssetConfigRoute_UpdateRejectsInvalidRetention(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing", `{"pruneInterval":"@daily"}`},
		{"zero", `{"pruneInterval":"@daily","pruneRetentionDays":0}`},
		{"negative", `{"pruneInterval":"@daily","pruneRetentionDays":-1}`},
		{"over max", `{"pruneInterval":"@daily","pruneRetentionDays":366}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, _ := newTestAdminRouter(t)

			w := doAssetConfigRequest(t, r, http.MethodPut, mintAdminToken(t), tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d (body %s), want 400", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "Invalid request payload") {
				t.Errorf("body = %s, want the payload error", w.Body.String())
			}

			var count int64
			db.Model(&model.AssetConfig{}).Count(&count)
			if count != 0 {
				t.Errorf("stored config rows = %d, want 0 after a rejected update", count)
			}
		})
	}
}

// A malformed body must produce exactly one response; a missing early return
// would append the invalid-cron response to the payload error.
func TestAssetConfigRoute_UpdateRejectsInvalidJSON(t *testing.T) {
	r, _, _ := newTestAdminRouter(t)

	w := doAssetConfigRequest(t, r, http.MethodPut, mintAdminToken(t), `{`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (body %s), want 400", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Invalid request payload") {
		t.Errorf("body = %s, want the payload error", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Invalid cron") {
		t.Errorf("handler wrote a second response: %s", w.Body.String())
	}
}

func TestAssetConfigRoute_RequiresAdmin(t *testing.T) {
	r, db, _ := newTestAdminRouter(t)

	guest := model.User{
		Username:        "guest",
		Email:           "guest@example.com",
		Role:            model.RoleGuest,
		PasswordVersion: 1,
	}
	if err := db.Create(&guest).Error; err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, tc := range []struct {
			name  string
			token string
			want  int
		}{
			{"anonymous", "", http.StatusUnauthorized},
			{"guest", mintTokenFor(t, guest.ID, model.RoleGuest), http.StatusForbidden},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				w := doAssetConfigRequest(t, r, method, tc.token, `{"pruneInterval":"@daily"}`)
				if w.Code != tc.want {
					t.Errorf("status = %d (body %s), want %d", w.Code, w.Body.String(), tc.want)
				}
			})
		}
	}
}
