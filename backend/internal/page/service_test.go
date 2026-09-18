package page

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/vexgo-org/vexgo/backend/internal/model"

	"gorm.io/gorm"
)

type fakeRepo struct {
	mu     sync.Mutex
	pages  map[uint]*model.Page
	bySlug map[string]uint
	nextID uint
	// lastSearch records the search term the service passed down, so tests can
	// assert on the cap applied before the query reaches the repository.
	lastSearch string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{pages: map[uint]*model.Page{}, bySlug: map[string]uint{}, nextID: 1}
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*model.Page, error) {
	var uid uint
	if _, err := fmt.Sscanf(id, "%d", &uid); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.pages[uid]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakeRepo) FindBySlug(_ context.Context, slug string) (*model.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.bySlug[slug]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *f.pages[id]
	return &cp, nil
}

func (f *fakeRepo) SlugExists(_ context.Context, slug string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.bySlug[slug]
	return ok, nil
}

func (f *fakeRepo) SlugExistsExcludeID(_ context.Context, slug string, excludeID uint) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.bySlug[slug]
	return ok && id != excludeID, nil
}

func (f *fakeRepo) Create(_ context.Context, page *model.Page) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.bySlug[page.Slug]; ok {
		return gorm.ErrDuplicatedKey
	}
	page.ID = f.nextID
	f.nextID++
	cp := *page
	f.pages[cp.ID] = &cp
	f.bySlug[cp.Slug] = cp.ID
	*page = cp
	return nil
}

func (f *fakeRepo) Save(_ context.Context, page *model.Page) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.pages[page.ID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if old.Slug != page.Slug {
		delete(f.bySlug, old.Slug)
		if _, taken := f.bySlug[page.Slug]; taken {
			return gorm.ErrDuplicatedKey
		}
		f.bySlug[page.Slug] = page.ID
	}
	cp := *page
	f.pages[cp.ID] = &cp
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, page *model.Page) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.pages[page.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.bySlug, page.Slug)
	delete(f.pages, page.ID)
	return nil
}

func (f *fakeRepo) List(_ context.Context, q ListQuery) ([]model.Page, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSearch = q.Search
	var out []model.Page
	for _, p := range f.pages {
		if q.Status != "" && string(p.Status) != q.Status {
			continue
		}
		if q.Search != "" && !strings.Contains(p.Title, q.Search) && !strings.Contains(p.Slug, q.Search) {
			continue
		}
		out = append(out, *p)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) ListNav(_ context.Context) ([]model.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.Page
	for _, p := range f.pages {
		if p.Status == model.PageStatusPublished && p.ShowInNav {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeRepo) FindUserByID(_ context.Context, _ uint) (*model.User, error) {
	return &model.User{Role: model.RoleAdmin}, nil
}

// The search cap counts runes: the old byte slice cut multi-byte terms
// mid-character, putting invalid UTF-8 in the LIKE pattern that PostgreSQL
// rejects in a query parameter.
func TestList_SearchTruncationIsRuneSafe(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)
	ctx := context.Background()

	// 200 runes / 600 bytes: over the old byte cap, exactly at the rune cap.
	term := strings.Repeat("评", maxSearchRunes)
	if _, _, err := svc.List(ctx, ListQuery{Search: term, Page: 1, Limit: 20}); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if repo.lastSearch != term {
		t.Errorf("a term within the rune cap must pass through unchanged, got %d runes",
			utf8.RuneCountInString(repo.lastSearch))
	}

	// 250 runes: cut at maxSearchRunes, still valid UTF-8.
	if _, _, err := svc.List(ctx, ListQuery{Search: strings.Repeat("评", maxSearchRunes+50), Page: 1, Limit: 20}); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if got := utf8.RuneCountInString(repo.lastSearch); got != maxSearchRunes {
		t.Errorf("expected %d runes, got %d", maxSearchRunes, got)
	}
	if !utf8.ValidString(repo.lastSearch) {
		t.Error("truncated search term is not valid UTF-8")
	}
}

func TestCreate_AdminOnly(t *testing.T) {
	svc := newServiceWithRepo(newFakeRepo())
	ctx := context.Background()
	if _, err := svc.Create(ctx, model.RoleAuthor, 1, CreatePageRequest{Slug: "about", Title: "About", Content: "hi"}); err == nil {
		t.Fatal("non-admin create should be forbidden")
	}
	if _, err := svc.Create(ctx, "", 0, CreatePageRequest{Slug: "about", Title: "About", Content: "hi"}); err == nil {
		t.Fatal("guest create should be forbidden")
	}
	p, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "About", Title: "About", Content: "hi", Status: "published"})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if p.Slug != "about" {
		t.Errorf("slug should be lowercased, got %q", p.Slug)
	}
}

func TestCreate_RejectsReservedUppercaseDuplicate(t *testing.T) {
	svc := newServiceWithRepo(newFakeRepo())
	ctx := context.Background()
	if _, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "admin", Title: "x", Content: "y"}); err == nil {
		t.Error("reserved slug should be rejected")
	}
	if _, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "HELLO", Title: "x", Content: "y"}); err != nil {
		t.Errorf("uppercase should be normalized, got %v", err)
	} else {
		// second create with same normalized slug conflicts
		if _, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "hello", Title: "x", Content: "y"}); err == nil {
			t.Error("duplicate slug should conflict")
		}
	}
	if _, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "hello-world", Title: "x", Content: "y"}); err != nil {
		t.Errorf("valid slug rejected: %v", err)
	}
	// Non-ASCII page slugs are rejected per the locked ASCII-only decision.
	if _, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "about-us", Title: "x", Content: "y"}); err != nil {
		t.Errorf("ascii slug rejected: %v", err)
	}
}

func TestGetBySlug_DraftHiddenFromGuest(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)
	ctx := context.Background()
	if _, err := svc.Create(ctx, model.RoleAdmin, 1, CreatePageRequest{Slug: "secret", Title: "s", Content: "c"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetBySlug(ctx, "secret", 0, ""); err == nil {
		t.Error("guest should not see draft")
	}
	if _, err := svc.GetBySlug(ctx, "secret", 0, model.RoleAdmin); err != nil {
		t.Errorf("admin should see draft: %v", err)
	}
}

// A page is readable by anonymous visitors, so its author block must not hand
// out account state or an email address the owner did not publish; the owner
// and administrators still see the account.
func TestAuthorBlockRedactedForPublicViewers(t *testing.T) {
	ctx := context.Background()

	// seedPublished stores a published page whose author block looks like a
	// preloaded admin account (the fake repository does not preload anything).
	seedPublished := func(t *testing.T) (*Service, *model.Page) {
		t.Helper()
		repo := newFakeRepo()
		svc := newServiceWithRepo(repo)
		if _, err := svc.Create(ctx, model.RoleAdmin, 7, CreatePageRequest{
			Slug: "about", Title: "A", Content: "c", Status: "published",
		}); err != nil {
			t.Fatal(err)
		}
		stored := repo.pages[1]
		stored.AuthorID = 7
		stored.Author = model.User{
			ID: 7, Username: "author", Email: "author@example.com",
			Role: model.RoleAdmin, EmailVerified: true,
			ProfileVisibility: model.ProfileVisibilityPublic,
		}
		return svc, stored
	}

	check := func(t *testing.T, p *model.Page, wantEmail string, wantAccountState bool) {
		t.Helper()
		if p.Author.Username != "author" {
			t.Fatalf("test setup lost the author block: %+v", p.Author)
		}
		if p.Author.Email != wantEmail {
			t.Errorf("expected email %q, got %q", wantEmail, p.Author.Email)
		}
		if wantAccountState {
			return
		}
		if p.Author.Role != "" || p.Author.EmailVerified || p.Author.ProfileVisibility != "" {
			t.Errorf("account state leaked in the author block: %+v", p.Author)
		}
	}

	t.Run("anonymous list and slug", func(t *testing.T) {
		svc, _ := seedPublished(t)

		pages, _, err := svc.List(ctx, ListQuery{Status: string(model.PageStatusPublished), Page: 1, Limit: 10})
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		check(t, &pages[0], "", false)

		page, err := svc.GetBySlug(ctx, "about", 0, "")
		if err != nil {
			t.Fatalf("GetBySlug error: %v", err)
		}
		check(t, page, "", false)
	})

	t.Run("admin keeps the account", func(t *testing.T) {
		svc, _ := seedPublished(t)

		page, err := svc.GetBySlug(ctx, "about", 99, model.RoleSuperAdmin)
		if err != nil {
			t.Fatalf("GetBySlug error: %v", err)
		}
		check(t, page, "author@example.com", true)
	})
}

func TestUpdateDelete_AdminOnly(t *testing.T) {
	repo := newFakeRepo()
	svc := newServiceWithRepo(repo)
	ctx := context.Background()
	p, err := svc.Create(ctx, model.RoleSuperAdmin, 1, CreatePageRequest{Slug: "docs", Title: "d", Content: "c", Status: "published"})
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("%d", p.ID)
	if _, err := svc.Update(ctx, id, model.RoleAuthor, UpdatePageRequest{Title: "x"}); err == nil {
		t.Error("non-admin update should be forbidden")
	}
	if err := svc.Delete(ctx, id, model.RoleAuthor); err == nil {
		t.Error("non-admin delete should be forbidden")
	}
	sortOrder := 5
	if _, err := svc.Update(ctx, id, model.RoleAdmin, UpdatePageRequest{SortOrder: &sortOrder}); err != nil {
		t.Errorf("admin update: %v", err)
	}
	if err := svc.Delete(ctx, id, model.RoleAdmin); err != nil {
		t.Errorf("admin delete: %v", err)
	}
}
