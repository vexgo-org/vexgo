# Theme Templates Reference

> **Reference** — the exact files, fields, functions and limits a VexGo theme can use. Look things up here; for the reasoning behind the system see [Theming](/concepts/theming), and for a walkthrough see the [Theme Development guide](/guides/theme-development).

## Theme directory layout

A theme directory, as extracted from an uploaded ZIP into `data/theme/<id>/`:

| Path               | Required | Purpose                                                                       |
| ------------------ | -------- | ----------------------------------------------------------------------------- |
| `vexgo-theme.json` | Yes      | Theme manifest. Without it the theme is not listed and cannot be activated.   |
| `index.html`       | No\*     | Home page template (`IndexData`).                                             |
| `post.html`        | No\*     | Post detail template (`PostData`).                                            |
| `page.html`        | No\*     | Generic custom-page template (`PageData`).                                    |
| `user.html`        | No\*     | Public profile template (`UserData`).                                         |
| `404.html`         | No       | Not-found template (`NotFoundData`), also the last fallback for custom pages. |
| `<slug>.html`      | No       | Dedicated template for the custom page with that slug. Lowercase, no slash.   |
| `i18n/<lang>.json` | No       | Flat `key: text` dictionaries. Without them `{{t "key"}}` renders the key.    |
| `seed/<slug>.md`   | No       | Default pages created on activation; never overwrite existing pages.          |
| `assets/<file>`    | No       | Static files served at `/theme-assets/<file>`.                                |
| anything else      | No       | Kept on disk but not read by the renderer.                                    |

\* A theme must provide **at least one** of `index.html`, `post.html`, `page.html`, `user.html` or `404.html`, or activation fails with "theme has no template files". Routes whose template is missing simply return a 404.

All templates are parsed into a single template set, so `{{define}}`/`{{template}}` fragments are shared across files. Root-level `.html` files that are not one of the five names above are treated as dedicated custom-page templates.

## `vexgo-theme.json`

| Field         | Type   | Required | Rules                                                                                                                  |
| ------------- | ------ | -------- | ---------------------------------------------------------------------------------------------------------------------- |
| `id`          | string | Yes      | `^[A-Za-z0-9_-]+$`, must equal the install directory, cannot be `default` (reserved).                                  |
| `name`        | string | Yes      | Non-empty display name.                                                                                                |
| `version`     | string | Yes      | Non-empty version string. Stored, not compared.                                                                        |
| `author`      | string | No       | Author name.                                                                                                           |
| `description` | string | No       | One-line description.                                                                                                  |
| `url`         | string | No       | Theme homepage or repository URL.                                                                                      |
| `preview`     | string | No       | Cover image URL. Must be `http://` or `https://` with a host and at most 2048 characters; a relative path is rejected. |

The manifest is decoded with a plain JSON unmarshal, so unknown fields are ignored.

## Pages, templates and contexts

| Request                                     | Template resolution                                           | Context        |
| ------------------------------------------- | ------------------------------------------------------------- | -------------- |
| `/` with `?search=`, `?category=`, `?page=` | `index.html`                                                  | `IndexData`    |
| `/post/<slug>` and `/posts/<slug>`          | `post.html`                                                   | `PostData`     |
| `/<slug>`                                   | `<slug>.html`, else `page.html`, else `404.html` (status 404) | `PageData`     |
| `/user/<id>`                                | `user.html`                                                   | `UserData`     |
| unmatched / missing content                 | `404.html`                                                    | `NotFoundData` |

Only published posts and published pages are rendered; drafts render the 404 template. A parsed-template error at render time produces a plain-text `404` body instead.

## Template contexts

### `SiteData`

Available as `.Site` on every page.

| Field             | Type   | Notes                                                        |
| ----------------- | ------ | ------------------------------------------------------------ |
| `Name`            | string | Site name (defaults to `VexGo`).                             |
| `Description`     | string | Site description.                                            |
| `Icon`            | string | Configured site icon URL, if any.                            |
| `URL`             | string | Configured site base URL.                                    |
| `ItemsPerPage`    | int    | Posts per list page (defaults to 20).                        |
| `Language`        | string | Resolved visitor language for this request, e.g. `en`, `zh`. |
| `DefaultLanguage` | string | Site default language from settings.                         |

### `NavPageData`

The navigation entries, available as `.Pages` on every page. One entry per **published** custom page with `showInNav` enabled, ordered by `SortOrder` and then by page id.

| Field       | Type   | Notes                         |
| ----------- | ------ | ----------------------------- |
| `Title`     | string | Page title.                   |
| `Slug`      | string | Page slug.                    |
| `URL`       | string | Ready-to-use URL (`/<slug>`). |
| `SortOrder` | int    | Ordering hint, lowest first.  |

### `PostCardData`

The summary shape used in post lists (`.Posts` on the home and user pages, and `.PopularPosts`).

| Field           | Type      | Notes                              |
| --------------- | --------- | ---------------------------------- |
| `ID`            | uint      | Post ID.                           |
| `Title`         | string    | Title.                             |
| `Slug`          | string    | Slug.                              |
| `Excerpt`       | string    | Excerpt.                           |
| `CoverImage`    | string    | Cover image URL, may be empty.     |
| `Category`      | string    | Category name, may be empty.       |
| `Tags`          | []string  | Tag names; may be empty.           |
| `AuthorName`    | string    | Empty when the post has no author. |
| `AuthorID`      | uint      | `0` when the post has no author.   |
| `AuthorAvatar`  | string    | Author avatar URL.                 |
| `CreatedAt`     | time.Time | Creation timestamp.                |
| `ViewCount`     | int       | View count.                        |
| `CommentsCount` | int64     | Comment count.                     |
| `LikesCount`    | int64     | Like count.                        |
| `URL`           | string    | Ready-to-use URL (`/post/<slug>`). |

### `IndexData`

Home page context.

| Field          | Type               | Notes                                                                                      |
| -------------- | ------------------ | ------------------------------------------------------------------------------------------ |
| `Site`         | \*`SiteData`       | Site-wide values.                                                                          |
| `Posts`        | []`PostCardData`   | Published posts for the current page.                                                      |
| `Pages`        | []`NavPageData`    | Navigation entries.                                                                        |
| `Pagination`   | \*`PaginationData` | Always set; `Pages` is empty when there is only one page.                                  |
| `Query`        | `IndexQueryData`   | Active `?search=` / `?category=` filters.                                                  |
| `Categories`   | []string           | Distinct non-empty category names among published posts.                                   |
| `PopularPosts` | []`PostCardData`   | Up to 5 posts ranked by likes × 5 + views, drawn from the 200 most recent published posts. |
| `PopularTags`  | []`PopularTagData` | Up to 10 tags, ordered by usage count descending.                                          |

### `PostData`

Post detail context. The post itself is nested under `.Post`.

| Field                | Type            | Notes                                   |
| -------------------- | --------------- | --------------------------------------- |
| `Site`               | \*`SiteData`    | Site-wide values.                       |
| `Pages`              | []`NavPageData` | Navigation entries.                     |
| `Post.ID`            | uint            | Post ID (used by the comment widget).   |
| `Post.Title`         | string          | Title.                                  |
| `Post.Slug`          | string          | Slug.                                   |
| `Post.Excerpt`       | string          | Excerpt.                                |
| `Post.CoverImage`    | string          | Cover image URL.                        |
| `Post.Category`      | string          | Category name.                          |
| `Post.Tags`          | []string        | Tag names.                              |
| `Post.AuthorName`    | string          | Empty when the post has no author.      |
| `Post.AuthorID`      | uint            | `0` when the post has no author.        |
| `Post.AuthorAvatar`  | string          | Author avatar URL.                      |
| `Post.CreatedAt`     | time.Time       | Creation timestamp.                     |
| `Post.UpdatedAt`     | time.Time       | Last update timestamp.                  |
| `Post.ViewCount`     | int             | View count.                             |
| `Post.CommentsCount` | int64           | Comment count.                          |
| `Post.LikesCount`    | int64           | Like count.                             |
| `Post.ContentHTML`   | template.HTML   | Rendered Markdown body; emit unescaped. |
| `Post.URL`           | string          | Ready-to-use URL (`/post/<slug>`).      |

### `UserData`

Public profile context.

| Field             | Type               | Notes                                                     |
| ----------------- | ------------------ | --------------------------------------------------------- |
| `Site`            | \*`SiteData`       | Site-wide values.                                         |
| `Pages`           | []`NavPageData`    | Navigation entries.                                       |
| `User.ID`         | uint               | User ID.                                                  |
| `User.Username`   | string             | Username.                                                 |
| `User.Avatar`     | string             | Avatar URL.                                               |
| `User.Bio`        | string             | Empty when the user hides their bio.                      |
| `User.CreatedAt`  | time.Time          | Registration timestamp.                                   |
| `User.PostsCount` | int64              | Number of published posts.                                |
| `Posts`           | []`PostCardData`   | The user's published posts, paginated.                    |
| `Pagination`      | \*`PaginationData` | Always set; `Pages` is empty when there is only one page. |

### `PageData`

Custom page context (both `page.html` and any `<slug>.html`).

| Field              | Type            | Notes                                   |
| ------------------ | --------------- | --------------------------------------- |
| `Site`             | \*`SiteData`    | Site-wide values.                       |
| `Pages`            | []`NavPageData` | Navigation entries.                     |
| `Page.ID`          | uint            | Page ID.                                |
| `Page.Title`       | string          | Title.                                  |
| `Page.Slug`        | string          | Slug.                                   |
| `Page.CreatedAt`   | time.Time       | Creation timestamp.                     |
| `Page.UpdatedAt`   | time.Time       | Last update timestamp.                  |
| `Page.ContentHTML` | template.HTML   | Rendered Markdown body; emit unescaped. |
| `Page.URL`         | string          | Ready-to-use URL (`/<slug>`).           |

### `NotFoundData`

| Field   | Type            | Notes               |
| ------- | --------------- | ------------------- |
| `Site`  | \*`SiteData`    | Site-wide values.   |
| `Pages` | []`NavPageData` | Navigation entries. |

### `PaginationData`

| Field         | Type             | Notes                                                                             |
| ------------- | ---------------- | --------------------------------------------------------------------------------- |
| `CurrentPage` | int              | 1-based current page.                                                             |
| `TotalPages`  | int              | Total page count.                                                                 |
| `HasPrev`     | bool             | Whether a previous page exists.                                                   |
| `HasNext`     | bool             | Whether a next page exists.                                                       |
| `PrevURL`     | string           | URL of the previous page (empty when `HasPrev` is false).                         |
| `NextURL`     | string           | URL of the next page (empty when `HasNext` is false).                             |
| `Pages`       | []`PageLinkData` | Windowed numbered links: first, last and current ± 1; nil when `TotalPages` is 1. |

The pointer is never nil when a list page renders successfully, and `TotalPages` is at least 1, so a single-page list still exposes `CurrentPage`, `HasPrev` and `HasNext`. Guard pagination markup with `{{if .Pagination.HasNext}}` rather than `{{if .Pagination}}`.

### `PageLinkData`

| Field       | Type   | Notes                                               |
| ----------- | ------ | --------------------------------------------------- |
| `Number`    | int    | Page number.                                        |
| `URL`       | string | URL of the page.                                    |
| `IsCurrent` | bool   | True for the current page.                          |
| `Ellipsis`  | bool   | True for a "…" separator; `Number`/`URL` are unset. |

### `IndexQueryData`

| Field      | Type   | Notes                                                                                            |
| ---------- | ------ | ------------------------------------------------------------------------------------------------ |
| `Search`   | string | Active `?search=` value; the home page matches it against post titles and bodies as a substring. |
| `Category` | string | Active `?category=` value; matched exactly against the post category.                            |

### `PopularTagData`

| Field   | Type   | Notes                              |
| ------- | ------ | ---------------------------------- |
| `Name`  | string | Tag name.                          |
| `Count` | int64  | Number of published posts with it. |

## Template helpers

Every theme template can call these functions:

| Helper        | Signature          | Returns                                                                                                                                 |
| ------------- | ------------------ | --------------------------------------------------------------------------------------------------------------------------------------- |
| `date`        | `date t layout`    | Formats a `time.Time` using a Go layout, e.g. `{{date .Post.CreatedAt "2006-01-02"}}`.                                                  |
| `add`         | `add a b`          | Sum of two ints, e.g. for 1-based positions.                                                                                            |
| `first`       | `first items n`    | At most `n` items; a non-positive `n` returns the slice unchanged.                                                                      |
| `truncate`    | `truncate s max`   | Trims whitespace, then caps at `max` **runes**, appending `...` when it cut; a non-positive `max` returns the trimmed string unchanged. |
| `userURL`     | `userURL id`       | `/user/<id>`.                                                                                                                           |
| `categoryURL` | `categoryURL name` | `/?category=<url-escaped>`.                                                                                                             |
| `searchURL`   | `searchURL name`   | `/?search=<url-escaped>`.                                                                                                               |
| `t`           | `t key`            | The merged dictionary's value for `key`, or `key` itself when missing.                                                                  |

Guidance:

- Use `truncate` for excerpts and titles in list layouts; it counts runes, so CJK text is never cut mid-character.
- `first` and `truncate` treat a limit of `0` as "leave the value alone" rather than "return nothing", so a misconfigured cap cannot blank out content.
- Use `userURL`, `categoryURL` and `searchURL` instead of building URLs inline. They also keep double quotes out of HTML attributes.
- `t` requires the theme to ship `i18n/<lang>.json`; without a dictionary it renders the key.

## `i18n/<lang>.json`

A flat object of string keys to strings; nesting is not supported.

```json
{
  "nav.home": "Home",
  "post.comments": "Comments"
}
```

- The filename is the **normalized** language code (`zh.json`, not `zh-CN.json`).
- Dictionaries are merged per key in the order `en` → site default → visitor language.
- A key missing from every dictionary renders as the key itself.

Language resolution order for a request:

```text
?lang=  →  vexgo_lang cookie  →  Accept-Language  →  site default  →  en
```

A `?lang=` value that normalizes to a valid code is persisted in the `vexgo_lang` cookie for one year.

## Seed frontmatter

`seed/<slug>.md` files use optional `---` frontmatter:

| Key         | Type   | Default     | Notes                   |
| ----------- | ------ | ----------- | ----------------------- |
| `title`     | string | the slug    | Page title.             |
| `showInNav` | bool   | `false`     | Accepts `true` or `1`.  |
| `sortOrder` | int    | `0`         | Navigation ordering.    |
| `status`    | string | `published` | `published` or `draft`. |

Everything after the frontmatter is the Markdown body. Slugs must match `^[a-z0-9-]{1,100}$`; files that do not qualify are skipped with a warning. Seeds are created only for slugs that do not already exist, on activation and on startup, and are authored by the site's first super admin (fallback: first admin).

## Asset URLs

| Disk path             | URL                                                         |
| --------------------- | ----------------------------------------------------------- |
| `assets/style.css`    | `/theme-assets/style.css`                                   |
| `assets/images/a.png` | `/theme-assets/images/a.png`                                |
| `favicon.ico`         | `/favicon.ico` (after the site icon and `data/favicon.ico`) |

`/theme-assets/*` is resolved against the **active** theme, so templates never contain a theme id. The `assets/` segment is added by the server and must not appear in the URL.

## Comment widget contract

Drop-in markup for the post template:

```html
<div
  id="vexgo-comments"
  data-post-id="{{.Post.ID}}"
  data-lang="{{.Site.Language}}"
></div>
<script src="/theme-assets/comments.js" defer></script>
```

| Attribute      | Required | Notes                                                                                       |
| -------------- | -------- | ------------------------------------------------------------------------------------------- |
| `id`           | Yes      | Must be `vexgo-comments`; the widget does nothing if the element is absent.                 |
| `data-post-id` | Yes      | The post ID. Without it the widget exits.                                                   |
| `data-lang`    | No       | Widget UI language; built-in strings cover `en` and `zh`, anything else falls back to `en`. |

The widget is framework-free, styles itself with inline styles, renders all user content with `textContent`, talks to the public comment/like API, and reuses the session stored by the admin SPA. A theme that ships its own `assets/comments.js` replaces the built-in widget; the server never injects one.

## Upload limits

Applied when a theme ZIP is uploaded, before anything is written to disk:

| Limit                | Value   | Failure                          |
| -------------------- | ------- | -------------------------------- |
| Archive size         | 32 MiB  | `400 Theme archive too large`    |
| Total unpacked size  | 100 MiB | `400 Theme archive too large`    |
| Files in the archive | 2000    | `400 Theme archive too large`    |
| Single unpacked file | 10 MiB  | `400 Theme archive too large`    |
| File extension       | `.zip`  | `400 File must be a zip archive` |

Additional validation: the archive must contain a readable `vexgo-theme.json` (at the root or inside a single top-level directory), `id`/`name`/`version` must be present, `id` must match the install directory, and `preview` must be an `http(s)` URL. Activation separately requires every shipped template to parse.

## Related Reading

- [Theming](/concepts/theming) — how the renderer uses these files.
- [Theme Development guide](/guides/theme-development) — building and installing a theme.
- [Architecture](/concepts/architecture) — where the renderer sits in the backend.
