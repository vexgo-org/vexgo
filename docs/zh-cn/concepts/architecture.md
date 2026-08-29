# 架构

> **原理讲解** —— 本页介绍 VexGo 的设计：后端结构、角色与权限的工作方式、内容审核管线、主题系统和 SSO。这是背景知识——用于理解 VexGo，而不是完成某个具体任务。

## 总览

VexGo 是一个自托管的博客 CMS，由两部分组成：

- **Go 后端**（`backend/`）—— 基于 Gin 和 GORM 构建的 HTTP API，提供管理面板、公开站点和 REST API。支持 SQLite、PostgreSQL 或 MySQL。
- **React 前端**（`frontend/`）—— 基于 TypeScript + Vite + Tailwind CSS 的 SPA，与 API 通信。构建产物嵌入后端二进制。

**主题系统**让后端能够以服务端渲染的方式，用上传的主题渲染公开页面——访客无需 JavaScript 即可阅读内容。

## 后端结构

后端采用领域导向的目录结构，位于 `backend/internal` 下，通过组合根进行引导：

```text
backend/
  cmd/vexgo/main.go         # 入口：通过 cli 解析配置，委托给 app 包处理
  internal/
    app/                     # 组合根：组装所有依赖
    auth/                    # 注册、登录、JWT、个人资料、密码重置
    cli/                     # cobra 命令行：参数、帮助、版本、.env 加载
    comment/                 # 评论和 AI 内容审核
    config/                  # 基于 viper 的分层配置解析，JWT、S3、SSO 初始化
    database/                # 连接、自动迁移、种子数据
    home/                    # 站点统计
    mailer/                  # SMTP 邮件构建与发送
    notification/            # 站内通知
    middleware/              # JWT 认证、角色权限、请求日志
    model/                   # GORM 数据模型 + 共享接口（Notifier、FileRemover、Mailer）
    post/                    # 文章 CRUD、分类、标签、点赞
    public/                  # 嵌入的前端、主题、SSR 渲染器、静态路由
    router/                  # 路由注册（组合所有领域）
    secrets/                 # 数据库中敏感信息的 AES-256-GCM 加密
    settings/                # 管理员配置（SMTP、AI、通用、主题）
    sso/                     # GitHub / Google / OIDC 登录
    upload/                  # 文件上传（本地磁盘或 S3）
    user/                    # 用户管理、角色、创作者申请
    verification/            # 邮箱验证和滑块验证码
```

### 分层架构（每个领域）

每个领域包遵循一致的三层模式：

```text
handler.go    → HTTP 请求解析、响应渲染（调用 service）
service.go    → 业务逻辑、跨领域编排（调用 repository）
repository.go → 持久化接口 + GORM 实现（调用数据库）
```

这种分离确保：

- **Handler** 不直接操作 GORM——它们委托给 Service。
- **Service** 在 `Repository` 接口之后与数据库无关，可以使用 fake 进行单元测试。
- **Repository** 封装所有 SQL/GORM 查询，包括批量操作以避免 N+1 问题。

### 共享接口（`model/interfaces.go`）

跨领域的接缝定义在 `model` 包中，作为小型接口：

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

// FileRemover 删除已存储的文件（通过公开 URL）；由 upload.Storage 实现。
type FileRemover interface {
    Delete(url string) error
}

// Mailer 是发送事务性邮件和管理邮箱验证/密码重置令牌的接缝；由 mailer 领域实现。
type Mailer interface {
    IsEmailEnabled() (bool, error)
    GenerateVerificationToken(userID uint) (string, error)
    SendVerificationEmail(toEmail, toName, verificationLink string) error
    VerifyEmail(token string) error
    GenerateEmailChangeToken(userID uint, newEmail string) (string, error)
    SendEmailChangeEmail(toEmail, toName, newEmail, verificationLink string) error
    GeneratePasswordResetToken(userID uint) (string, error)
    SendPasswordResetEmail(toEmail, toName, resetLink string) error
    ConfirmEmailChange(token string) error
}
```

这些接口允许 `post`、`comment` 和 `user` 等领域触发通知和文件清理——`auth`/`verification` 则用于发送邮件——而无需导入具体实现，从而保持依赖图无环。

### 组合根（`internal/app/`）

`internal/app/app.go` 包是组合根（也称"装配层"）。它：

1. 接收 `cli.Execute()` 解析完成的配置——cobra 负责命令行参数，viper 负责按「参数 > 配置文件 > 环境变量 > 默认值」分层。
2. 确保 JWT 密钥存在（开发环境生成随机兜底值）并应用前端地址默认值；SSO 结构体在配置解析阶段派生。
3. 打开数据库并运行迁移/种子数据。
4. 从 `settings_encryption_key` 构建静态加密 cipher（未设置时记录警告，敏感信息保持明文存储），并运行 `database.MigrateSecretsAtRest` 将仍为明文的敏感信息就地加密（幂等）。
5. 创建存储（本地或 S3）。
6. 实例化每个领域的依赖并将它们装配到路由器中。

`cmd/vexgo/main.go` 是一个轻量入口，调用 `cli.Execute()`，然后调用 `app.New(cfg)` 和 `app.Run()`。

### 依赖规则

包结构保证了依赖图**无环**：

```text
cmd/vexgo/main.go
    ├─→ internal/cli          ← cobra 命令树，将参数绑定到 viper
    │       └─→ config        ← 分层解析（参数 > 文件 > 环境变量 > 默认值）
    └─→ internal/app          ← 组合根，导入所有包
            ├─→ config         ← 解析完成的 Config 类型（不导入领域包）
            ├─→ database       ← Open/AutoMigrate/Seed（导入 config、model）
            ├─→ router         ← 路由注册（导入所有领域 handler）
            └─→ internal/*     ← 各领域包

叶子包（无内部导入）：
    model/         ← 数据模型 + 共享接口，被所有领域包导入
    config/        ← 配置解析，被 cli、app、auth、database、sso、upload 导入
    secrets/       ← 静态敏感信息的 AES-256-GCM cipher；各领域通过自己的
                     SecretCipher 接口消费，由 app 装配

共享层：
    middleware/    ← JWT 认证、角色权限、请求日志（仅导入 model）

跨领域边：
    auth/          ← 被 comment、post、sso 使用（隐私过滤）
    mailer/        ← 实现 model.Mailer，被 auth 和 verification 用于发送邮件
    notification/  ← 实现 model.Notifier，被 comment、post、user 作为通知接缝使用
    upload/        ← 实现 model.FileRemover，被 user、auth、post 用于文件清理
    verification/  ← 被 auth 用作验证码检查接缝
    public/        ← 被 settings 用作主题渲染器接缝
```

### 配置管理

配置通过三层优先级链加载：

```text
命令行参数  →  配置文件（YAML）  →  环境变量  →  默认值
  （最高优先级）                              （最低优先级）
```

命令行使用 **cobra** 定义（`internal/cli`），各来源由 **viper** 分层（`internal/config`）。分层顺序的一个直接结果：配置文件中的显式 `false` 会覆盖环境变量中的 `true`——环境变量只填充配置文件未设置的键。

`config.Config` 结构体保存所有运行时值。**不存在全局变量**——JWT 密钥、SSO 配置和前端 URL 是 `Config` 的字段，而非包级别变量。

### context.Context 传播

所有 service 和 repository 方法都以 `context.Context` 作为第一个参数。Handler 从 Gin 请求上下文传递 `c.Request.Context()`，支持：

- 请求作用域的取消和超时
- 分布式追踪传播
- GORM `WithContext()` 查询取消

## 用户、角色与权限

认证基于 JWT。每个用户只有一个角色；每次请求都会根据数据库中的角色检查权限。

| 角色          | 权限                                             |
| ------------- | ------------------------------------------------ |
| `super_admin` | 一切操作。绕过所有权限检查。不能被其他用户修改。 |
| `admin`       | 审核内容、管理用户和设置、审批创作者申请         |
| `author`      | 直接发布文章                                     |
| `contributor` | 申请角色升级（创作者申请）                       |
| `guest`       | 新注册用户——受限访问                             |

权限检查是累积的：

- **Is admin** = `admin` 或 `super_admin`
- **Is author** = `author` 及以上角色
- **Is contributor** = `contributor` 及以上角色

### 创作者申请

新用户注册为 `guest`。他们可以提交**创作者申请**（附理由）请求升级。管理员审核队列，批准或拒绝每份申请；批准后用户升入更高角色层级。

## 内容审核

VexGo 有两条审核管线——文章和评论各一条。两者都围绕 `status` 字段：

- **文章**：`draft` → `pending` → `published` / `rejected`（被拒文章可重新提交）
- **评论**：`published`、`pending`、`rejected`

### 文章审核

作者发布文章时，可直接进入 `published`（若作者有发布权限），或进入 `pending` 等待管理员审核。管理员批准或拒绝，可附加拒绝原因。

### 评论审核

评论审核由三个相互独立的开关驱动，可在管理面板配置（默认全部关闭，此时新评论立即发布）：

- **关键词过滤** —— 包含屏蔽关键词的评论直接拒绝，不再调用大模型
- **大模型审核** —— 由已配置的大模型（OpenAI 兼容 API）按提示词审核每条评论。拒绝结论则评论被拒绝；通过结论仅在人工审核关闭时发布。任何大模型故障（网络、超时、非 200、非 JSON 响应）都会使评论保持 `pending`——整条管道 fail-closed，被误导或故障的模型最多把评论送进待审队列，绝不会发布垃圾内容
- **人工审核** —— 所有未被发布或拒绝的评论在队列中等待管理员裁决（人工终审）

审核配置（开关、提示词、关键词、模型）存储在数据库中，通过管理面板或 `/moderation` API 端点管理。旧版本单一的"启用 AI 审核"开关会在启动时自动迁移为新的开关组合。

## 主题系统

公开页面由服务端渲染。内置的**默认主题**始终可用；管理员可通过管理面板上传 ZIP 格式的主题。

主题包含：

```text
theme.zip
└── theme-id/
    ├── vexgo-theme.json   # 元数据（id、name、author、version 等）
    ├── preview.png        # 可选预览图
    └── dist/              # 构建好的前端资源（index.html、JS、CSS）
```

安装的主题解压到 `data/theme/<id>/`，由渲染器提供。当前主题存储在数据库中，无需重启即可在运行时切换。

## SSO

登录可以委托给外部身份提供商：

- **GitHub** 和 **Google** OAuth
- 任意 **OpenID Connect (OIDC)** 提供商（Keycloak、Authentik、Authelia、Okta、Casdoor 等）

SSO 流程使用授权码模式 + 弹窗；结果写入 `localStorage` 的 `sso_callback_result` 键，打开方页面通过 `storage` 事件获取。当 `allow_local_login` 为 `false` 时，密码登录被完全禁用，SSO 成为唯一入口。

回调地址：

| 提供商 | 回调地址                                      |
| ------ | --------------------------------------------- |
| GitHub | `https://your-domain/api/sso/github/callback` |
| Google | `https://your-domain/api/sso/google/callback` |
| OIDC   | `https://your-domain/api/sso/oidc/callback`   |

`BASE_URL` 必须指向你的公网实例地址，以便正确生成这些重定向。

## 存储

- **上传文件** 默认存到本地数据目录，启用 S3 后存到任意 **S3 兼容对象存储**（AWS S3、MinIO、Garage 等）。
- **元数据**（用户、文章、评论、设置）存储在数据库中——默认 SQLite，生产环境用 PostgreSQL/MySQL。

## 通知

站内通知按用户存储。评论、点赞、回复、文章审核和角色变更等事件都会在接收者的收件箱中创建通知，通过 `/notifications` API 暴露。

通知系统使用**接缝接口**（`model.Notifier`）——各领域调用 `notifier.CreateNotification()` 而不导入 notification 包。具体实现在启动时由组合根注入。

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

`database.AutoMigrate()` 创建或更新所有模型的表结构。`database.Seed()` 在记录不存在时插入默认数据（管理员用户、SMTP/通用/AI/主题设置、默认分类）。

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

JWT 中间件验证令牌并通过 `middleware.CurrentUser(c)` / `middleware.CurrentUserID(c)` 辅助函数将用户写入 Gin 上下文。权限中间件将数据库中的角色与端点要求的角色进行比对。`super_admin` 始终通过。

### 性能：批量查询

对于显示每篇文章点赞/评论数的列表端点，post 领域使用**批量查询**而非 N+1：

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

查询次数从 `3 × N` 降为 **3 次**，无论页面大小。

## 测试

每个领域包都有自己的 `_test.go` 文件。测试基础设施：

- 使用**内存 SQLite**（`glebarez/sqlite`）进行快速、隔离的数据库测试。
- 使用 fake 代替跨领域依赖（`fakeNotifier`、`fakeFiles`），避免测试与其他领域耦合。
- 每个测试使用 `AutoMigrate()` 创建全新数据库，仅注入所需数据。

运行完整测试套件：

```bash
cd backend && go test ./...
```

查看覆盖率：

```bash
cd backend && go test -cover ./internal/post/... ./internal/user/... ./internal/comment/... ./internal/notification/...
```

## 相关阅读

- [配置参考](/zh-cn/reference/configuration) —— 全部参数、变量和配置键
- [API 参考](/zh-cn/reference/api) —— 该架构暴露的 REST 端点
- [配置指南](/zh-cn/guides/configuration) —— 实操配置方法
