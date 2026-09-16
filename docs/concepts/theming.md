# Theming

> **Explanation** — how VexGo turns a theme directory into server-rendered pages: the request pipeline, how templates are resolved, how the language is chosen, what gets cached, and what happens to a theme's seed pages. Read this to understand _why_ the theme system behaves the way it does. For step-by-step instructions, see the [Theme Development guide](/guides/theme-development); for exact field and function tables, see the [Theme Templates reference](/reference/theme-templates).

## Overview

Public pages in VexGo are rendered on the server. When a visitor opens `/`, `/post/my-post`, `/about` or `/user/2`, the backend executes a Go template from the active theme and returns finished HTML. The visitor's browser does not need JavaScript to read content, search engines see the full markup, and the same HTML is served to every client.

The admin panel is a separate, non-public surface: it lives under `/admin/` and is a JavaScript single-page application. Everything that is not an API route or an asset route is either a public SSR page or a 404.

A **theme** is a directory of HTML templates, an optional `i18n/` dictionary directory, an optional `seed/` directory of default pages, and an `assets/` directory for CSS, JavaScript and images. There is no compilation step and no plugin API: a theme is data the renderer reads.

## Where a theme lives

There are two kinds of theme, and they are treated the same by the renderer:

- **The default theme** is embedded into the backend binary. It is always available, cannot be deleted, and is the fallback whenever the active theme cannot be resolved.
- **Uploaded themes** are extracted from ZIP archives into `data/theme/<id>/` and become available immediately.

The **active theme** is a single value stored in the database. Switching it takes effect on the next request without a restart, and each request resolves the theme once, so a page never mixes two themes.

## One request's journey

The router registers a small set of public routes, and a catch-all handles the rest. Each route resolves a theme, builds the template data, and renders one template:

| Request                                               | Template file                            | Template context |
| ----------------------------------------------------- | ---------------------------------------- | ---------------- |
| `/` (with `?search=`, `?category=`, `?page=` filters) | `index.html`                             | `IndexData`      |
| `/post/<slug>`, `/posts/<slug>`                       | `post.html`                              | `PostData`       |
| `/<slug>`                                             | `<slug>.html` → `page.html` → `404.html` | `PageData`       |
| `/user/<id>`                                          | `user.html`                              | `UserData`       |
| anything unmatched                                    | `404.html` (optional)                    | `NotFoundData`   |

Two deliberate choices are visible in that table:

- **The custom-page chain falls back rather than fails.** A page at `/<slug>` uses a dedicated `<slug>.html` template when the theme ships one, otherwise the generic `page.html`, otherwise the 404 template with a 404 status. That is why the default theme can ship special layouts for its `links` and `timeline` seed pages while still having every other slug render through `page.html`.
- **Content visibility is decided before the template runs.** Draft and pending posts are filtered out in the query, so a guessed slug renders the 404 page instead of leaking unpublished text. The one exception is an explicit admin draft preview, which is documented in the guide.

If a route's template is missing or fails to parse at render time, the response degrades to a plain `404` rather than a stack trace, so a broken theme on a live site produces a missing page, not a server error.

## How templates are resolved

All `.html` files at the root of a theme are parsed into **one** template set. This is what makes Go's `{{define "name"}}` / `{{template "name"}}` mechanism work across files: a theme can keep its header and footer in `partials.html`—any filename works—and include them from every page.

Five names are meaningful to the server, and the rest are optional:

- `index.html`, `post.html`, `page.html`, `user.html` — the four page templates
- `404.html` — the not-found template
- any other root-level `<slug>.html` — a dedicated template for the custom page with that slug. Files with slashes, dot-segments or uppercase names are ignored; only plain lowercase `<slug>.html` files qualify.

A theme must provide at least one of the five named templates to be considered valid, and **activating a theme validates that every template it ships parses**. A theme with a syntax error is rejected at activation time rather than breaking the public site later. Since a page template must be a complete HTML document, each file typically begins with `<!DOCTYPE html>`.

## Where static assets come from

Templates never name a theme. Assets are referenced with a stable prefix:

```html
<link rel="stylesheet" href="/theme-assets/style.css" />
```

The server resolves `/theme-assets/<file>` against the **active** theme's `assets/` directory. Note that the `assets/` segment is added by the server: the file on disk is `assets/style.css`, but the URL is `/theme-assets/style.css`. A template can therefore be copied between themes unmodified, and two themes can both have a `style.css` without colliding.

Because assets are same-origin with the public site, the theme's JavaScript runs with the site's privileges. This is why uploading a theme is an admin-only operation — see [Trust boundaries](#trust-boundaries).

## How the language is chosen

Language handling is deliberately split between the server and the theme:

1. The server resolves one **visitor language** per request, in this order:
   `?lang=` query parameter → `vexgo_lang` cookie → `Accept-Language` header → the site's configured default → `en`.
   A valid `?lang=` value is remembered for a year in the cookie, so a language switch survives navigation.
2. The theme ships flat key/value dictionaries as `i18n/<lang>.json`.
3. The server merges dictionaries along the fallback chain **`en` → site default → visitor language**, so a partially translated theme degrades key by key instead of failing the page. A key that no dictionary supplies renders as the key itself, which makes a missing translation obvious on the page rather than invisible.

The merge order has one consequence worth knowing: the site default language **overrides** the theme's `en` strings, and the visitor's language overrides both. A theme that ships `en.json` and `zh.json` on a site configured for Chinese therefore renders Chinese to an English-speaking visitor only if `en.json` exists and the visitor's language resolves to `en` — which it does, since `en` is the last fallback.

The resolved language is exposed to templates as `.Site.Language`, and the `t` helper reads from the merged dictionary:

```html
<h1>{{t "nav.home"}}</h1>
```

The backend hardcodes no strings of its own. Whatever a theme ships under `i18n/` is exactly what can be served, which is why a theme without an `i18n/` directory still works: every `{{t "..."}}` renders its key.

## Caching and hot reload

Parsed templates are cached per theme, keyed by the newest modification time of the theme's files, so a theme is re-read from disk only when it changes. This has two practical effects:

- Uploading, overwriting or deleting a theme invalidates its cache automatically; edited templates appear on the next request without restarting the server.
- Switching the active theme is a database write, not a deploy.

For theme authors this means iteration does not require a rebuild: edit a template, re-upload the theme (or overwrite its files), reload the page.

## Seed pages

A theme can bootstrap its own custom pages by shipping Markdown files under `seed/`:

```text
seed/about.md
seed/links.md
```

Each file is `<slug>.md` with optional YAML frontmatter, and the slug must match the page slug rule (lowercase ASCII letters, digits and hyphens). When a theme is activated, the server creates a page for every seed file whose slug does not already exist, and it does so on every startup as well.

The key word is **does not already exist**: seeds are a starting point, never a reset. Editing or deleting a seeded page in the admin panel is safe — the next activation will not recreate or overwrite it. Seed pages are authored by the site's first super admin (falling back to the first admin), so a fresh install with no admin simply skips seed creation.

This is how the default theme gets its **Links** and **Timeline** pages into the navigation on first run.

## Trust boundaries

The public SSR surface and the admin surface share an origin, so the theme system's trust model is worth stating plainly:

- A theme's templates and its `assets/` JavaScript run in the public site's origin. A malicious theme is therefore equivalent to script injection on the public site — including the ability to read the admin panel's `localStorage` if an administrator browses the themed site. Uploading a theme is an admin-only action for exactly this reason.
- Theme templates cannot execute Go, register routes or query the database. They receive the data context described in the [Theme Templates reference](/reference/theme-templates) and nothing else.
- Markdown content is rendered with GFM extensions and safe defaults: raw HTML in Markdown is escaped rather than passed through. The resulting HTML is exposed to templates as `ContentHTML` and is the only value a theme may emit unescaped.
- Everything else a template prints is escaped by Go's `html/template`, so a post title containing `<script>` is displayed as text, not executed.

## Related Reading

- [Theme Development guide](/guides/theme-development) — build, package and install a theme.
- [Theme Templates reference](/reference/theme-templates) — every template context field, helper and file format.
- [Architecture](/concepts/architecture) — where the renderer sits in the backend.
