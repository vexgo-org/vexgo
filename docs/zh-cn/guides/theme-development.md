# 主题开发指南

> **操作指南** —— 编写一个 VexGo 主题、打包并安装它，然后按需添加页面、翻译和交互。每一节都是一个可以独立完成的任务。逐字段的细节见[主题模板参考](/zh-cn/reference/theme-templates)，系统的设计原理见[主题系统](/zh-cn/concepts/theming)。

## 开始之前

你需要：

- 一个正在运行的 VexGo 实例，以及一个管理员账号（用于上传主题）。
- 一个文本编辑器。不需要编译器、Node.js 或构建步骤——主题就是普通的 HTML、CSS 和 JavaScript。
- 一个查询模板可用数据的地方：[主题模板参考](/zh-cn/reference/theme-templates)。

最好的参考实现是内置的默认主题。它的源码在独立的 [vexgo-default-theme](https://github.com/vexgo-org/vexgo-default-theme) 仓库（构建时由 React 组件生成模板）；`scripts/fetch-default-theme.sh` 会把它的 `dist/` 构建产物复制到 `backend/internal/public/default-theme/` 以便嵌入。可以读它的写法找思路，但要注意生成的 `.html` 被压缩成了单行，并且是**构建产物**——不要直接改它，也不建议照搬，手写自己的模板更好。

## 第 1 步：创建最小主题

能装能渲染的最小主题只需要一个清单和一个模板：

```text
my-theme/
├── vexgo-theme.json
└── index.html
```

`vexgo-theme.json`：

```json
{
  "id": "my-theme",
  "name": "My Theme",
  "author": "Your Name",
  "version": "1.0.0",
  "description": "A minimal VexGo theme."
}
```

`index.html`：

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

这就是一个可用的首页。本指南后面所有内容都是在此基础上叠加的。

如果当前主题缺少某个路由对应的模板，那个路由会返回 404——所以在需要文章页、自定义页和用户页之前，先把 `post.html`、`page.html`、`user.html` 补上；也可以先只发布首页。

## 第 2 步：补齐清单

`vexgo-theme.json` 位于主题根目录，在上传时被读取。以下字段必填：

| 字段      | 说明                                                                             |
| --------- | -------------------------------------------------------------------------------- |
| `id`      | 必须与主题目录名完全一致。可用字母、数字、`_`、`-`。不能为 `default`（保留值）。 |
| `name`    | 显示在管理面板中的名称。                                                         |
| `version` | 你自己的版本号；VexGo 会存储它，但不做版本比较。                                 |

以下字段可选，会显示在后台主题列表中：

| 字段          | 说明                                                                                                         |
| ------------- | ------------------------------------------------------------------------------------------------------------ |
| `author`      | 作者名。                                                                                                     |
| `description` | 一句话描述。                                                                                                 |
| `url`         | 主题主页或仓库地址。                                                                                         |
| `preview`     | 主题选择器里的封面图 **URL**，必须是 `http://` 或 `https://`。像 `assets/cover.png` 这样的相对路径会被拒绝。 |

未知字段会被忽略，所以你可以放一些自己工具用的额外键。

## 第 3 步：打包并安装

VexGo 接受 `.zip` 包，目录结构可以是下面两种之一：

```text
# 结构 A —— 顶层文件夹（推荐）
my-theme.zip
└── my-theme/
    ├── vexgo-theme.json
    └── index.html

# 结构 B —— 文件直接在压缩包根目录
my-theme.zip
├── vexgo-theme.json
└── index.html
```

两种情况下主题都会被安装到 `data/theme/<id>/`，其中 `<id>` 来自清单里的 `id`（结构 A 中还要求文件夹名与它一致）。压缩包本身可以叫任意名字，不要求和主题 id 相同。

建议从目录内部打包，这样清单会落在服务端期望的位置：

```bash
cd my-theme && zip -r ../my-theme.zip .
```

然后在管理面板打开 **设置 → 主题**，上传压缩包并激活。如果更习惯用 API，接口见 [API 参考](/zh-cn/reference/api)；压缩包放在 `theme` 这个 multipart 字段里。

上传会在写入任何文件之前完成校验：

| 限制             | 值      |
| ---------------- | ------- |
| 压缩包大小       | 32 MiB  |
| 解压后总大小     | 100 MiB |
| 压缩包内文件数量 | 2000    |
| 单个解压文件     | 10 MiB  |

超过这些限制、或没有可读的 `vexgo-theme.json`，都会返回 `400`。清单的 `id` 与文件夹不匹配、或 `preview` 不是 `http(s)` URL，同样会被拒绝。

## 第 4 步：编写文章页

`post.html` 用于 `/post/<slug>`，文章数据位于 `.Post` 之下：

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

有两点需要注意：

- **`{{.Post.ContentHTML}}` 是渲染好的 Markdown 正文。** 这是唯一一个应当以不转义方式输出的值；Go 模板知道它是安全的 HTML。不要把它包进 `<pre>` 或做转义。
- **作者是可选的。** 文章可能没有作者记录，所以如果布局强依赖作者，请用 `{{if .Post.AuthorID}}` 包住 `.Post.AuthorName` 的使用。

无论 slug 是什么，草稿、待审和被拒的文章在这条路由上都无法访问，只有管理员草稿预览例外——而这不是主题需要处理的事。

## 第 5 步：编写自定义页和用户页

`page.html` 是自定义页面（例如 `/about`、`/contact`）的通用模板：

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

`user.html` 渲染 `/user/<id>` 的用户公开主页，包含该用户已发布的文章和分页：

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

当用户隐藏了个人简介时，`{{.User.Bio}}` 本身已经是空字符串，不需要再判断可见性开关。

## 第 6 步：为某个页面定制专属布局

主题根目录下任意一个纯小写的 `<slug>.html`，都会成为该 slug 对应自定义页面的专属模板。如果主题有 `links.html`，且存在 slug 为 `links` 的页面，那么该页面会用 `links.html` 渲染，而不是 `page.html`。

默认主题就是这样给它的 `links` 和 `timeline` 页面配上专属布局的。注意文件名里不能有斜杠——`pages/links.html` 不算页面模板。

要让这个页面存在，还需要提供一个种子文件。`seed/links.md`：

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

frontmatter 支持 `title`（默认取文件名）、`showInNav`（`true` 或 `1`）、`sortOrder`（整数）和 `status`（默认 `published`，也支持 `draft`）。正文是 Markdown。文件名即 slug，必须是小写 ASCII 字母、数字和连字符。

种子只会为尚不存在的 slug 创建页面——它们用于引导全新安装，永远不会覆盖已编辑的页面。

## 第 7 步：为主题做多语言

在 `i18n/` 下放扁平 JSON 字典：

```text
i18n/
├── en.json
└── zh.json
```

`i18n/en.json`：

```json
{
  "nav.home": "Home",
  "post.comments": "Comments"
}
```

`i18n/zh.json`：

```json
{
  "nav.home": "首页",
  "post.comments": "评论"
}
```

在模板任意位置使用 `t` 辅助函数：

```html
<a href="/">{{t "nav.home"}}</a>
```

访客语言的确定顺序：

```text
?lang=zh  →  vexgo_lang cookie  →  Accept-Language  →  站点默认语言  →  en
```

字典会按 `en` → 站点默认语言 → 访客语言 的顺序**逐键**合并，因此缺少某个翻译时只会回退，不会让整页失败。所有字典都没有的键会渲染成键名本身，方便在开发阶段发现遗漏。

两个实用建议：

- 每个主题都应提供 `en.json`。它是回退链的基底，你未提供其语言字典的访客会回退到这里。
- 语言代码会被归一化：`zh-CN`、`zh_CN` 和 `ZH` 都解析为 `zh`，字典文件名用归一化后的代码（`i18n/zh.json`）。

## 第 8 步：用静态资源做样式

把 CSS、JavaScript 和图片放在 `assets/`，通过固定前缀引用：

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

`assets/` 这一段由服务端补上，所以 `assets/style.css` 的访问地址是 `/theme-assets/style.css`，**不是** `/theme-assets/assets/style.css`。正是这层间接让模板在当前主题改变后依然可用。

## 第 9 步：添加可选交互

公开页面是服务端渲染的，所以任何交互都属于渐进增强。主题最想要的两个功能，都已有现成的脚本可以直接引入。

### 评论与点赞

评论组件是自包含的，样式也自带，因此能在任何主题里正常显示。在文章页加入：

```html
<div
  id="vexgo-comments"
  data-post-id="{{.Post.ID}}"
  data-lang="{{.Site.Language}}"
></div>
<script src="/theme-assets/comments.js" defer></script>
```

`data-post-id` 必填；`data-lang` 选择组件内置文案（自带 `en` 和 `zh`），未匹配时回退英文。组件通过公开的评论与点赞 API 通信，并复用管理面板存储的登录态，所以已登录的访客可以直接评论，主题不需要自己实现认证。

如果你想用自己的组件替换它，保留相同的 `id`/`data-post-id` 约定即可，也可以完全去掉——服务端从不注入它。

### 深色模式

深色模式属于主题自己的事：默认主题在 `<html>` 上切换 class，并把选择存进 `localStorage`。后端没有对应设置项，按你的标记结构实现即可。

## 需要注意的限制

- **模板就是 Go 的 `html/template`。** 不能调用任意函数、遍历数据库或访问文件系统，可用的只有[主题模板参考](/zh-cn/reference/theme-templates)里列出的数据和辅助函数。
- **根目录所有 `.html` 会被解析进同一个集合。** 一个文件里的 `{{define}}` 对其他所有文件可见，且文件按名称排序解析，所以共享片段要集中放置，避免重复定义。
- **任何一处语法错误都会阻止激活。** 有条件的话先在本地跑一遍，再装到预发实例上，确认无误后再切换生产站点。
- **每个页面都必须是完整文档。** 没有布局继承机制，共享标记请使用 `{{define}}`/`{{template}}`。
- **正文标记由服务端生成。** Markdown 由服务端以安全模式渲染，原始 HTML 被转义。主题无法选择开启文章里的原始 HTML，也不应尝试重新渲染 `ContentHTML`。

## 故障排查

| 现象                           | 可能原因                                                                                            |
| ------------------------------ | --------------------------------------------------------------------------------------------------- |
| 主题能安装，但所有页面都是 404 | `.html` 文件被放在与 `id` 不一致的文件夹里，或放在了 `dist/` 下。模板必须位于主题根目录。           |
| 上传报 "Theme ID mismatch"     | 清单里的 `id` 与它所在的文件夹名不一致。改其中之一。                                                |
| 上传报 "Invalid preview URL"   | `preview` 是相对路径。改用完整的 `http(s)://` URL，或删掉该字段。                                   |
| 页面能渲染但样式没加载         | 样式表被引用成了 `/theme-assets/assets/style.css`。去掉多余的 `assets/` 段。                        |
| 页面用了通用布局               | 它的 `<slug>.html` 未被识别：文件名必须是主题根目录下纯小写的 `.html`，且与页面 slug 完全一致。     |
| 翻译不生效                     | 字典文件名必须是归一化后的语言（`zh.json`，不是 `zh-CN.json`），且模板中用的是 `t` 而非硬编码文本。 |
| 种子页面只出现一次             | 这是约定：种子永不覆盖已存在的页面，包括你编辑过的页面。                                            |
| 改了线上主题却看不到变化       | 重新上传主题（上传会刷新缓存），或直接覆盖 `data/theme/<id>/` 里的文件。                            |
