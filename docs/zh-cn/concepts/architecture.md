# 架构

> 本页介绍 VexGo 的设计：后端结构、角色与权限、内容审核管线、主题系统和 SSO。需要按步骤操作时请看各篇指南。

## 总览

VexGo 是一个自托管的博客 CMS，由两部分组成：

- Go 后端（`backend/`）：基于 Gin 和 GORM 构建的 HTTP API，提供管理面板、公开站点和 REST API。支持 SQLite、PostgreSQL 或 MySQL。
- React 前端（`frontend/`）：管理面板，基于 TypeScript + Vite + Tailwind CSS 的 SPA，与 API 通信。构建产物嵌入后端二进制，并只在 `/admin/...` 下提供；旧的一级路径（如 `/login`）会 301 重定向到对应位置且保留查询串，因此旧书签仍然可用。公开页面则由主题进行服务端渲染。

主题系统让后端用上传的主题渲染公开页面，访客无需 JavaScript 即可阅读内容。

## 后端结构

后端采用领域导向的目录结构，位于 `backend/internal` 下，通过组合根进行引导：

```text
backend/
  cmd/vexgo/main.go         # 入口：通过 cli 解析配置，委托给 app 包处理
  internal/
    api/                     # REST 接口共享的传输类型（响应体、错误结构）
    app/                     # 组合根：组装所有依赖
    asset/                   # 资产管理：基于不可变 blob 的上传与软删除
    auth/                    # 注册、登录、JWT、个人资料、密码重置、邮箱验证
    cache/                   # 缓存后端：进程内内存 + Valkey（valkey-io/valkey-go）
    captcha/                 # 滑块验证码的生成与校验
    cli/                     # cobra 命令行：参数、帮助、版本、.env 加载
    comment/                 # 评论和 AI 内容审核
    config/                  # 基于 viper 的分层配置解析，JWT、S3、SSO 初始化
    database/                # 连接与种子数据
    home/                    # 站点统计
    mailer/                  # SMTP 邮件构建与发送
    notification/            # 站内通知
    middleware/              # JWT 认证、角色权限、请求日志
    migration/               # 启动迁移：表结构、数据回填、敏感信息加密
    model/                   # GORM 数据模型 + 共享接口（Notifier、FileRemover）；无 JSON 标签
    page/                    # 自定义页面（/:slug）
    post/                    # 文章 CRUD、分类、标签、点赞
    public/                  # 嵌入的前端、主题、SSR 服务与处理器、静态路由
    router/                  # 路由注册（组合所有领域）
    secrets/                 # 数据库中敏感信息的 AES-256-GCM 加密
    settings/                # 管理员配置（SMTP、AI、通用、主题）
    sso/                     # GitHub / Google / OIDC 登录
    storage/                 # Storage 接缝后的文件后端（本地磁盘或 S3）
    upload/                  # 旧版上传接口与媒体文件记录，正由 asset/ 取代
    user/                    # 用户管理、角色、创作者申请
```

### 分层架构（每个领域）

每个领域包遵循三层模式：

```text
handler.go    → HTTP 请求解析、响应渲染（调用 service）
service.go    → 业务逻辑、跨领域编排（调用 repository）
repository.go → 持久化接口 + GORM 实现（调用数据库）
types.go      → API 传输类型（请求/响应 DTO），带 JSON 标签供 swag/orval 使用
```

- Handler 不直接操作 GORM，而是委托给 Service。
- Service 在 `Repository` 接口之后与数据库无关，因此无需真实数据库即可测试。各领域的测试用内存 SQLite 运行，接缝则用 fake。
- Repository 封装所有 SQL/GORM 查询，包括避免 N+1 的批量操作。
- **Handler 绑定 `types.go` 中的请求类型并直接传给 Service**；Service 返回数据库模型，Handler 再通过显式的 `toXxxResponse` 函数将其转为响应 DTO。

### 模型/DTO 分离

`internal/model/` 中的数据库模型**不带 JSON 标签** —— 测试 `TestModelsHaveNoJSONTags` 强制要求这一点。每个 HTTP 响应都使用领域 `types.go`（或 `internal/api/` 中的共享形状如 `api.UserSummary`）中声明的专用 DTO。这防止凭证、令牌、存储键泄露到响应或 OpenAPI schema 中，也阻止 Handler 意外嵌入裸模型。缓存读路径（`post/cached_repository.go`、`home/cached_repository.go`）更进一步：它们存储显式的线格式（`cachedPost`、`cachedAuthor`、`cachedSettings`），只保存访客可见的数据，因此共享 Valkey 缓存永远不会积累敏感字段。

### 共享接缝（`model/interfaces.go`）

`model` 包中只有两个跨领域接缝的小型接口，其余接口由消费方领域自行声明：

```go
// NotificationInput 是 CreateNotification 接收的通知字段（userID、type、title、content、relatedID、relatedType）。
type NotificationInput struct {
    UserID        uint
    Type          NotificationType
    Title         string
    Content       string
    RelatedID     string
    RelatedType   NotificationRelatedType
    RelatedPostID *uint
}

// Notifier 是创建通知的接缝；由 notification 领域实现。
type Notifier interface {
    CreateNotification(ctx context.Context, input NotificationInput) error
}

// FileRemover 按存储 key 删除已存储的文件；由 storage.Storage 实现。
type FileRemover interface {
    Delete(ctx context.Context, url string) error
}
```

具体实现由 `internal/app` 装配，所以没有任何领域包导入其他领域的具体类型：

- `notification` 实现 `Notifier`；由 `post`、`comment`、`user` 消费。
- `storage.Storage` 实现 `FileRemover`；由 `post`、`user`、`auth` 用于文件清理。
- `captcha` 实现 `auth` 自己声明的 `CaptchaChecker` 接缝；`post`/`home` 自行声明 `ReadCache`。`secrets` 是另一个例外：作为叶子包，它声明唯一的 `secrets.SecretCipher` 接口，由 `settings`、`comment`、`mailer` 共同消费。
- 邮件是例外：`mailer.Service` 以具体类型注入 `auth` 与 `settings`。它只负责发送；验证令牌、密码重置与帐号数据仍留在各领域的 repository 中。
- `public` 也是例外：它在启动时持有具体的 `*page.Service`，用于创建主题自带的种子页。

### 缓存后端（`internal/cache/`）

`internal/cache` 是一个叶子包，不 import 任何其他 backend 内部包。它提供一个 `Cache` 接口（`Get`/`Set`/`Delete`/`GetDel`/`Incr`）以及两个实现：进程内内存后端与基于 `valkey-io/valkey-go` 的 Valkey（兼容 Redis）后端。所有键以 `vexgo:` 前缀命名空间隔离，使服务器可以与无关应用共享。

遵循消费方声明接缝的惯例，没有任何领域包 import `cache`。取而代之：

- `middleware` 声明 `CounterStore`：分布式固定窗口限流器背后的原子自增
- `sso` 声明 `StateStore`：通过 `Set`/`GetDel` 实现的一次性 OAuth state
- `post` 与 `home` 声明 `ReadCache`：公开读路径的读穿透装饰器

`cache.Cache` 在结构上满足以上全部接口，组合根注入具体后端（memory，或启动时拨号并 PING 的 valkey 连接）。运行期存储错误在限流器中**fail-open**（可用性优先于防滥用），在 SSO state 检查中**fail-closed**（CSRF 防护保持完整）。

### 组合根（`internal/app/`）

`internal/app/app.go` 包是组合根，也称装配层。它：

1. 接收 `cli.Execute()` 解析完成的配置：cobra 负责命令行参数，viper 按「参数 > 配置文件 > 环境变量 > 默认值」分层。
2. 确保 JWT 密钥存在（开发环境生成随机兜底值）并应用前端地址默认值；SSO 结构体在配置解析阶段派生。
3. 打开数据库，运行迁移并写入种子数据。
4. 从 `settings_encryption_key` 构建静态加密 cipher（未设置时记录警告，敏感信息保持明文存储），并运行 `migration.Service.MigrateSecrets` 将仍为明文的敏感信息就地加密（幂等）。
5. 创建存储（本地或 S3）。
6. 实例化每个领域的依赖并将它们装配到路由器中。

`cmd/vexgo/main.go` 是一个轻量入口，调用 `cli.Execute()`，然后调用 `app.New(cfg)` 和 `app.Run()`。

### 依赖规则

包结构保证依赖图无环：

```text
cmd/vexgo/main.go
    ├─→ internal/cli          ← cobra 命令树，将参数绑定到 viper
    │       └─→ config        ← 分层解析（参数 > 文件 > 环境变量 > 默认值）
    └─→ internal/app          ← 组合根，导入所有包
            ├─→ config         ← 解析完成的 Config 类型（不导入领域包）
            ├─→ database       ← Open/Seed（导入 config、model）
            ├─→ migration      ← MigrateDB/MigrateSecrets（导入 model、secrets、asset、storage）
            ├─→ router         ← 路由注册（导入所有领域 handler）
            └─→ internal/*     ← 各领域包

叶子包（无内部导入）：
    model/         ← 数据模型 + 共享接缝，被所有领域包导入
    config/        ← 配置解析，被 cli、app、database、sso、storage 导入
    secrets/       ← 静态敏感信息的 AES-256-GCM cipher；声明共享的
                     secrets.SecretCipher 接缝，由 settings、comment、mailer 消费
    cache/         ← 内存 + Valkey 后端；各领域通过自己的窄接缝
                     （CounterStore、StateStore、ReadCache）消费
    storage/       ← 本地磁盘与 S3 文件后端（仅导入 config）；由 upload
                     上传媒体、migration 的媒体回填，以及
                     model.FileRemover 清理路径消费

共享层：
    middleware/    ← JWT 认证、角色权限、请求日志（仅导入 model）

跨领域边：
    auth/          ← 被 comment、post、sso 使用（隐私过滤）
    notification/  ← 实现 model.Notifier，被 comment、post、user 作为通知接缝使用
    storage/       ← storage.Storage 实现 model.FileRemover，被 user、auth、post
                     用于文件清理；upload 与 asset 领域通过它存储媒体，
                     migration 的媒体回填也会读取它
    captcha/       ← 实现 auth 声明的验证码检查接缝 auth.CaptchaChecker
    public/        ← 主题目录与预览链接，实现 settings.ThemeManager 接缝
    page/          ← 被 public 用来创建主题自带的种子页
    mailer/        ← 以具体类型 *mailer.Service 注入 auth 与 settings（只负责发送）
```

### 配置管理

配置按优先级链加载：

```text
命令行参数  →  配置文件（YAML）  →  环境变量  →  默认值
  （最高优先级）                              （最低优先级）
```

命令行用 cobra 定义（`internal/cli`），各来源由 viper 分层（`internal/config`）。分层顺序有一个直接结果：配置文件中的显式 `false` 会覆盖环境变量中的 `true`，因为环境变量只填充配置文件未设置的键。

`config.Config` 结构体保存所有运行时值。没有全局变量，JWT 密钥、SSO 配置和前端 URL 都是 `Config` 的字段，不是包级别变量。

### context.Context 传播

所有 service 和 repository 方法都以 `context.Context` 作为第一个参数。Handler 从 Gin 请求上下文传递 `c.Request.Context()`，可用于：

- 请求作用域的取消和超时
- 分布式追踪传播
- GORM `WithContext()` 查询取消

## 用户、角色与权限

认证基于 JWT。每个用户只有一个角色，每次请求都会根据数据库中的角色检查权限。

| 角色          | 权限                                             |
| ------------- | ------------------------------------------------ |
| `super_admin` | 一切操作。绕过所有权限检查。不能被其他用户修改。 |
| `admin`       | 审核内容、管理用户和设置、审批创作者申请         |
| `author`      | 直接发布文章                                     |
| `contributor` | 申请角色升级（创作者申请）                       |
| `guest`       | 新注册用户，受限访问                             |

权限检查是累积的：

- Is admin = `admin` 或 `super_admin`
- Is author = `author` 及以上角色
- Is contributor = `contributor` 及以上角色

### 创作者申请

新用户注册为 `guest`，可以提交创作者申请（附理由）请求升级。管理员审核队列，批准或拒绝每份申请；批准后用户升入更高角色层级。

## 内容审核

VexGo 有两条审核管线，文章和评论各一条。两者都围绕 `status` 字段：

- **文章**：`draft` → `pending` → `published` / `rejected`（被拒文章可重新提交）
- **评论**：`published`、`pending`、`rejected`

### 文章审核

作者发布文章时，可直接进入 `published`（若作者有发布权限），或进入 `pending` 等待管理员审核。管理员批准或拒绝，可附加拒绝原因。

### 评论审核

评论审核由三个相互独立的开关驱动，可在管理面板配置。默认全部关闭，此时新评论立即发布：

- 关键词过滤：包含屏蔽关键词的评论直接拒绝，不再调用大模型。
- 大模型审核：由已配置的大模型（OpenAI 兼容 API）按提示词审核每条评论。拒绝结论则评论被拒绝，通过结论仅在人工审核关闭时发布。任何大模型故障（网络、超时、非 200、非 JSON 响应）都会使评论保持 `pending`，因为整条管道 fail-closed：被误导或故障的模型最多把评论送进待审队列，绝不会发布垃圾内容。
- 人工审核：所有未被发布或拒绝的评论在队列中等待管理员裁决（人工终审）。

审核配置（开关、提示词、关键词、模型）存储在数据库中，通过管理面板或 `/moderation` API 端点管理。旧版本单一的「启用 AI 审核」开关会在启动时自动迁移为新的开关组合。

## 主题系统

公开页面由服务端渲染：`internal/public` 中的 SSR 服务执行当前主题的 Go 模板并返回渲染好的 HTML，因此访客无需执行 JavaScript 即可阅读内容。内置的默认主题始终可用，管理员可通过管理面板上传 ZIP 格式的主题。当前主题是数据库中的一个值，无需重启即可在运行时切换。

一个主题就是「模板 + 可选的翻译、种子页面和静态资源」构成的目录：

```text
my-theme/
├── vexgo-theme.json   # 元数据（id、name、version 等）
├── index.html         # 首页模板
├── post.html          # 文章详情模板
├── page.html          # 自定义页面通用模板
├── user.html          # 用户主页模板
├── 404.html           # 可选的未找到页面模板
├── <slug>.html        # 可选：某个自定义页面的专属模板
├── i18n/<lang>.json   # 可选：翻译字典
├── seed/<slug>.md     # 可选：默认页面
└── assets/            # 静态文件，通过 /theme-assets/* 提供
```

上传的主题解压到 `data/theme/<id>/`。根目录下的所有模板会解析进同一个集合（因此 `{{define}}` 片段可跨文件共享），由 `html/template` 自动转义渲染，Markdown 正文由 goldmark 以安全模式渲染。解析后的模板按主题缓存，主题文件变化时重新读取。

[主题系统](/zh-cn/concepts/theming) 介绍了完整管线：模板解析及其回退链、语言优先级、缓存、种子页面，以及主题的信任边界。逐字段细节见[主题模板参考](/zh-cn/reference/theme-templates)。

## SSO

登录可以委托给外部身份提供商：

- GitHub 和 Google OAuth
- 任意 OpenID Connect (OIDC) 提供商（Keycloak、Authentik、Authelia、Okta、Casdoor 等）

SSO 流程使用授权码模式 + 弹窗；结果写入 `localStorage` 的 `sso_callback_result` 键，打开方页面通过 `storage` 事件获取。当 `allow_local_login` 为 `false` 时，密码登录被完全禁用，SSO 成为唯一入口。

回调地址：

| 提供商 | 回调地址                                      |
| ------ | --------------------------------------------- |
| GitHub | `https://your-domain/api/sso/github/callback` |
| Google | `https://your-domain/api/sso/google/callback` |
| OIDC   | `https://your-domain/api/sso/oidc/callback`   |

`BASE_URL` 必须指向你的公网实例地址，以便正确生成这些重定向。

## 存储

- 上传文件默认存到本地数据目录，启用 S3 后存到任意 S3 兼容对象存储（AWS S3、MinIO、Garage 等）。
- 两种后端都放在 `internal/storage` 下，共用一个 `Storage` 接口（`Put`/`Open`/`Delete`/`URL`），因此 `upload` 与 `asset` 不需要知道拿到的是哪个后端，测试也可以换成指向临时目录的 `LocalStorage`。具体后端由 `app.initStorage` 选择并注入。
- 对象一律按 key 寻址，绝不使用 URL。`asset` 领域是替代性的上传路径：`Asset` 是面向用户的记录（稳定的 `public_id`、可见性），指向不可变的 `Blob`（存储 key、MIME 类型、大小）。原始内容通过 `/api/assets/{publicID}/original`（`GET`/`HEAD`）提供；只有公开资源可解析，私有资源返回 404。旧版 `/uploads/...` URL 继续可用。旧版 `MediaFile` 记录在已记录 key 时会把存储 key（供 `Delete` 使用）与浏览器 `url` 一并保存。
- 已配置的站点图标是通用设置引用的公开图片 `Asset`。`/favicon.ico` 会重定向到其原始 URL；未配置时依次回退到 `data/favicon.ico`、当前主题的 `favicon.ico` 和管理后台内置图标。
- key 在进入后端前先由 `cleanKey` 归一化并拦截，`LocalStorage` 还用 `os.Root` 约束每一次操作，恶意 key 无法逃出 `data/media`。
- 元数据（用户、文章、评论、设置）存储在数据库：默认 SQLite，生产环境用 PostgreSQL/MySQL。

## 通知

站内通知按用户存储。评论、点赞、回复、文章审核和角色变更等事件都会在接收者的收件箱中创建通知，通过 `/notifications` API 暴露。

通知系统使用接缝接口 `model.Notifier`，各领域调用 `notifier.CreateNotification()` 而不导入 notification 包，具体实现在启动时由组合根注入。

## 数据库

### 连接

`database.Open()` 支持三种后端：

| 后端       | 默认 | 生产可用     |
| ---------- | ---- | ------------ |
| SQLite     | 是   | 适合小型部署 |
| MySQL      | 否   | 是           |
| PostgreSQL | 否   | 是           |

数据库类型由 `db_type` 配置字段或 `DB_TYPE` 环境变量决定。连接 MySQL 时，若数据库不存在会自动创建。

### 迁移与种子数据

`migration.Service.MigrateDB()` 创建或更新所有模型的表结构，映射旧版评论审核开关，回填文章 slug，将旧版 `media_files` 记录转换为 `Asset` + `Blob` 记录，再将旧版 `general_settings.site_icon`、`users.avatar` 与 `posts.cover_image` URL 映射到生成的 asset，随后删除这些列，并将文章与页面内容中的旧版媒体 URL 改写为生成的 asset URL。`database.Seed()` 在记录不存在时插入默认数据（管理员用户、SMTP/通用/AI/主题设置、默认分类）。

站点图标、用户头像和文章封面映射在媒体回填之后运行，因此只能复用该轮回填创建的 asset。非本地 URL 或回填时被跳过的文件会令引用为空；旧列仍会被删除，因此旧 URL 既不会保留，也不会在后续启动中复活。用户头像映射将旧版 URL 与 `media_files.url` 匹配（覆盖本地与 S3 URL），要求媒体文件与 asset 均属于该用户，且仅关联公开的图片 asset。文章封面映射以相同方式匹配，且仅关联公开的图片 asset；它不校验归属，因为旧版列没有记录上传者，且封面可能由管理员设置为其他作者的上传。

内容改写会在上述映射之后运行，且是幂等的：仅当媒体记录生成了公开 asset 时，才会将 `posts.content` 与 `pages.content` 中完全匹配的旧 URL 替换为 asset URL，因此外部链接与被跳过文件的引用保持不变。升级后编辑器上传会直接使用 asset API，新内容从一开始就使用这些 URL。

有三类旧版记录会被有意跳过，每条都会记录其 `id` 与对应的 key 或 URL：

- 嗅探出的类型不是图片、视频或音频的文件——即旧版的 PDF、SVG、`.md`、`.csv` 与 `.txt` 上传，因为 `model.BlobType` 没有对应类型。文章仍以 URL 引用这些文件；设置、用户头像与文章封面不再引用，因为旧版列在映射后会被删除。
- 空文件。
- URL 为绝对地址且没有记录 `StorageKey` 的记录——即尚未记录 key 之前写入的上传，包括旧版 S3 上传。没有 key 就没有可打开或改写的内容。

因其他原因失败的行（对象缺失、文件不可读、数据库错误）会记录为错误，并在下次启动时重试。

最后一条启动日志会报告已迁移行数与已读取行数；差值包含被跳过和失败的行。

## 请求流程

一个典型请求的流程：

```text
浏览器 / API 客户端
      │  HTTP
      ▼
Gin 路由器（internal/router）
      │
      ▼
中间件链：日志 → 可选 JWT 认证 → 角色权限检查
      │
      ▼
领域处理器（如 internal/post/handler.go）
      │  传递 c.Request.Context()
      ▼
领域服务（如 internal/post/service.go）
      │  调用 repository 方法
      ▼
Repository（如 internal/post/repository.go）
      │  GORM 查询，使用 .WithContext(ctx)
      ▼
JSON 响应（主题页面则为 SSR 渲染的 HTML）
```

JWT 中间件验证令牌并通过 `middleware.CurrentUser(c)` / `middleware.CurrentUserID(c)` 辅助函数将用户写入 Gin 上下文。权限中间件用数据库中的角色比对端点要求的角色。`super_admin` 始终通过。

### 性能：批量查询

对于显示每篇文章点赞/评论数的列表端点，post 领域使用批量查询而非 N+1：

```text
// 之前（N+1）：每篇文章 3 次查询
for _, post := range posts {
    repo.CountLikes(ctx, post.ID)       // 查询 1
    repo.CountComments(ctx, post.ID)    // 查询 2
    repo.FindLike(ctx, post.ID, userID) // 查询 3
}

// 之后（批量）：总共 3 次查询
likesCounts    := repo.BatchCountLikesByPostIDs(ctx, postIDs)       // 1 次 GROUP BY 查询
commentsCounts := repo.BatchCountCommentsByPostIDs(ctx, postIDs)    // 1 次 GROUP BY 查询
likedPosts     := repo.BatchFindLikedPostIDs(ctx, postIDs, userID)  // 1 次 IN 查询
```

查询次数从 `3 × N` 降为 3 次，与页面大小无关。

## 测试

每个领域包都有自己的 `_test.go` 文件。测试基础设施：

- 使用内存 SQLite（`glebarez/sqlite`）进行快速、隔离的数据库测试。
- 跨领域依赖改用 fake（`fakeNotifier`、`fakeFiles`），避免测试与其他领域耦合。
- 每个测试创建全新数据库——需要真实表结构时使用 `migration.Service.MigrateDB()`，否则使用针对性的 `AutoMigrate`——并仅注入所需数据。

运行完整测试套件：

```bash
go test ./backend/...
```

查看覆盖率：

```bash
go test -cover ./backend/internal/post/... ./backend/internal/user/... ./backend/internal/comment/... ./backend/internal/notification/...
```

## 相关阅读

- [主题系统](/zh-cn/concepts/theming)：服务端渲染管线与主题系统如何工作
- [配置参考](/zh-cn/reference/configuration)：全部参数、变量和配置键
- [API 参考](api.html)：该架构暴露的 REST 端点
- [配置指南](/zh-cn/guides/configuration)：实操配置方法
