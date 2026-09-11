package public

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/vexgo-org/vexgo/backend/internal/model"
	"github.com/vexgo-org/vexgo/backend/internal/page"
)

// seedDir is the theme subdirectory holding default page seeds:
// seed/<slug>.md files with YAML frontmatter (title, showInNav, sortOrder,
// status) and a Markdown body. Themes own their seed data; the backend only
// creates rows for slugs that do not exist yet and never overwrites them.
const seedDir = "seed"

// seedSlugRe mirrors the locked page-slug rule (Q18): lowercase ASCII,
// digits and hyphens. Files whose names do not qualify are skipped.
var seedSlugRe = regexp.MustCompile(`^[a-z0-9-]{1,100}$`)

// SeedPage is one parsed theme seed file.
type SeedPage struct {
	Slug      string
	Title     string
	Content   string
	ShowInNav bool
	SortOrder int
	Status    model.PageStatus
}

// parseSeedFile parses a seed/<slug>.md file. The optional frontmatter block
// (--- ... ---) carries title (default: slug), showInNav, sortOrder and
// status (default: published); everything after it is the Markdown body.
func parseSeedFile(slug string, src []byte) SeedPage {
	seed := SeedPage{Slug: slug, Title: slug, Status: model.PageStatusPublished}
	text := string(src)
	rest := text
	if strings.HasPrefix(text, "---") {
		if end := strings.Index(text[3:], "\n---"); end >= 0 {
			for line := range strings.Lines(text[3 : 3+end]) {
				key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
				if !ok {
					continue
				}
				value = strings.TrimSpace(value)
				switch strings.ToLower(strings.TrimSpace(key)) {
				case "title":
					if value != "" {
						seed.Title = value
					}
				case "showinnav":
					seed.ShowInNav = value == "true" || value == "1"
				case "sortorder":
					if n, err := strconv.Atoi(value); err == nil {
						seed.SortOrder = n
					}
				case "status":
					if value == string(model.PageStatusDraft) {
						seed.Status = model.PageStatusDraft
					}
				}
			}
			rest = text[3+end+len("\n---"):]
		}
	}
	seed.Content = strings.TrimSpace(rest)
	return seed
}

// LoadThemeSeeds reads a theme's seed pages in filename order. A missing
// seed directory means the theme ships no seeds (nil, nil). Unreadable files
// and invalid slugs are skipped with a warning so one bad file cannot block
// the rest.
func (r *Renderer) LoadThemeSeeds(themeID string) ([]SeedPage, error) {
	base, err := r.themeFS(themeID)
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(base, seedDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		if !seedSlugRe.MatchString(slug) {
			slog.Warn("skipping theme seed with invalid slug", "theme", themeID, "file", e.Name())
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	seeds := make([]SeedPage, 0, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(base, seedDir+"/"+name)
		if err != nil {
			slog.Warn("skipping unreadable theme seed", "theme", themeID, "file", name, "err", err)
			continue
		}
		seeds = append(seeds, parseSeedFile(strings.TrimSuffix(name, ".md"), content))
	}
	return seeds, nil
}

// ActiveThemeID returns the currently active theme id.
func (r *Renderer) ActiveThemeID() string {
	return r.activeTheme()
}

// EnsureThemeSeeds creates a theme's seed pages for slugs that do not exist
// yet and returns how many rows were created. Existing rows (including user
// edits) are never touched; reserved or invalid slugs are skipped with a
// warning. It is safe to run on every startup and on every theme activation.
func (r *Renderer) EnsureThemeSeeds(ctx context.Context, themeID string) (int, error) {
	seeds, err := r.LoadThemeSeeds(themeID)
	if err != nil {
		return 0, err
	}
	if len(seeds) == 0 {
		return 0, nil
	}
	var author model.User
	if err := r.db.WithContext(ctx).
		Where("role = ?", model.RoleSuperAdmin).
		Order("id ASC").
		First(&author).Error; err != nil {
		if err := r.db.WithContext(ctx).
			Where("role = ?", model.RoleAdmin).
			Order("id ASC").
			First(&author).Error; err != nil {
			slog.Warn("skipping theme seeds without an admin author", "theme", themeID)
			return 0, nil
		}
	}
	svc := page.NewService(page.Deps{DB: r.db, JWTSecret: r.jwtSecret})
	created := 0
	for _, seed := range seeds {
		if _, err := svc.GetBySlug(ctx, seed.Slug, model.RoleSuperAdmin); err == nil {
			continue
		} else if !errors.Is(err, page.ErrPageNotFound) {
			return created, err
		}
		_, err := svc.Create(ctx, model.RoleSuperAdmin, author.ID, page.CreateRequest{
			Slug: seed.Slug, Title: seed.Title, Content: seed.Content,
			ShowInNav: seed.ShowInNav, SortOrder: seed.SortOrder, Status: seed.Status,
		})
		if err != nil {
			slog.Warn("skipping theme seed page", "theme", themeID, "slug", seed.Slug, "err", err)
			continue
		}
		created++
	}
	return created, nil
}
