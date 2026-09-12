# Theme Development

> **How-to guide** — build a VexGo theme, package it and install it, then add the pages, translations and interactions you need. Each section is a task you can complete on its own. Field-by-field details live in the [Theme Templates reference](/reference/theme-templates); the reasoning behind the system is in [Theming](/concepts/theming).

## Before you start

You need:

- A running VexGo instance with an admin account (to upload the theme).
- A text editor. No compiler, Node.js or build step is required — a theme is plain HTML, CSS and JavaScript.
- Somewhere to look up what a template receives: the [Theme Templates reference](/reference/theme-templates).

The best reference implementation is the built-in default theme. Its source lives in `frontend-public/` (React components that emit the templates at build time) and the generated result is in `backend/internal/public/default-theme/`. Read it for patterns, but note that the generated `.html` files are minified into a single line and are **build output** — do not edit them, and prefer writing your templates by hand.

## Step 1: Create a minimal theme

The smallest theme that installs and renders has one manifest and one template:

```text
my-theme/
├── vexgo-theme.json
└── index.html
```

`vexgo-theme.json`:

```json
{
  "id": "my-theme",
  "name": "My Theme",
  "author": "Your Name",
  "version": "1.0.0",
  "description": "A minimal VexGo theme."
}
```

`index.html`:

```html
<!DOCTYPE html>
<html lang="{{.Site.Language}}">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Site.Name}}</title>
  </head>
  <body>
    <h1>{{.Site.Name}}</h1>
    {{range .Posts}}
    <article>
      <h2><a href="{{.URL}}">{{.Title}}</a></h2>
      <p>{{.Excerpt}}</p>
    </article>
    {{end}}
  </body>
</html>
```

That is a working home page. Everything else in this guide is additive.

If the active theme is missing a template for a route, that route returns a 404 — so add `post.html`, `page.html` and `user.html` before you want those pages to work, or ship only the home page at first.

## Step 2: Fill in the manifest

`vexgo-theme.json` sits at the root of the theme and is read at upload time. These fields are required:

| Field     | Notes                                                                                                     |
| --------- | --------------------------------------------------------------------------------------------------------- |
| `id`      | Must match the theme's directory name exactly. Letters, digits, `_`, `-`. Cannot be `default` (reserved). |
| `name`    | Display name shown in the admin panel.                                                                    |
| `version` | Your own version string; VexGo stores it but does not compare versions.                                   |

These are optional but shown in the admin theme list:

| Field         | Notes                                                                                                                                   |
| ------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `author`      | Author name.                                                                                                                            |
| `description` | One-line description.                                                                                                                   |
| `url`         | Theme homepage or repository.                                                                                                           |
| `preview`     | Cover image **URL** shown in the theme picker. Must be `http://` or `https://`. A relative path such as `assets/cover.png` is rejected. |

Unknown fields are ignored, so you can keep extra keys for your own tooling.

## Step 3: Package and install

VexGo accepts a `.zip` archive with either of two layouts:

```text
# Layout A — top-level folder (recommended)
my-theme.zip
└── my-theme/
    ├── vexgo-theme.json
    └── index.html

# Layout B — files at the archive root
my-theme.zip
├── vexgo-theme.json
└── index.html
```

In both cases the theme is installed under `data/theme/<id>/`, where `<id>` is the manifest's `id` (which in layout A must also equal the folder name). The archive itself may have any filename — it does not have to match the theme id.

Build the archive from a directory so the manifest lands where the server expects it:

```bash
cd my-theme && zip -r ../my-theme.zip .
```

Then, in the admin panel, open **Settings → Theme**, upload the archive, and activate it. If you prefer the API, the endpoint is documented in the [API reference](/reference/api); the archive goes in a `theme` multipart part.

Uploads are validated before anything is written:

| Limit                | Value   |
| -------------------- | ------- |
| Archive size         | 32 MiB  |
| Total unpacked size  | 100 MiB |
| Files in the archive | 2000    |
| Single unpacked file | 10 MiB  |

Archives that exceed these limits, or that have no readable `vexgo-theme.json`, are rejected with a `400`. The same goes for a manifest whose `id` does not match its folder, or whose `preview` is not an `http(s)` URL.

## Step 4: Write the post page

`post.html` is used for `/post/<slug>`. The post data is nested under `.Post`:

```html
<!DOCTYPE html>
<html lang="{{.Site.Language}}">
  <head>
    <title>{{.Post.Title}} — {{.Site.Name}}</title>
  </head>
  <body>
    <article>
      <h1>{{.Post.Title}}</h1>
      <p>
        {{date .Post.CreatedAt "2006-01-02"}} · {{.Post.ViewCount}} views ·
        {{.Post.LikesCount}} likes
      </p>
      {{if .Post.CoverImage}}
      <img src="{{.Post.CoverImage}}" alt="{{.Post.Title}}" />
      {{end}}
      <div class="content">{{.Post.ContentHTML}}</div>
    </article>

    <div
      id="vexgo-comments"
      data-post-id="{{.Post.ID}}"
      data-lang="{{.Site.Language}}"
    ></div>
    <script src="/theme-assets/comments.js" defer></script>
  </body>
</html>
```

Two things to notice:

- **`{{.Post.ContentHTML}}` is the rendered Markdown body.** It is the one value you should emit unescaped; Go's templates know it is safe HTML. Do not wrap it in `<pre>` or escape it.
- **The author is optional.** A post may have no author row, so guard any use of `.Post.AuthorName` with `{{if .Post.AuthorID}}` if your layout requires one.

Drafts, pending and rejected posts are not reachable on this route regardless of the slug — only the admin draft preview can see them, and that is not something a theme has to handle.

## Step 5: Write the custom page and user pages

`page.html` is the generic template for custom pages such as `/about` or `/contact`:

```html
<!DOCTYPE html>
<html lang="{{.Site.Language}}">
  <head>
    <title>{{.Page.Title}} — {{.Site.Name}}</title>
  </head>
  <body>
    <h1>{{.Page.Title}}</h1>
    <div class="content">{{.Page.ContentHTML}}</div>
  </body>
</html>
```

`user.html` renders a public profile at `/user/<id>`, including that user's published posts and pagination:

```html
<!DOCTYPE html>
<html lang="{{.Site.Language}}">
  <head>
    <title>{{.User.Username}} — {{.Site.Name}}</title>
  </head>
  <body>
    <img src="{{.User.Avatar}}" alt="{{.User.Username}}" />
    <h1>{{.User.Username}}</h1>
    {{if .User.Bio}}
    <p>{{.User.Bio}}</p>
    {{end}}
    <p>{{.User.PostsCount}} posts</p>

    {{range .Posts}}
    <article><a href="{{.URL}}">{{.Title}}</a></article>
    {{end}} {{if .Pagination.HasPrev}}<a href="{{.Pagination.PrevURL}}"
      >Previous</a
    >{{end}} {{if .Pagination.HasNext}}<a href="{{.Pagination.NextURL}}">Next</a
    >{{end}}
  </body>
</html>
```

`{{.User.Bio}}` is already empty when the user has hidden their bio, so you do not need to check a visibility flag.

## Step 6: Add a dedicated layout for one page

Any plain lowercase `<slug>.html` file at the theme root becomes the dedicated template for the custom page with that slug. If a theme has `links.html` and a page with the slug `links`, that page renders with `links.html` instead of `page.html`.

This is how the default theme gives its `links` and `timeline` pages bespoke layouts. Note that a slash in the name disqualifies it — `pages/links.html` is not a page template.

To make the page exist in the first place, ship a seed file. `seed/links.md`:

````markdown
---
title: Links
showInNav: true
sortOrder: 101
status: published
---

My friends.

```friends
VexGo | https://github.com/vexgo-org/vexgo |  | A self-hosted blog CMS
```
````

The frontmatter keys are `title` (defaults to the filename), `showInNav` (`true` or `1`), `sortOrder` (integer) and `status` (`published` by default, `draft` supported). The body is Markdown. The filename becomes the slug and must be lowercase ASCII letters, digits and hyphens.

Seeds are created only for slugs that do not already exist — they bootstrap a fresh install and never overwrite an edited page.

## Step 7: Translate the theme

Put flat JSON dictionaries under `i18n/`:

```text
i18n/
├── en.json
└── zh.json
```

`i18n/en.json`:

```json
{
  "nav.home": "Home",
  "post.comments": "Comments"
}
```

`i18n/zh.json`:

```json
{
  "nav.home": "首页",
  "post.comments": "评论"
}
```

Use the `t` helper anywhere in a template:

```html
<a href="/">{{t "nav.home"}}</a>
```

How a visitor's language is picked:

```text
?lang=zh  →  vexgo_lang cookie  →  Accept-Language  →  site default  →  en
```

Dictionaries are merged **per key** along the chain `en` → site default → visitor language, so a missing translation falls back rather than breaking the page. A key that no dictionary defines renders as the key itself, which makes omissions visible during development.

Two practical notes:

- Ship an `en.json` in every theme. It is the base of the chain, and a visitor whose language you do not ship will fall back to it.
- Language codes are normalized: `zh-CN`, `zh_CN` and `ZH` all resolve to `zh`, and the dictionary file is named after the normalized code (`i18n/zh.json`).

## Step 8: Style the theme with assets

Put CSS, JavaScript and images in `assets/` and reference them through the stable prefix:

```text
my-theme/
└── assets/
    ├── style.css
    └── logo.svg
```

```html
<link rel="stylesheet" href="/theme-assets/style.css" />
<img src="/theme-assets/logo.svg" alt="logo" />
```

The `assets/` segment is added by the server, so a file at `assets/style.css` is served at `/theme-assets/style.css` — **not** `/theme-assets/assets/style.css`. This indirection is what lets a template keep working when the active theme changes.

## Step 9: Add optional interactions

Because public pages are server-rendered, anything interactive is progressive enhancement. The two features most themes want are already available as drop-in scripts.

### Comments and likes

The comment widget is self-contained and styles itself, so it renders in any theme. Include it on the post page:

```html
<div
  id="vexgo-comments"
  data-post-id="{{.Post.ID}}"
  data-lang="{{.Site.Language}}"
></div>
<script src="/theme-assets/comments.js" defer></script>
```

The `data-post-id` attribute is required; `data-lang` selects the widget's built-in strings (`en` and `zh` are included) and falls back to English. The widget talks to the public comment and like API, and reuses the admin SPA's stored session, so visitors who have logged in can post without the theme implementing authentication.

If you want to replace the widget with your own, keep the same `id`/`data-post-id` convention, or drop it entirely — the server never injects it.

### Dark mode

Dark mode is a theme concern: the default theme toggles a class on `<html>` and persists the choice in `localStorage`. There is no backend setting for it, so implement it however fits your markup.

## Limitations to keep in mind

- **Templates are Go `html/template`.** They cannot call arbitrary functions, loop over the database or access the filesystem. Only the values and helpers in the [Theme Templates reference](/reference/theme-templates) are available.
- **All root-level `.html` files are parsed into one set.** A `{{define}}` block in one file is visible from every other file, and files are parsed in sorted filename order, so keep shared fragments in one place to avoid duplicate definitions.
- **A syntax error anywhere blocks activation.** Run the theme locally first if you can, and install it in a staging instance before switching a production site.
- **Every page must be a complete document.** There is no layout inheritance; use `{{define}}`/`{{template}}` for shared markup.
- **Content markup is server-authored.** Markdown is rendered by the server in safe mode, with raw HTML escaped. A theme cannot opt into raw HTML from posts, and should not try to re-render `ContentHTML`.

## Troubleshooting

| Symptom                                  | Likely cause                                                                                                                            |
| ---------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| Theme installs but every page is a 404   | The `.html` files are nested inside a folder that does not match `id`, or they are under `dist/`. Templates must be at the theme root.  |
| Upload rejected with "Theme ID mismatch" | The manifest's `id` does not equal the folder it sits in. Rename either one.                                                            |
| Upload rejected as "Invalid preview URL" | `preview` is a relative path. Use a full `http(s)://` URL, or drop the field.                                                           |
| Pages render but CSS is missing          | The stylesheet is referenced as `/theme-assets/assets/style.css`. Drop the extra `assets/` segment.                                     |
| A page renders the generic layout        | Its `<slug>.html` was not recognized: the filename must be a plain lowercase `.html` at the theme root, matching the page slug exactly. |
| Translations do not change               | The dictionary filename must be the normalized language (`zh.json`, not `zh-CN.json`), and `t` must be used instead of hardcoded text.  |
| A seed page appears only once            | That is the contract: seeds never overwrite an existing page, including a page you edited.                                              |
| Edits to a live theme do not appear      | Re-upload the theme (uploading invalidates the cache), or overwrite its files in `data/theme/<id>/`.                                    |
