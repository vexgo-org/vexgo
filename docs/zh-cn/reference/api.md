# VexGo API 文档

基础 URL：`/api`

认证：JWT Bearer token（公开端点除外）。
所有需要认证的请求都必须携带：`Authorization: Bearer <token>`

角色：`super_admin`、`admin`、`author`、`contributor`、`guest`。标记为「仅管理员」的端点需要 `admin` 或 `super_admin` 角色（`super_admin` 始终通过权限检查）。

---

## 目录

- [公开 API](#公开-api)
- [认证](#认证)
- [SSO（单点登录）](#sso-单点登录)
- [帖子管理](#帖子管理)
- [评论](#评论)
- [点赞](#点赞)
- [文件上传](#文件上传)
- [通知](#通知)
- [审核](#审核)
- [用户管理](#用户管理)
- [创作者申请](#创作者申请)
- [配置](#配置)
- [错误响应](#错误响应)
- [分页](#分页)
- [数据类型](#数据类型)
- [备注](#备注)

---

## 公开 API

### GET /posts

获取支持筛选的分页帖子列表。

**查询参数：**

- `page`（int，默认：1）：页码
- `limit`（int，默认：10，最大：100）：每页条数
- `category`（string）：按分类名称筛选
- `search`（string）：在标题和内容中搜索

**响应：**

```json
{
  "posts": [
    {
      "id": 1,
      "title": "My Post",
      "content": "Post content",
      "excerpt": "",
      "coverImage": "",
      "viewCount": 5,
      "authorId": 1,
      "author": { "id": 1, "username": "admin", "role": "super_admin" },
      "category": "tech",
      "tags": [],
      "status": "published",
      "rejectionReason": "",
      "createdAt": "2026-03-17T21:14:35Z",
      "updatedAt": "2026-03-17T21:14:35Z",
      "likesCount": 0,
      "isLiked": false,
      "commentsCount": 0
    }
  ],
  "pagination": { "total": 1, "page": 1, "limit": 10, "totalPages": 1 }
}
```

**备注：**

- 游客只能看到已发布的帖子（当 `allowGuestViewPosts` 启用时）
- 非管理员用户只能看到其他用户已发布的帖子
- 管理员可以查看除已拒绝外的所有帖子

---

### GET /posts/:slug

按 URL 别名获取单个帖子。

**响应：**

```json
{
  "post": {
    "id": 1,
    "title": "My Post",
    "content": "Post content",
    "category": "tech",
    "tags": [],
    "status": "published",
    "author": { "id": 1, "username": "admin", "role": "super_admin" },
    "likesCount": 0,
    "isLiked": false,
    "commentsCount": 0,
    "createdAt": "2026-03-17T21:14:35Z",
    "updatedAt": "2026-03-17T21:14:35Z"
  }
}
```

**错误响应：**

- `403`：`{"error": "You must be logged in to view this post"}`（游客浏览被禁用）
- `404`：`{"error": "Post does not exist", "postId": "<id>"}`

---

---

### GET /posts/by-id/:id

按数字 ID 获取单个帖子。适用于调用方持有 ID 但没有别名的场景（例如通知数据）。

**响应：** 与 `GET /posts/:slug` 格式相同。

**错误响应：**

- `403`：`{"error": "You must be logged in to view this post"}`（游客浏览被禁用）
- `404`：`{"error": "Post does not exist", "slug": "<slug>"}`

---

### GET /categories

获取所有分类。每一项都带有 `postCount`，即 `category` 字段等于该分类名称的文章数量。

**响应：**

```json
{
  "categories": [
    {
      "id": 1,
      "name": "Default",
      "description": "Default category",
      "postCount": 12
    },
    { "id": 2, "name": "tech", "description": "", "postCount": 0 }
  ]
}
```

**备注：**

- 仅当 `allowGuestViewPosts` 启用时游客才能查看分类
- 如果游客浏览被禁用且用户未登录，返回空数组

---

### GET /tags

获取所有标签。每一项都带有 `postCount`，即引用该标签的文章数量。

**响应：**

```json
{
  "tags": [{ "id": 1, "name": "golang", "postCount": 3 }]
}
```

与 `/categories` 相同的游客可见性规则。

---

### POST /categories

创建新分类。需要登录，且调用者必须是贡献者及以上角色（`contributor`、`author`、`admin`、`super_admin`）。`guest` 角色与未登录请求将被拒绝。

**请求：**

```json
{
  "name": "tech",
  "description": "技术相关文章"
}
```

`name` 为必填；`description` 为选填。分类名称必须唯一。

**错误响应：**

- `401`：`{"error": "No authentication information provided"}`（缺少或无效的令牌）
- `403`：`{"error": "Insufficient permissions to create a category"}`（角色为 guest 或权限不足）
- `409`：`{"error": "A category with this name already exists", "code": "duplicate_name"}`（名称重复）
- `400`：`{"error": "..."}`（`name` 缺失或无效）

**响应（201）：**

```json
{
  "message": "Category created successfully",
  "category": { "id": 3, "name": "tech", "description": "技术相关文章" }
}
```

---

### POST /tags

创建新标签。需要登录，且调用者必须是贡献者及以上角色（`contributor`、`author`、`admin`、`super_admin`）。`guest` 角色与未登录请求将被拒绝。

**请求：**

```json
{
  "name": "golang"
}
```

`name` 为必填，且唯一标识一个标签。

**错误响应：**

- `401`：`{"error": "No authentication information provided"}`（缺少或无效的令牌）
- `403`：`{"error": "Insufficient permissions to create a tag"}`（角色为 guest 或权限不足）
- `409`：`{"error": "A tag with this name already exists", "code": "duplicate_name"}`（名称重复）
- `400`：`{"error": "..."}`（`name` 缺失或无效）

**响应（201）：**

```json
{
  "message": "Tag created successfully",
  "tag": { "id": 4, "name": "golang" }
}
```

---

### DELETE /categories/:id

删除分类。需要登录，且调用者必须是贡献者及以上角色（`contributor`、`author`、`admin`、`super_admin`）。`guest` 角色与未登录请求将被拒绝。

仅当分类为**空**时才能删除——即没有任何文章的 `category` 字段等于该分类名称。仍在使用中的分类不会被强制删除或解除关联。

**响应（200）：**

```json
{
  "message": "Category deleted successfully"
}
```

**错误响应：**

- `401`：`{"error": "No authentication information provided"}`（缺少或无效的令牌）
- `403`：`{"error": "Insufficient permissions to delete a category"}`（角色为 guest 或权限不足）
- `400`：`{"error": "Category is used by N posts"}`（分类仍被文章引用）
- `404`：`{"error": "Category does not exist"}`（`id` 不存在、为零或非数字）

---

### DELETE /tags/:id

删除标签。需要登录，且调用者必须是贡献者及以上角色（`contributor`、`author`、`admin`、`super_admin`）。`guest` 角色与未登录请求将被拒绝。

仅当标签为**空**时才能删除——即没有任何文章通过 `post_tags` 关联引用该标签。仍在使用中的标签不会被强制删除或解除关联。

**响应（200）：**

```json
{
  "message": "Tag deleted successfully"
}
```

**错误响应：**

- `401`：`{"error": "No authentication information provided"}`（缺少或无效的令牌）
- `403`：`{"error": "Insufficient permissions to delete a tag"}`（角色为 guest 或权限不足）
- `400`：`{"error": "Tag is used by N posts"}`（标签仍被文章引用）
- `404`：`{"error": "Tag does not exist"}`（`id` 不存在、为零或非数字）

---

### GET /stats

获取站点聚合统计。

**响应：**

```json
{
  "stats": {
    "posts": 100,
    "users": 50,
    "comments": 200,
    "categories": 10,
    "tags": 30
  }
}
```

---

### GET /stats/popular-posts

获取最热门的帖子（按点赞数和浏览量排序）。

**查询参数：**

- `limit`（int，默认：5）：返回的帖子数量

**响应：**

```json
{ "posts": [...] }
```

---

### GET /stats/latest-posts

按创建时间获取最新帖子。

**查询参数：**

- `limit`（int，默认：5）：返回的帖子数量

**响应：**

```json
{ "posts": [...] }
```

---

### GET /themes

获取所有可用主题。

**响应：**

```json
{
  "themes": [
    {
      "id": "default",
      "name": "vexgo default theme",
      "author": "vexgo",
      "version": "1.0.0",
      "description": "vexgo default theme",
      "url": "https://github.com/vexgo-org/vexgo"
    }
  ]
}
```

内置默认主题始终返回。安装在数据目录（`data/theme/<id>/vexgo-theme.json`）下的其他主题会被追加。

---

### GET /theme/:id/preview

获取指定主题的预览图。

**响应：** 图片文件（PNG/JPG）

**错误响应：**

- `404`：`{"error": "theme not found"}` / `{"error": "preview not specified"}` / `{"error": "preview image not found"}`

---

### GET /comments/post/:id

获取帖子的已发布评论（可选登录，应用作者隐私过滤）。

**响应：**

```json
{
  "comments": [
    {
      "id": 1,
      "postId": 1,
      "userId": 2,
      "user": { "id": 2, "username": "user1" },
      "content": "Comment text",
      "status": "published",
      "parentId": null,
      "createdAt": "2026-03-17T21:14:35Z",
      "updatedAt": "2026-03-17T21:14:35Z"
    }
  ]
}
```

---

### GET /likes/:postId

获取帖子的点赞状态和数量（可选登录）。

**响应：**

```json
{ "postId": 1, "likesCount": 42, "isLiked": false }
```

---

### GET /posts/user/:id

获取指定用户已发布的帖子。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10）

**响应：**

```json
{
  "posts": [...],
  "pagination": { "total": 20, "page": 1, "limit": 10, "totalPages": 2 }
}
```

---

### GET /verify-email

使用 token 验证邮箱地址。

**查询参数：**

- `token`（string，必填）：验证 token

**响应（初始验证）：**

```json
{ "message": "Email verification successful! You can now log in." }
```

**响应（更改邮箱 - 找到用户）：**

```json
{
  "message": "Email change successful! Your new email is now active.",
  "require_relogin": true,
  "new_email": "newemail@example.com"
}
```

**响应（更改邮箱 - 未找到用户）：**

```json
{
  "message": "Email change successful! Your new email is now active.",
  "require_relogin": true
}
```

**备注：**

- 以 `email-change-` 为前缀的 token 验证邮箱更改；其他 token 验证初始邮箱
- 更改邮箱后用户必须重新登录（`require_relogin: true`）

---

### GET /captcha

生成滑块拼图验证码。

**响应：**

```json
{
  "id": "uuid",
  "token": "captcha_token",
  "thumbX": 25,
  "thumbY": 80,
  "thumbWidth": 60,
  "thumbHeight": 60,
  "image": "data:image/jpeg;base64,...",
  "thumb": "data:image/png;base64,...",
  "expires_at": "2026-03-17T21:14:35Z"
}
```

**备注：**

- `image` 是主图，`thumb` 是拼图块；两者都是可直接渲染的 data-URI base64 字符串
- `thumbX`/`thumbY`/`thumbWidth`/`thumbHeight` 描述拼图块在主图中的初始展示位置和尺寸；客户端需要将拼图块拖到对应的缺口处并提交拖放坐标
- 正确的拖放位置不会暴露给客户端
- 验证码只能验证一次，5 分钟后过期；任何一次验证失败同样会使验证码作废，客户端需要重新获取
- 两个验证码接口都按客户端 IP 限流（默认每分钟 30 次，可通过 `captcha_rate_limit_per_minute` 配置）；超限的客户端会收到 `429`

---

### POST /captcha/verify

验证验证码 token 和拖放位置。

**请求：**

```json
{ "id": "uuid", "token": "token", "x": 150, "y": 80 }
```

**响应（成功）：**

```json
{ "success": true, "message": "Verification successful" }
```

**错误响应：**

- `404`：`{"error": "Captcha does not exist or has expired"}`
- `400`：`{"error": "Captcha already used"}`
- `400`：`{"error": "Captcha has expired"}`
- `400`：`{"error": "Verification failed, please try again"}`

`x` 和 `y` 两个坐标都会与存储的答案比对，每个坐标允许 ±5 像素容差。验证失败一次后该验证码即作废——请重新获取新的验证码再试。

---

## 认证

### POST /auth/register

注册新用户账户。

**请求：**

```json
{
  "email": "user@example.com",
  "password": "password123",
  "username": "username",
  "captcha_id": "uuid",
  "captcha_token": "token",
  "captcha_x": 150,
  "captcha_y": 80
}
```

**响应（201）：**

```json
{
  "message": "Registration successful! Please verify your email address before logging in. Check your inbox and click the verification link.",
  "user": {
    "id": 1,
    "username": "username",
    "email": "user@example.com",
    "role": "guest"
  },
  "email_verified": false,
  "requires_verification": true
}
```

**错误响应：**

- `409`：`{"error": "user already exists"}`
- `403`：`{"error": "registration is disabled, please contact administrator"}`
- `400`：需要验证码 / 验证码过期 / 验证码不匹配

**备注：**

- 当通用设置中启用了 `captchaEnabled` 时，验证码字段为必填
- 当验证码被禁用时，`captcha_id`、`captcha_token`、`captcha_x` 和 `captcha_y` 可以是空字符串 / 0

---

### POST /auth/login

使用邮箱和密码登录。

**请求：**

```json
{
  "email": "user@example.com",
  "password": "password123",
  "captcha_id": "uuid",
  "captcha_token": "token",
  "captcha_x": 150,
  "captcha_y": 80
}
```

**响应（200）：**

```json
{
  "token": "jwt_token_here",
  "user": {
    "id": 1,
    "username": "username",
    "email": "user@example.com",
    "role": "admin",
    "avatar": "",
    "bio": "",
    "birthday": ""
  }
}
```

**错误响应：**

- `401`：`{"message": "invalid email or password"}`
- `403`：`{"message": "Please verify your email address first. ...", "email_verified": false}`
- `400`：`{"error": "please complete the captcha verification"}`

---

### GET /auth/me

获取当前用户信息（需要认证）。

**响应：**

```json
{
  "user": {
    "id": 1,
    "email": "user@example.com",
    "username": "username",
    "role": "admin",
    "email_verified": true,
    "avatar": "url_to_avatar",
    "bio": "My bio",
    "birthday": "2023-01-01",
    "createdAt": "2026-03-17T21:14:35Z",
    "profile_visibility": "public",
    "hide_email": false,
    "hide_birthday": false,
    "hide_bio": false
  }
}
```

---

### GET /auth/user

`/auth/me` 的别名（需要认证）。

---

### PUT /auth/profile

更新当前用户的个人资料（需要认证）。所有字段均为可选。

**请求：**

```json
{
  "username": "new_username",
  "bio": "My bio",
  "avatar": "url_to_avatar",
  "birthday": "2023-01-01"
}
```

**响应：**

```json
{
  "user": {
    "id": 1,
    "username": "new_username",
    "bio": "My bio",
    "avatar": "url_to_avatar",
    "birthday": "2023-01-01"
  }
}
```

---

### PUT /auth/password

更改当前用户的密码（需要认证）。

**请求：**

```json
{ "oldPassword": "old_password", "newPassword": "new_password" }
```

**响应：**

```json
{ "message": "Password changed successfully" }
```

`newPassword` 至少需要 6 个字符。更改密码会使之前签发的 token 失效。

---

### PUT /auth/email

请求更改邮箱（需要认证）。验证邮件会发送到新地址。

**请求：**

```json
{ "email": "newemail@example.com" }
```

**响应：**

```json
{
  "message": "Verification email sent. Please check your inbox and click the link to complete email change.",
  "pending": true
}
```

---

### PUT /auth/settings

更新当前用户的隐私设置（需要认证）。

**请求：**

```json
{
  "profile_visibility": "public",
  "hide_email": true,
  "hide_birthday": false,
  "hide_bio": false
}
```

**响应：**

```json
{
  "message": "Settings updated successfully",
  "user": {
    "id": 1,
    "profile_visibility": "public",
    "hide_email": true,
    "hide_birthday": false,
    "hide_bio": false
  }
}
```

---

### POST /auth/request-password-reset

请求密码重置邮件（公开）。

**请求：**

```json
{ "email": "user@example.com" }
```

**响应：**

```json
{ "message": "If the email exists, reset link has been sent" }
```

---

### POST /auth/reset-password

使用邮件中的 token 重置密码（公开）。

**请求：**

```json
{ "token": "reset_token", "password": "new_password" }
```

**响应：**

```json
{ "message": "Password reset successfully" }
```

`password` 至少需要 6 个字符。

---

### GET /auth/verification-status

获取当前用户的邮箱验证状态（需要认证）。

**响应：**

```json
{ "email_verified": true, "email": "user@example.com" }
```

---

## SSO（单点登录）

### GET /sso/providers

获取已启用的 SSO 提供商（公开）。

**响应：**

```json
{ "providers": ["github", "google"], "allow_local_login": true }
```

`providers` 只包含已启用的提供商（支持 GitHub、Google 和 OIDC）。当密码登录被禁用时，`allow_local_login` 为 `false`。

---

### GET /sso/:provider/login

发起 SSO 登录（重定向到提供商）。

**查询参数：**

- `method`（string，可选，默认：`sso_get_token`）：
  - `sso_get_token` — 完整登录；回调时签发 JWT
  - `get_sso_id` — 只返回提供商侧的 ID（用于在设置页将 SSO 绑定到已有账户）

**响应：** `302` 重定向到 OAuth2/OIDC 提供商。

---

### GET /sso/:provider/callback

SSO 回调端点；提供商重定向到这里。

**查询参数：**

- `code`（string）：授权码
- `state`（string）：CSRF state 参数
- `method`（string）：与登录请求中的值相同

**响应：** 一个 HTML 页面（不是 JSON）。结果写入 `localStorage` 的 `sso_callback_result` 键下，然后弹窗关闭。打开者页面通过 `storage` 事件读取结果：

- 成功时，payload 是一个 JSON 字符串，例如 `{"token": "<jwt>"}`（针对 `sso_get_token`）或 `{"sso_id": "<provider user id>"}`（针对 `get_sso_id`）
- 出错时，payload 是 `{"error": "<message>"}`，响应状态为 `400`

---

## 帖子管理

### POST /posts

创建新帖子（需要认证）。

**请求：**

```json
{
  "slug": "my-post",
  "title": "My Post",
  "content": "Post content in markdown...",
  "category": "tech",
  "tags": ["golang", "programming"],
  "excerpt": "Post excerpt",
  "coverImage": "/uploads/image.jpg",
  "status": "draft"
}
```

`slug`、`title`、`content` 和 `category` 为必填。`slug` 必须是 URL 安全的字符串（小写字母/数字，连字符分隔，不能以连字符开头或结尾，不能有连续连字符）。`category` 接受分类名称或 ID；`status` 是 `draft` / `pending` / `published` 之一。

**错误响应：**

- `400`：别名格式无效或为空
- `409`：`{"error": "Slug is already taken by another post", "code": "slug_taken"}`（别名已被占用）

**响应（201）：**

```json
{
  "message": "Post created successfully",
  "post": { "id": 1, "slug": "my-post", "title": "My Post", "status": "draft" }
}
```

---

### GET /posts/user/my-posts

获取当前用户的帖子（需要认证）。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10）
- `status`（string，可选）：按帖子状态筛选

**响应：**

```json
{ "posts": [...], "pagination": { "total": 10, "page": 1, "limit": 10, "totalPages": 1 } }
```

---

### GET /posts/drafts

获取当前用户的草稿帖子（需要认证）。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10）

**响应：**

```json
{ "posts": [...], "pagination": { "total": 3, "page": 1, "limit": 10, "totalPages": 1 } }
```

---

### PUT /posts/:id

更新帖子（需要认证；仅作者或管理员）。

**请求：** 与 `POST /posts` 相同的字段；全部可选。

**响应：**

```json
{
  "message": "Post updated successfully",
  "post": {
    "id": 1,
    "slug": "my-post",
    "title": "Updated Title",
    "status": "published"
  }
}
```

**错误响应：**

- `400`：别名格式无效
- `403`：`{"error": "Not authorized to modify this post"}`
- `404`：`{"error": "Post does not exist"}`
- `409`：`{"error": "Slug is already taken by another post", "code": "slug_taken"}`

---

### DELETE /posts/:id

删除帖子（需要认证；仅作者或管理员）。

**响应：**

```json
{ "message": "Post deleted successfully" }
```

---

## 评论

### POST /comments

创建评论（需要认证）。

**请求：**

```json
{ "postId": 1, "content": "This is a comment", "parentId": null }
```

`postId` 接受数字或数字字符串；`content` 限制为 100 个字符；`parentId` 可选，用于回复评论。

**响应（201）：**

```json
{
  "message": "Comment created successfully",
  "comment": {
    "id": 1,
    "postId": 1,
    "userId": 1,
    "content": "This is a comment",
    "status": "published",
    "parentId": null,
    "createdAt": "2026-03-17T21:14:35Z"
  },
  "commentsCount": 1,
  "requiresModeration": false
}
```

当评论以 `pending` 状态创建（启用了审核）时，`requiresModeration` 为 `true`。

---

### DELETE /comments/:id

删除评论（需要认证；仅评论作者或管理员）。

**响应：**

```json
{ "message": "Comment deleted", "commentsCount": 1 }
```

---

## 点赞

### POST /likes/:postId

切换帖子的点赞状态（需要认证）。

**响应：**

```json
{
  "message": "Liked successfully",
  "postId": 1,
  "isLiked": true,
  "likesCount": 42
}
```

再次调用会取消点赞：

```json
{ "message": "Like removed", "postId": 1, "isLiked": false, "likesCount": 41 }
```

---

## 文件上传

### POST /upload/file

上传单个文件（需要认证）。

**表单数据：**

- `file`（file）：要上传的文件

**响应：**

```json
{
  "message": "File uploaded successfully",
  "file": {
    "id": 1,
    "url": "/uploads/uuid.ext",
    "size": 1024,
    "type": "image/jpeg",
    "userId": 1,
    "createdAt": "2026-03-17T21:14:35Z"
  }
}
```

文件以 UUID 文件名存储。存储根据配置使用本地磁盘或 S3。

---

### POST /upload/files

上传多个文件（需要认证）。

**表单数据：**

- `files`（file[]）：多个文件

**响应：**

```json
{
  "message": "File upload completed",
  "files": [
    { "id": 1, "url": "/uploads/uuid1.ext", "size": 1024, "userId": 1 },
    { "id": 2, "url": "/uploads/uuid2.ext", "size": 2048, "userId": 1 }
  ]
}
```

---

### GET /upload/my-files

获取当前用户上传的文件（需要认证）。

**响应：**

```json
{
  "files": [
    {
      "id": 1,
      "url": "/uploads/uuid.ext",
      "size": 1024,
      "type": "image/jpeg",
      "userId": 1,
      "createdAt": "2026-03-17T21:14:35Z"
    }
  ]
}
```

---

### DELETE /upload/:id

删除上传的文件（需要认证；仅上传者或管理员）。

**响应：**

```json
{ "message": "File deleted" }
```

---

## 通知

### GET /notifications

获取当前用户的通知（需要认证）。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10）
- `type`（string，可选）：按通知类型筛选（`comment`、`like`、`reply`、`review`、`role`）
- `is_read`（string，可选）：按已读状态筛选（`true` 或 `false`）

**响应：**

```json
{
  "notifications": [
    {
      "id": 1,
      "user_id": 1,
      "type": "comment",
      "title": "Post Commented",
      "content": "User \"alice\" commented on your post \"Hello\": ...",
      "related_id": "1",
      "related_type": "post",
      "is_read": false,
      "created_at": "2026-03-17T21:14:35Z",
      "updated_at": "2026-03-17T21:14:35Z"
    }
  ],
  "pagination": { "total": 20, "page": 1, "limit": 10, "totalPages": 2 }
}
```

---

### GET /notifications/unread-count

获取当前用户的未读通知数（需要认证）。

**响应：**

```json
{ "unreadCount": 5 }
```

---

### PUT /notifications/:id/read

将通知标记为已读（需要认证）。

**响应：**

```json
{ "message": "Notification marked as read" }
```

---

### PUT /notifications/read-all

将所有通知标记为已读（需要认证）。

**响应：**

```json
{ "message": "All notifications marked as read" }
```

---

### DELETE /notifications/:id

删除通知（需要认证；仅通知所有者）。

**响应：**

```json
{ "message": "Notification deleted" }
```

---

## 审核

所有审核端点都需要 `admin` 或 `super_admin` 角色。

### 评论审核

#### GET /moderation/comments/pending

获取待审核的评论。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10）

**响应：**

```json
{
  "comments": [
    {
      "id": 1,
      "content": "Comment to moderate",
      "user": { "id": 2, "username": "user1" },
      "post": { "id": 1, "title": "My Post" },
      "status": "pending",
      "createdAt": "2026-03-17T21:14:35Z"
    }
  ],
  "pagination": { "total": 1, "page": 1, "limit": 10, "totalPages": 1 }
}
```

#### GET /moderation/comments/approved

获取已批准（`published`）的评论。参数和结构与上面相同。

#### GET /moderation/comments/rejected

获取已拒绝的评论。参数和结构与上面相同。

#### PUT /moderation/comments/approve/:id

批准评论（状态 → `published`）。

**响应：**

```json
{ "message": "Comment approved", "comment": { "id": 1, "status": "published" } }
```

#### PUT /moderation/comments/reject/:id

拒绝评论（状态 → `rejected`）。无请求体。

**响应：**

```json
{ "message": "Comment rejected", "comment": { "id": 1, "status": "rejected" } }
```

#### GET /moderation/comments/config

获取评论审核配置（API 密钥已掩码）。

**响应：**

```json
{
  "manualReviewEnabled": false,
  "keywordFilterEnabled": false,
  "llmReviewEnabled": false,
  "modelProvider": "",
  "apiKey": "",
  "apiEndpoint": "",
  "modelName": "gpt-3.5-turbo",
  "moderationPrompt": "You are a comment moderation assistant. ...",
  "blockKeywords": "spam,advertisement"
}
```

**备注：**

- 三个开关相互独立，默认均为 `false`；全部关闭时，新评论立即发布。
- 审核按固定顺序执行，并在第一个决策处短路：
  1. **关键词过滤**（开启时）：包含屏蔽关键词的评论被拒绝，不再调用 LLM。
  2. **大模型审核**（开启时）：由已配置的 OpenAI 兼容端点审核评论。拒绝结论则评论被拒绝；通过结论在人工审核开启时进入待审队列，否则发布。**任何 LLM 故障（网络、超时、非 200、非 JSON 响应）都会使评论保持 `pending`，即使人工审核未开启**（fail-closed）。
  3. **人工审核**（开启时）：评论保持 `pending`。
  4. 否则评论直接发布。
- 在没有已存储（或本次提供）的 API 密钥与端点时开启 `llmReviewEnabled`，请求将以 `400` 被拒绝。

#### PUT /moderation/comments/config

更新评论审核配置。

**请求：** 与 GET 响应相同的字段，外加 `apiKey`（留空以保留现有密钥）。

**响应：**

```json
{
  "message": "Comment moderation configuration updated successfully",
  "config": {
    "manualReviewEnabled": true,
    "keywordFilterEnabled": true,
    "llmReviewEnabled": true,
    "modelProvider": "openai",
    "modelName": "gpt-3.5-turbo"
  }
}
```

#### POST /moderation/comments/config/test

向已保存的大模型审核配置发送一条测试评论，验证连通性。需要已配置大模型凭据（API 密钥、端点、模型名称）。

**响应：**

```json
{
  "message": "LLM moderation endpoint reachable",
  "response": "model gpt-3.5-turbo replied: approved=true, reason=ok"
}
```

**错误：** 配置不完整时返回 `400`；端点不可达或响应异常时返回 `500` 并附带上游错误信息。

### 帖子审核

#### GET /moderation/pending

获取状态为 `pending` 的帖子。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10）
- `search`（string，可选）

**响应：**

```json
{ "posts": [...], "pagination": { "total": 2, "page": 1, "limit": 10, "totalPages": 1 } }
```

#### GET /moderation/approved

获取状态为 `published` 的帖子。参数和结构与上面相同。

#### GET /moderation/rejected

获取状态为 `rejected` 的帖子。参数和结构与上面相同。

#### PUT /moderation/approve/:id

批准帖子（状态 → `published`）。

**响应：**

```json
{ "message": "Post approved", "post": { "id": 1, "status": "published" } }
```

#### PUT /moderation/reject/:id

拒绝帖子（状态 → `rejected`）。

**请求：**

```json
{ "rejectionReason": "Reason for rejection" }
```

**响应：**

```json
{
  "message": "Post has been rejected",
  "post": { "id": 1, "status": "rejected" }
}
```

#### PUT /moderation/resubmit/:id

重新提交被拒绝的帖子（状态 → `pending`）。

**响应：**

```json
{
  "message": "Post resubmitted for moderation",
  "post": { "id": 1, "status": "pending" }
}
```

只有被拒绝的帖子才能重新提交。

---

## 用户管理

本节所有端点都需要 `admin` 或 `super_admin` 角色。

### GET /users

获取分页用户列表。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10，最大：100）
- `search`（string，可选）：按用户名或邮箱搜索

**响应：**

```json
{
  "users": [
    {
      "id": 1,
      "username": "user1",
      "email": "user@example.com",
      "role": "admin",
      "email_verified": true,
      "createdAt": "2026-03-17T21:14:35Z"
    }
  ],
  "pagination": { "total": 1, "page": 1, "limit": 10, "totalPages": 1 }
}
```

### PUT /users/:id/role

更新用户角色。

**请求：**

```json
{ "role": "admin" }
```

有效角色：`super_admin`、`admin`、`author`、`contributor`、`guest`。

**响应：**

```json
{
  "message": "User role updated successfully",
  "user": { "id": 2, "role": "admin" }
}
```

**备注：**

- 用户不能修改自己的角色
- 只有 `super_admin` 才能修改 `super_admin` 账户

### DELETE /users/:id

删除用户及其所有帖子和评论。

**响应：**

```json
{ "message": "User deleted successfully" }
```

**备注：**

- 用户不能删除自己的账户
- 非 `super_admin` 不能删除 `super_admin` 账户

---

## 创作者申请

### POST /users/apply-creator

提交创作者申请（需要认证）。

只有 `guest` 或 `contributor` 角色的用户可以申请角色升级。

**请求：**

```json
{ "reason": "I want to publish posts" }
```

**响应：**

```json
{ "message": "Application submitted successfully", "applicationId": 1 }
```

**错误响应：**

- `400`：`{"error": "only guest and contributor users can apply for role upgrade"}`
- `400`：`{"error": "..."}`（已有待处理的申请）

### GET /users/creator-applications

获取待审核的创作者申请（仅管理员）。

**查询参数：**

- `page`（int，默认：1）
- `limit`（int，默认：10，最大：100）
- `status`（string，默认：`pending`）：`pending`、`approved`、`rejected`

**响应：**

```json
{
  "applications": [
    {
      "id": 1,
      "userId": 2,
      "username": "user1",
      "email": "user@example.com",
      "currentRole": "contributor",
      "status": "pending",
      "reason": "I want to publish posts",
      "createdAt": "2026-03-17T21:14:35Z",
      "updatedAt": "2026-03-17T21:14:35Z"
    }
  ],
  "pagination": { "total": 1, "page": 1, "limit": 10, "totalPages": 1 }
}
```

### PUT /users/creator-applications/:id/review

审核创作者申请（仅管理员）。

**请求：**

```json
{ "action": "approve", "reason": "Optional review note" }
```

`action` 为 `approve` 或 `reject`。

**响应：**

```json
{ "message": "Application reviewed successfully" }
```

---

## 配置

本节所有端点都需要 `admin` 或 `super_admin` 角色，`GET /config/general` 和 `GET /config/theme` 除外，它们是公开的。

### SMTP 配置

#### GET /config/smtp

获取 SMTP 配置（不返回密码）。

**响应：**

```json
{
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "user@example.com",
  "password": "",
  "fromEmail": "noreply@example.com",
  "fromName": "VexGo",
  "testEmail": ""
}
```

#### PUT /config/smtp

更新 SMTP 配置。返回更新后的配置。

**请求：**

```json
{
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "user@example.com",
  "password": "password",
  "fromEmail": "noreply@example.com",
  "fromName": "VexGo",
  "testEmail": "test@example.com"
}
```

`password` 留空以保留现有密码。

**响应：** 更新后的 SMTP 配置（与 `GET /config/smtp` 结构相同）。

#### POST /config/smtp/test

发送测试邮件。无请求体 — 收件人是配置的 `testEmail`，回退到操作管理员自己的邮箱。

**响应：**

```json
{
  "message": "Test email has been sent to your inbox",
  "to": "test@example.com"
}
```

### AI 配置

#### GET /config/ai

获取 AI 配置（不返回 API 密钥）。

**响应：**

```json
{
  "enabled": false,
  "provider": "openai",
  "apiEndpoint": "",
  "apiKey": "",
  "modelName": "gpt-3.5-turbo"
}
```

#### PUT /config/ai

更新 AI 配置。

**请求：**

```json
{
  "enabled": true,
  "provider": "openai",
  "apiEndpoint": "https://api.openai.com/v1",
  "apiKey": "sk-...",
  "modelName": "gpt-3.5-turbo"
}
```

`apiKey` 留空以保留现有密钥。

**响应：**

```json
{
  "message": "AI config updated successfully",
  "aiConfig": {
    "enabled": true,
    "provider": "openai",
    "apiEndpoint": "https://api.openai.com/v1",
    "apiKey": "",
    "modelName": "gpt-3.5-turbo"
  }
}
```

#### POST /config/ai/test

测试 AI 连接。无请求体；使用内置测试提示词。

**响应：**

```json
{ "message": "AI connection test successful!", "response": "This is a test." }
```

#### GET /config/ai/models

列出配置的 AI 端点可用的模型。

**响应：**

```json
{
  "message": "Models fetched successfully",
  "models": [
    {
      "id": "gpt-3.5-turbo",
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    }
  ]
}
```

### 通用设置

#### GET /config/general

获取通用设置（公开访问）。

**响应：**

```json
{
  "captchaEnabled": false,
  "registrationEnabled": true,
  "allowGuestViewPosts": true,
  "siteName": "VexGo",
  "siteDescription": "",
  "siteIcon": "",
  "itemsPerPage": 20
}
```

#### PUT /config/general

更新通用设置（仅管理员）。

**请求：** 与 GET 响应相同的字段。

**响应：**

```json
{
  "message": "General settings updated successfully",
  "generalSettings": {
    "siteName": "VexGo",
    "registrationEnabled": true,
    "allowGuestViewPosts": true,
    "captchaEnabled": false,
    "itemsPerPage": 20
  }
}
```

### 主题配置

#### GET /config/theme

获取当前启用的主题（公开访问）。

**响应：**

```json
{ "activeTheme": "default" }
```

#### PUT /config/theme

设置全局启用的主题（仅管理员）。

**请求：**

```json
{ "activeTheme": "theme_id" }
```

**响应：**

```json
{ "message": "Theme updated successfully", "activeTheme": "theme_id" }
```

#### POST /themes/upload

上传并安装新主题（仅管理员）。

**表单数据：**

- `theme`（file）：包含主题文件和 `vexgo-theme.json` 元数据文件的 ZIP 压缩包

**响应：**

```json
{ "message": "Theme uploaded successfully" }
```

**主题结构：**

```text
theme.zip
└── theme-id/
    ├── vexgo-theme.json (metadata, required)
    ├── preview.png (optional preview image)
    └── dist/ (built frontend assets, index.html, ...)
```

**vexgo-theme.json：**

```json
{
  "id": "theme-id",
  "name": "Theme Name",
  "description": "Theme description",
  "author": "Author Name",
  "version": "1.0.0",
  "preview": "preview.png"
}
```

安装的主题会解压到 `data/theme/<id>/` 并由渲染器提供服务；内置默认主题（`default`）始终可用。

---

## 错误响应

所有端点都可能返回以下错误响应：

**400 Bad Request：**

```json
{ "error": "Invalid request parameters" }
```

**401 Unauthorized：**

```json
{ "error": "Unauthorized" }
```

**403 Forbidden：**

```json
{ "error": "Insufficient permissions" }
```

**404 Not Found：**

```json
{ "error": "Resource not found" }
```

**409 Conflict：**

```json
{ "error": "Resource already exists" }
```

**500 Internal Server Error：**

```json
{ "error": "Internal server error" }
```

注意：部分端点在认证相关失败时返回 `{"message": ...}` 而不是 `{"error": ...}` — 参见各端点章节。

---

## 分页

分页端点接受 `page` 和 `limit` 查询参数并返回：

```json
{
  "data": [...],
  "pagination": { "total": 100, "page": 1, "limit": 10, "totalPages": 10 }
}
```

实际列表键因端点而异（`posts`、`comments`、`users`、`notifications`、`applications`）。

---

## 数据类型

### 用户角色类型

- `super_admin`：超级管理员（最高权限，绕过所有权限检查）
- `admin`：管理员
- `author`：作者（可以发布帖子）
- `contributor`：贡献者（可以申请角色升级）
- `guest`：游客 / 新注册用户（访问受限）

### 帖子状态

- `draft`：草稿（仅作者可见）
- `pending`：待审核
- `published`：已发布并可见
- `rejected`：被管理员拒绝

### 评论状态

- `published`：已批准并可见
- `pending`：待审核（启用审核时）
- `rejected`：被管理员拒绝

---

## 备注

1. 所有时间戳均为 RFC 3339 格式（带时区的 ISO 8601）
2. 认证中间件验证 JWT token，并在 Gin 上下文中设置 `userID` 和 `user`
3. 权限中间件根据用户角色（来自数据库）检查所需角色；`super_admin` 始终通过
4. 根据查看者的角色和目标用户的隐私设置，对用户数据应用隐私过滤
5. 文件上传支持本地存储和 S3（可配置）
6. SSO 支持 GitHub、Google 以及任何 OpenID Connect (OIDC) 提供商
7. 评论审核可配置；当前引擎基于关键词，AI API 集成计划中
8. 主题系统允许通过 ZIP 上传自定义前端主题
9. 目前未实现速率限制
