package public

import (
	"html/template"
	"time"

	"github.com/vexgo-org/vexgo/backend/internal/model"
)

// ThemeInfo represents metadata for a theme. Preview is the cover image and
// must be an http(s) URL when set; local paths are rejected at upload time.
type ThemeInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Preview     string `json:"preview,omitempty"`
}

// SeedPage is one parsed theme seed file.
type SeedPage struct {
	Slug      string
	Title     string
	Content   string
	ShowInNav bool
	SortOrder int
	Status    model.PageStatus
}

// Dict is one language's flat key -> text lookup for a theme.
type Dict map[string]string

// SiteData is the site-wide context available to every theme template.
type SiteData struct {
	Name         string
	Description  string
	Icon         string
	URL          string
	ItemsPerPage int
	// Language is the resolved visitor language for this request
	// (e.g. "en", "zh"), after the ?lang=/cookie/Accept-Language/site
	// default chain. Templates emit it as <html lang> and the t function
	// uses it implicitly.
	Language string
	// DefaultLanguage is the configured site default from settings.
	DefaultLanguage string
}

// NavPageData is one custom page shown in the theme navigation.
type NavPageData struct {
	Title     string
	Slug      string
	URL       string
	SortOrder int
}

// PostCardData is the public summary of a post for list pages (home, user).
type PostCardData struct {
	ID            uint
	Title         string
	Slug          string
	Excerpt       string
	CoverImage    string
	Category      string
	Tags          []string
	AuthorName    string
	AuthorID      uint
	AuthorAvatar  string
	CreatedAt     time.Time
	ViewCount     int
	CommentsCount int64
	LikesCount    int64
	URL           string
}

// PageLinkData is one numbered pagination entry. Ellipsis entries carry no
// number or URL; they render as a literal "..." separator.
type PageLinkData struct {
	Number    int
	URL       string
	IsCurrent bool
	Ellipsis  bool
}

// PaginationData drives the navigation of list pages: prev/next links plus
// the windowed list of numbered page links (first, last, current ± 1).
type PaginationData struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevURL     string
	NextURL     string
	Pages       []PageLinkData
}

// IndexQueryData carries the active filters of the home page.
type IndexQueryData struct {
	Search   string
	Category string
}

// PopularTagData is one entry of the hot-tags sidebar: a tag name and how
// many published posts carry it.
type PopularTagData struct {
	Name  string
	Count int64
}

// PopularTag is one row of the popular-tags query: a tag name and how many
// published posts carry it.
type PopularTag struct {
	Name  string
	Count int64
}

// IndexData is the context of the home page template.
type IndexData struct {
	Site         *SiteData
	Posts        []PostCardData
	Pages        []NavPageData
	Pagination   *PaginationData
	Query        IndexQueryData
	Categories   []string
	PopularPosts []PostCardData
	PopularTags  []PopularTagData
}

// PostData is the context of the post detail template.
type PostData struct {
	Site  *SiteData
	Pages []NavPageData
	Post  struct {
		ID            uint
		Title         string
		Slug          string
		Excerpt       string
		CoverImage    string
		Category      string
		Tags          []string
		AuthorName    string
		AuthorID      uint
		AuthorAvatar  string
		CreatedAt     time.Time
		UpdatedAt     time.Time
		ViewCount     int
		CommentsCount int64
		LikesCount    int64
		ContentHTML   template.HTML
		URL           string
	}
}

// UserData is the context of the user profile template.
type UserData struct {
	Site  *SiteData
	Pages []NavPageData
	User  struct {
		ID         uint
		Username   string
		Avatar     string
		Bio        string
		CreatedAt  time.Time
		PostsCount int64
	}
	Posts      []PostCardData
	Pagination *PaginationData
}

// NotFoundData is the context of the optional 404 template.
type NotFoundData struct {
	Site  *SiteData
	Pages []NavPageData
}

// PageData is the context of custom page templates (page.html and any
// <slug>.html dedicated template).
type PageData struct {
	Site  *SiteData
	Pages []NavPageData
	Page  struct {
		ID          uint
		Title       string
		Slug        string
		CreatedAt   time.Time
		UpdatedAt   time.Time
		ContentHTML template.HTML
		URL         string
	}
}

// PostListQuery selects one page of published posts for a public listing page.
// Base is the URL prefix pagination links are built from; Search and Category
// filter like the public API does; AuthorID narrows to one author (0 = any).
type PostListQuery struct {
	Base     string
	Page     int
	Limit    int
	Search   string
	Category string
	AuthorID uint
}
