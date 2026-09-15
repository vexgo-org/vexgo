# 主题模板参考

> **技术参考** —— VexGo 主题可用的全部文件、字段、函数与限制，供查阅。设计原理见[主题系统](/zh-cn/concepts/theming)，实操步骤见[主题开发指南](/zh-cn/guides/theme-development)。

## 主题目录结构

主题目录（ZIP 上传后解压到 `data/theme/<id>/`）：

| 路径               | 必填 | 用途                                                             |
| ------------------ | ---- | ---------------------------------------------------------------- |
| `vexgo-theme.json` | 是   | 主题清单。缺少它主题不会被列出，也无法激活。                     |
| `index.html`       | 否\* | 首页模板（`IndexData`）。                                        |
| `post.html`        | 否\* | 文章详情模板（`PostData`）。                                     |
| `page.html`        | 否\* | 自定义页面通用模板（`PageData`）。                               |
| `user.html`        | 否\* | 用户公开主页模板（`UserData`）。                                 |
| `404.html`         | 否   | 未找到页面模板（`NotFoundData`），也是自定义页面的最后一级回退。 |
| `<slug>.html`      | 否   | 该 slug 对应自定义页面的专属模板。纯小写、不含斜杠。             |
| `i18n/<lang>.json` | 否   | 扁平的 `键: 文本` 字典。没有它时 `{{t "key"}}` 渲染成键名本身。  |
| `seed/<slug>.md`   | 否   | 激活时创建的默认页面；永不覆盖已存在的页面。                     |
| `assets/<file>`    | 否   | 静态文件，通过 `/theme-assets/<file>` 提供。                     |
| 其他任意文件       | 否   | 会保留在磁盘上，但渲染器不读取。                                 |

\* 主题必须至少提供 `index.html`、`post.html`、`page.html`、`user.html` 或 `404.html` 之一，否则激活时报 "theme has no template files"。缺少模板的路由会直接返回 404。

所有模板会被解析进同一个模板集合，因此 `{{define}}`/`{{template}}` 片段可跨文件共享。除上述五个名称之外的根目录 `.html` 文件，都会被当作自定义页面的专属模板。

## `vexgo-theme.json`

| 字段          | 类型   | 必填 | 规则                                                                                        |
| ------------- | ------ | ---- | ------------------------------------------------------------------------------------------- |
| `id`          | string | 是   | `^[A-Za-z0-9_-]+$`，必须与安装目录一致，不能为 `default`（保留值）。                        |
| `name`        | string | 是   | 非空显示名。                                                                                |
| `version`     | string | 是   | 非空版本号。仅存储，不做比较。                                                              |
| `author`      | string | 否   | 作者名。                                                                                    |
| `description` | string | 否   | 一句话描述。                                                                                |
| `url`         | string | 否   | 主题主页或仓库地址。                                                                        |
| `preview`     | string | 否   | 封面图 URL，必须是带主机的 `http://` 或 `https://` 且不超过 2048 个字符；相对路径会被拒绝。 |

清单用普通 JSON 反序列化解析，未知字段会被忽略。

## 页面、模板与上下文

| 请求                                         | 模板解析规则                                                   | 上下文         |
| -------------------------------------------- | -------------------------------------------------------------- | -------------- |
| `/`，支持 `?search=`、`?category=`、`?page=` | `index.html`                                                   | `IndexData`    |
| `/post/<slug>` 与 `/posts/<slug>`            | `post.html`                                                    | `PostData`     |
| `/<slug>`                                    | `<slug>.html`，否则 `page.html`，否则 `404.html`（状态码 404） | `PageData`     |
| `/user/<id>`                                 | `user.html`                                                    | `UserData`     |
| 未匹配路径 / 内容不存在                      | `404.html`                                                     | `NotFoundData` |

只渲染已发布的文章和页面；草稿会渲染 404 模板。若渲染时模板解析失败，响应体退化为纯文本 `404`。

## 模板上下文

### `SiteData`

每个页面都以 `.Site` 提供。

| 字段              | 类型   | 说明                                        |
| ----------------- | ------ | ------------------------------------------- |
| `Name`            | string | 站点名（默认 `VexGo`）。                    |
| `Description`     | string | 站点描述。                                  |
| `Icon`            | string | 已配置的站点图标 URL，可能为空。            |
| `URL`             | string | 已配置的站点根 URL。                        |
| `ItemsPerPage`    | int    | 列表页每页文章数（默认 20）。               |
| `Language`        | string | 本次请求解析出的访客语言，例如 `en`、`zh`。 |
| `DefaultLanguage` | string | 站点设置中的默认语言。                      |

### `NavPageData`

导航项，每个页面都以 `.Pages` 提供。每个**已发布**且启用了 `showInNav` 的自定义页面占一项，按 `SortOrder` 升序、再按页面 id 排序。

| 字段        | 类型   | 说明                            |
| ----------- | ------ | ------------------------------- |
| `Title`     | string | 页面标题。                      |
| `Slug`      | string | 页面 slug。                     |
| `URL`       | string | 可直接使用的 URL（`/<slug>`）。 |
| `SortOrder` | int    | 排序提示，越小越靠前。          |

### `PostCardData`

文章列表（首页与用户页的 `.Posts`，以及 `.PopularPosts`）使用的摘要结构。

| 字段            | 类型      | 说明                                 |
| --------------- | --------- | ------------------------------------ |
| `ID`            | uint      | 文章 ID。                            |
| `Title`         | string    | 标题。                               |
| `Slug`          | string    | slug。                               |
| `Excerpt`       | string    | 摘要。                               |
| `CoverImage`    | string    | 封面图 URL，可能为空。               |
| `Category`      | string    | 分类名，可能为空。                   |
| `Tags`          | []string  | 标签名，可能为空。                   |
| `AuthorName`    | string    | 文章无作者时为空。                   |
| `AuthorID`      | uint      | 文章无作者时为 `0`。                 |
| `AuthorAvatar`  | string    | 作者头像 URL。                       |
| `CreatedAt`     | time.Time | 创建时间。                           |
| `ViewCount`     | int       | 阅读数。                             |
| `CommentsCount` | int64     | 评论数。                             |
| `LikesCount`    | int64     | 点赞数。                             |
| `URL`           | string    | 可直接使用的 URL（`/post/<slug>`）。 |

### `IndexData`

首页上下文。

| 字段           | 类型               | 说明                                                                |
| -------------- | ------------------ | ------------------------------------------------------------------- |
| `Site`         | \*`SiteData`       | 站点级数据。                                                        |
| `Posts`        | []`PostCardData`   | 当前页的已发布文章。                                                |
| `Pages`        | []`NavPageData`    | 导航项。                                                            |
| `Pagination`   | \*`PaginationData` | 始终有值；只有一页时其 `Pages` 为空。                               |
| `Query`        | `IndexQueryData`   | 当前生效的 `?search=` / `?category=` 过滤条件。                     |
| `Categories`   | []string           | 已发布文章中非空的去重分类名。                                      |
| `PopularPosts` | []`PostCardData`   | 最多 5 篇，按 点赞 × 5 + 阅读 排名，从最近 200 篇已发布文章中选取。 |
| `PopularTags`  | []`PopularTagData` | 最多 10 个热门标签，按使用量降序。                                  |

### `PostData`

文章详情上下文。文章本身嵌套在 `.Post` 之下。

| 字段                 | 类型            | 说明                                 |
| -------------------- | --------------- | ------------------------------------ |
| `Site`               | \*`SiteData`    | 站点级数据。                         |
| `Pages`              | []`NavPageData` | 导航项。                             |
| `Post.ID`            | uint            | 文章 ID（评论组件使用）。            |
| `Post.Title`         | string          | 标题。                               |
| `Post.Slug`          | string          | slug。                               |
| `Post.Excerpt`       | string          | 摘要。                               |
| `Post.CoverImage`    | string          | 封面图 URL。                         |
| `Post.Category`      | string          | 分类名。                             |
| `Post.Tags`          | []string        | 标签名。                             |
| `Post.AuthorName`    | string          | 文章无作者时为空。                   |
| `Post.AuthorID`      | uint            | 文章无作者时为 `0`。                 |
| `Post.AuthorAvatar`  | string          | 作者头像 URL。                       |
| `Post.CreatedAt`     | time.Time       | 创建时间。                           |
| `Post.UpdatedAt`     | time.Time       | 最后更新时间。                       |
| `Post.ViewCount`     | int             | 阅读数。                             |
| `Post.CommentsCount` | int64           | 评论数。                             |
| `Post.LikesCount`    | int64           | 点赞数。                             |
| `Post.ContentHTML`   | template.HTML   | 渲染后的 Markdown 正文；不转义输出。 |
| `Post.URL`           | string          | 可直接使用的 URL（`/post/<slug>`）。 |

### `UserData`

用户公开主页上下文。

| 字段              | 类型               | 说明                                  |
| ----------------- | ------------------ | ------------------------------------- |
| `Site`            | \*`SiteData`       | 站点级数据。                          |
| `Pages`           | []`NavPageData`    | 导航项。                              |
| `User.ID`         | uint               | 用户 ID。                             |
| `User.Username`   | string             | 用户名。                              |
| `User.Avatar`     | string             | 头像 URL。                            |
| `User.Bio`        | string             | 用户隐藏简介时为空。                  |
| `User.CreatedAt`  | time.Time          | 注册时间。                            |
| `User.PostsCount` | int64              | 已发布文章数。                        |
| `Posts`           | []`PostCardData`   | 该用户的已发布文章，已分页。          |
| `Pagination`      | \*`PaginationData` | 始终有值；只有一页时其 `Pages` 为空。 |

### `PageData`

自定义页面上下文（`page.html` 和任何 `<slug>.html` 都使用它）。

| 字段               | 类型            | 说明                                 |
| ------------------ | --------------- | ------------------------------------ |
| `Site`             | \*`SiteData`    | 站点级数据。                         |
| `Pages`            | []`NavPageData` | 导航项。                             |
| `Page.ID`          | uint            | 页面 ID。                            |
| `Page.Title`       | string          | 标题。                               |
| `Page.Slug`        | string          | slug。                               |
| `Page.CreatedAt`   | time.Time       | 创建时间。                           |
| `Page.UpdatedAt`   | time.Time       | 最后更新时间。                       |
| `Page.ContentHTML` | template.HTML   | 渲染后的 Markdown 正文；不转义输出。 |
| `Page.URL`         | string          | 可直接使用的 URL（`/<slug>`）。      |

### `NotFoundData`

| 字段    | 类型            | 说明         |
| ------- | --------------- | ------------ |
| `Site`  | \*`SiteData`    | 站点级数据。 |
| `Pages` | []`NavPageData` | 导航项。     |

### `PaginationData`

| 字段          | 类型             | 说明                                                                         |
| ------------- | ---------------- | ---------------------------------------------------------------------------- |
| `CurrentPage` | int              | 当前页，从 1 开始。                                                          |
| `TotalPages`  | int              | 总页数。                                                                     |
| `HasPrev`     | bool             | 是否存在上一页。                                                             |
| `HasNext`     | bool             | 是否存在下一页。                                                             |
| `PrevURL`     | string           | 上一页 URL（`HasPrev` 为 false 时为空）。                                    |
| `NextURL`     | string           | 下一页 URL（`HasNext` 为 false 时为空）。                                    |
| `Pages`       | []`PageLinkData` | 窗口化的页码链接：首页、末页和当前页 ± 1；`TotalPages` 为 1 时该切片为 nil。 |

列表页渲染成功时这个指针始终非 nil，且 `TotalPages` 至少为 1，所以只有一页时 `CurrentPage`、`HasPrev`、`HasNext` 依然可读。请用 `{{if .Pagination.HasNext}}` 而不是 `{{if .Pagination}}` 来判断分页。

### `PageLinkData`

| 字段        | 类型   | 说明                                            |
| ----------- | ------ | ----------------------------------------------- |
| `Number`    | int    | 页码。                                          |
| `URL`       | string | 该页 URL。                                      |
| `IsCurrent` | bool   | 是否为当前页。                                  |
| `Ellipsis`  | bool   | 是否为「…」分隔项；此时 `Number`/`URL` 未设置。 |

### `IndexQueryData`

| 字段       | 类型   | 说明                                                       |
| ---------- | ------ | ---------------------------------------------------------- |
| `Search`   | string | 当前 `?search=` 值；首页会用它作为子串匹配文章标题和正文。 |
| `Category` | string | 当前 `?category=` 值；与文章分类精确匹配。                 |

### `PopularTagData`

| 字段    | 类型   | 说明                       |
| ------- | ------ | -------------------------- |
| `Name`  | string | 标签名。                   |
| `Count` | int64  | 使用该标签的已发布文章数。 |

## 模板辅助函数

每个主题模板都可以调用以下函数：

| 辅助函数      | 签名               | 返回值                                                                                           |
| ------------- | ------------------ | ------------------------------------------------------------------------------------------------ |
| `date`        | `date t layout`    | 按 Go 布局格式化 `time.Time`，例如 `{{date .Post.CreatedAt "2006-01-02"}}`。                     |
| `add`         | `add a b`          | 两个整数之和，例如用于渲染从 1 开始的位置。                                                      |
| `first`       | `first items n`    | 最多 `n` 个元素；`n` 非正数时原样返回切片。                                                      |
| `truncate`    | `truncate s max`   | 去除首尾空白后按 **rune** 截断到 `max`，发生截断时追加 `...`；`max` 非正数时返回去空白后的原串。 |
| `userURL`     | `userURL id`       | `/user/<id>`。                                                                                   |
| `categoryURL` | `categoryURL name` | `/?category=<URL 转义>`。                                                                        |
| `searchURL`   | `searchURL name`   | `/?search=<URL 转义>`。                                                                          |
| `t`           | `t key`            | 合并字典中 `key` 的值；缺失时返回键名本身。                                                      |

使用建议：

- 列表布局中的摘要和标题请用 `truncate`；它按 rune 计数，中日韩文本不会被截断在字符中间。
- `first` 和 `truncate` 把限制值 `0` 视为「保持原样」而非「返回空」，所以配错的长度上限不会把内容清空。
- 构建 URL 请用 `userURL`、`categoryURL`、`searchURL`，不要手写。它们也能避免双引号出现在 HTML 属性里。
- `t` 需要主题提供 `i18n/<lang>.json`；没有字典时它渲染键名。

## `i18n/<lang>.json`

扁平的字符串键值对象，不支持嵌套。

```json
{
  "nav.home": "Home",
  "post.comments": "Comments"
}
```

- 文件名是**归一化后**的语言代码（`zh.json`，不是 `zh-CN.json`）。
- 字典按 `en` → 站点默认语言 → 访客语言 的顺序逐键合并。
- 所有字典都缺失的键会渲染成键名本身。

一次请求的语言解析顺序：

```text
?lang=  →  vexgo_lang cookie  →  Accept-Language  →  站点默认语言  →  en
```

`?lang=` 归一化后合法时，会写入 `vexgo_lang` cookie 并保留一年。

## 种子 frontmatter

`seed/<slug>.md` 文件支持可选的 `---` frontmatter：

| 键          | 类型   | 默认值      | 说明                     |
| ----------- | ------ | ----------- | ------------------------ |
| `title`     | string | 文件名 slug | 页面标题。               |
| `showInNav` | bool   | `false`     | 接受 `true` 或 `1`。     |
| `sortOrder` | int    | `0`         | 导航排序。               |
| `status`    | string | `published` | `published` 或 `draft`。 |

frontmatter 之后的内容即 Markdown 正文。slug 必须匹配 `^[a-z0-9-]{1,100}$`，不符合的文件会被跳过并记录警告。种子只在 slug 尚不存在时创建，触发时机为主题激活和服务启动；作者取站点第一个超级管理员（无则第一个管理员）。

## 资源 URL

| 磁盘路径              | URL                                                    |
| --------------------- | ------------------------------------------------------ |
| `assets/style.css`    | `/theme-assets/style.css`                              |
| `assets/images/a.png` | `/theme-assets/images/a.png`                           |
| `favicon.ico`         | `/favicon.ico`（在站点图标和 `data/favicon.ico` 之后） |

`/theme-assets/*` 解析到**当前**主题，因此模板里不包含主题 id。`assets/` 这一段由服务端补上，不能出现在 URL 中。

## 评论组件契约

文章模板中的即插即用标记：

```html
<div
  id="vexgo-comments"
  data-post-id="{{.Post.ID}}"
  data-lang="{{.Site.Language}}"
></div>
<script src="/theme-assets/comments.js" defer></script>
```

| 属性           | 必填 | 说明                                                      |
| -------------- | ---- | --------------------------------------------------------- |
| `id`           | 是   | 必须是 `vexgo-comments`；元素不存在时组件不工作。         |
| `data-post-id` | 是   | 文章 ID。缺少时组件直接退出。                             |
| `data-lang`    | 否   | 组件界面语言；内置文案覆盖 `en` 和 `zh`，其他值回退英文。 |

组件不依赖任何框架，自带内联样式，所有用户内容都用 `textContent` 渲染，通过公开的评论/点赞 API 通信，并复用管理面板存储的登录态。若主题自带 `assets/comments.js`，则替换内置组件；服务端从不注入。

## 上传限制

上传主题 ZIP 时应用，在写入磁盘之前校验：

| 限制             | 值      | 失败信息                         |
| ---------------- | ------- | -------------------------------- |
| 压缩包大小       | 32 MiB  | `400 Theme archive too large`    |
| 解压后总大小     | 100 MiB | `400 Theme archive too large`    |
| 压缩包内文件数量 | 2000    | `400 Theme archive too large`    |
| 单个解压文件     | 10 MiB  | `400 Theme archive too large`    |
| 文件扩展名       | `.zip`  | `400 File must be a zip archive` |

其他校验：压缩包必须包含可读的 `vexgo-theme.json`（位于根目录或单个顶层目录中）；`id`/`name`/`version` 必须存在；`id` 必须与安装目录一致；`preview` 必须是 `http(s)` URL。激活时还会额外要求每个模板都能解析通过。

## 相关阅读

- [主题系统](/zh-cn/concepts/theming) —— 渲染器如何使用这些文件。
- [主题开发指南](/zh-cn/guides/theme-development) —— 编写并安装主题。
- [架构](/zh-cn/concepts/architecture) —— 渲染器在后端中的位置。
