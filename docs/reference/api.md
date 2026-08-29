# VexGo API Documentation

Base URL: `/api`

Authentication: JWT Bearer token (except for public endpoints).
All authenticated requests must include: `Authorization: Bearer <token>`

Roles: `super_admin`, `admin`, `author`, `contributor`, `guest`. Endpoints marked "admin only" require the `admin` or `super_admin` role (`super_admin` always passes the permission check).

---

## Table of Contents

- [Public APIs](#public-apis)
- [Authentication](#authentication)
- [SSO (Single Sign-On)](#sso-single-sign-on)
- [Posts Management](#posts-management)
- [Comments](#comments)
- [Likes](#likes)
- [File Upload](#file-upload)
- [Notifications](#notifications)
- [Moderation](#moderation)
- [User Management](#user-management)
- [Creator Applications](#creator-applications)
- [Configuration](#configuration)
- [Error Responses](#error-responses)
- [Pagination](#pagination)
- [Data Types](#data-types)
- [Notes](#notes)

---

## Public APIs

### GET /posts

Get a paginated list of posts with filtering support.

**Query Parameters:**

- `page` (int, default: 1): Page number
- `limit` (int, default: 10, max: 100): Items per page
- `category` (string): Filter by category name
- `search` (string): Search in title and content

**Response:**

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

**Notes:**

- Guests can only see published posts (when `allowGuestViewPosts` is enabled)
- Non-admin users can only see published posts from other users
- Admins can see all posts except rejected ones

---

### GET /posts/:slug

Get a single post by URL slug.

**Response:**

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

**Error Responses:**

- `403`: `{"error": "You must be logged in to view this post"}` (guest viewing disabled)
- `404`: `{"error": "Post does not exist", "postId": "<id>"}`

---

---

### GET /posts/by-id/:id

Get a single post by numeric ID. Useful when the caller has an ID but not the slug (e.g., from notification data).

**Response:** Same format as `GET /posts/:slug`.

**Error Responses:**

- `403`: `{"error": "You must be logged in to view this post"}` (guest viewing disabled)
- `404`: `{"error": "Post does not exist", "postId": "<id>"}`

---

### GET /categories

Get all categories. Each item carries `postCount`, the number of posts whose `category` field equals the category name.

**Response:**

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

**Notes:**

- Guests can view categories only when `allowGuestViewPosts` is enabled
- If guest viewing is disabled and the user is not logged in, returns an empty array

---

### GET /tags

Get all tags. Each item carries `postCount`, the number of posts referencing the tag.

**Response:**

```json
{
  "tags": [{ "id": 1, "name": "golang", "postCount": 3 }]
}
```

Same guest-visibility rules as `/categories`.

---

### POST /categories

Create a new category. Requires authentication, and the caller must be a contributor or above (`contributor`, `author`, `admin`, `super_admin`). The `guest` role and anonymous requests are rejected.

**Request:**

```json
{
  "name": "tech",
  "description": "Technology-related posts"
}
```

`name` is required; `description` is optional. Category names must be unique.

**Error Responses:**

- `401`: `{"error": "No authentication information provided"}` (missing/invalid token)
- `403`: `{"error": "Insufficient permissions to create a category"}` (guest role or insufficient role)
- `409`: `{"error": "A category with this name already exists", "code": "duplicate_name"}` (duplicate name)
- `400`: `{"error": "..."}` (missing/invalid `name`)

**Response (201):**

```json
{
  "message": "Category created successfully",
  "category": {
    "id": 3,
    "name": "tech",
    "description": "Technology-related posts"
  }
}
```

---

### POST /tags

Create a new tag. Requires authentication, and the caller must be a contributor or above (`contributor`, `author`, `admin`, `super_admin`). The `guest` role and anonymous requests are rejected.

**Request:**

```json
{
  "name": "golang"
}
```

`name` is required and uniquely identifies the tag.

**Error Responses:**

- `401`: `{"error": "No authentication information provided"}` (missing/invalid token)
- `403`: `{"error": "Insufficient permissions to create a tag"}` (guest role or insufficient role)
- `409`: `{"error": "A tag with this name already exists", "code": "duplicate_name"}` (duplicate name)
- `400`: `{"error": "..."}` (missing/invalid `name`)

**Response (201):**

```json
{
  "message": "Tag created successfully",
  "tag": { "id": 4, "name": "golang" }
}
```

---

### DELETE /categories/:id

Delete a category. Requires authentication, and the caller must be a contributor or above (`contributor`, `author`, `admin`, `super_admin`). The `guest` role and anonymous requests are rejected.

A category can only be deleted when it is **empty** — no post's `category` field equals the category name. Categories still in use are never force-deleted or unlinked.

**Response (200):**

```json
{
  "message": "Category deleted successfully"
}
```

**Error Responses:**

- `401`: `{"error": "No authentication information provided"}` (missing/invalid token)
- `403`: `{"error": "Insufficient permissions to delete a category"}` (guest role or insufficient role)
- `400`: `{"error": "Category is used by N posts"}` (the category is still referenced by posts)
- `404`: `{"error": "Category does not exist"}` (unknown, zero or non-numeric `id`)

---

### DELETE /tags/:id

Delete a tag. Requires authentication, and the caller must be a contributor or above (`contributor`, `author`, `admin`, `super_admin`). The `guest` role and anonymous requests are rejected.

A tag can only be deleted when it is **empty** — no post references it via the `post_tags` association. Tags still in use are never force-deleted or unlinked.

**Response (200):**

```json
{
  "message": "Tag deleted successfully"
}
```

**Error Responses:**

- `401`: `{"error": "No authentication information provided"}` (missing/invalid token)
- `403`: `{"error": "Insufficient permissions to delete a tag"}` (guest role or insufficient role)
- `400`: `{"error": "Tag is used by N posts"}` (the tag is still referenced by posts)
- `404`: `{"error": "Tag does not exist"}` (unknown, zero or non-numeric `id`)

---

### GET /stats

**Response:**

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

Get most popular posts (ranked by likes and views).

**Query Parameters:**

- `limit` (int, default: 5): Number of posts to return

**Response:**

```json
{ "posts": [...] }
```

---

### GET /stats/latest-posts

Get latest posts by creation date.

**Query Parameters:**

- `limit` (int, default: 5): Number of posts to return

**Response:**

```json
{ "posts": [...] }
```

---

### GET /themes

Get all available themes.

**Response:**

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

The embedded default theme is always returned. Additional themes installed under the data directory (`data/theme/<id>/vexgo-theme.json`) are appended.

---

### GET /theme/:id/preview

Get the preview image for a specific theme.

**Response:** Image file (PNG/JPG)

**Error Responses:**

- `404`: `{"error": "theme not found"}` / `{"error": "preview not specified"}` / `{"error": "preview image not found"}`

---

### GET /comments/post/:id

Get published comments for a post (optional login, applies author privacy filtering).

**Response:**

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

Get like status and count for a post (optional login).

**Response:**

```json
{ "postId": 1, "likesCount": 42, "isLiked": false }
```

---

### GET /posts/user/:id

Get published posts of a specific user.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{
  "posts": [...],
  "pagination": { "total": 20, "page": 1, "limit": 10, "totalPages": 2 }
}
```

---

### GET /verify-email

Verify an email address using a token.

**Query Parameters:**

- `token` (string, required): Verification token

**Response (initial verification):**

```json
{ "message": "Email verification successful! You can now log in." }
```

**Response (email change - user found):**

```json
{
  "message": "Email change successful! Your new email is now active.",
  "require_relogin": true,
  "new_email": "newemail@example.com"
}
```

**Response (email change - user not found):**

```json
{
  "message": "Email change successful! Your new email is now active.",
  "require_relogin": true
}
```

**Notes:**

- Tokens prefixed with `email-change-` verify an email change; other tokens verify the initial email
- After an email change the user must log in again (`require_relogin: true`)

---

### GET /captcha

Generate a sliding puzzle captcha.

**Response:**

```json
{
  "id": "uuid",
  "token": "captcha_token",
  "bg_image": "data:image/png;base64,...",
  "puzzle_img": "data:image/png;base64,...",
  "y": 100,
  "expires_at": "2026-03-17T21:14:35Z"
}
```

**Notes:**

- Only the X coordinate is verified; `y` is returned for the frontend
- A captcha can be verified only once and expires after 5 minutes

---

### POST /captcha/verify

Verify a captcha token and position.

**Request:**

```json
{ "id": "uuid", "token": "token", "x": 150 }
```

**Response (success):**

```json
{ "success": true, "message": "Verification successful" }
```

**Error Responses:**

- `404`: `{"error": "Captcha does not exist or has expired"}`
- `400`: `{"error": "Captcha already used"}`
- `400`: `{"error": "Captcha has expired"}`
- `400`: `{"error": "Verification failed, please try again"}`

X position verification allows ±10 pixel tolerance.

---

## Authentication

### POST /auth/register

Register a new user account.

**Request:**

```json
{
  "email": "user@example.com",
  "password": "password123",
  "username": "username",
  "captcha_id": "uuid",
  "captcha_token": "token",
  "captcha_x": 150
}
```

**Response (201):**

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

**Error Responses:**

- `409`: `{"error": "user already exists"}`
- `403`: `{"error": "registration is disabled, please contact administrator"}`
- `400`: captcha required / captcha expired / captcha mismatch

**Notes:**

- Captcha fields are required when `captchaEnabled` is turned on in general settings
- `captcha_id`, `captcha_token` and `captcha_x` may be empty strings / 0 when captcha is disabled

---

### POST /auth/login

Login with email and password.

**Request:**

```json
{
  "email": "user@example.com",
  "password": "password123",
  "captcha_id": "uuid",
  "captcha_token": "token",
  "captcha_x": 150
}
```

**Response (200):**

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

**Error Responses:**

- `401`: `{"message": "invalid email or password"}`
- `403`: `{"message": "Please verify your email address first. ...", "email_verified": false}`
- `400`: `{"error": "please complete the captcha verification"}`

---

### GET /auth/me

Get current user information (requires authentication).

**Response:**

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

Alias of `/auth/me` (requires authentication).

---

### PUT /auth/profile

Update the current user's profile (requires authentication). All fields are optional.

**Request:**

```json
{
  "username": "new_username",
  "bio": "My bio",
  "avatar": "url_to_avatar",
  "birthday": "2023-01-01"
}
```

**Response:**

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

Change the current user's password (requires authentication).

**Request:**

```json
{ "oldPassword": "old_password", "newPassword": "new_password" }
```

**Response:**

```json
{ "message": "Password changed successfully" }
```

`newPassword` must be at least 6 characters. Changing the password invalidates previously issued tokens.

---

### PUT /auth/email

Request an email change (requires authentication). A verification email is sent to the new address.

**Request:**

```json
{ "email": "newemail@example.com" }
```

**Response:**

```json
{
  "message": "Verification email sent. Please check your inbox and click the link to complete email change.",
  "pending": true
}
```

---

### PUT /auth/settings

Update the current user's privacy settings (requires authentication).

**Request:**

```json
{
  "profile_visibility": "public",
  "hide_email": true,
  "hide_birthday": false,
  "hide_bio": false
}
```

**Response:**

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

Request a password reset email (public).

**Request:**

```json
{ "email": "user@example.com" }
```

**Response:**

```json
{ "message": "If the email exists, reset link has been sent" }
```

---

### POST /auth/reset-password

Reset a password with the emailed token (public).

**Request:**

```json
{ "token": "reset_token", "password": "new_password" }
```

**Response:**

```json
{ "message": "Password reset successfully" }
```

`password` must be at least 6 characters.

---

### GET /auth/verification-status

Get the current user's email verification status (requires authentication).

**Response:**

```json
{ "email_verified": true, "email": "user@example.com" }
```

---

## SSO (Single Sign-On)

### GET /sso/providers

Get the enabled SSO providers (public).

**Response:**

```json
{ "providers": ["github", "google"], "allow_local_login": true }
```

`providers` contains only enabled providers (GitHub, Google, and OIDC are supported). `allow_local_login` is `false` when password login is disabled.

---

### GET /sso/:provider/login

Initiate an SSO login (redirects to the provider).

**Query Parameters:**

- `method` (string, optional, default: `sso_get_token`):
  - `sso_get_token` — full login; a JWT is issued on callback
  - `get_sso_id` — returns only the provider-side ID (used to bind SSO to an existing account from the settings page)

**Response:** `302` redirect to the OAuth2/OIDC provider.

---

### GET /sso/:provider/callback

SSO callback endpoint; the provider redirects here.

**Query Parameters:**

- `code` (string): Authorization code
- `state` (string): CSRF state parameter
- `method` (string): Same value as the login request

**Response:** An HTML page (not JSON). The result is written to `localStorage` under the key `sso_callback_result` and the popup window closes. The opener page reads the result via the `storage` event:

- On success, the payload is a JSON string, e.g. `{"token": "<jwt>"}` (for `sso_get_token`) or `{"sso_id": "<provider user id>"}` (for `get_sso_id`)
- On error, the payload is `{"error": "<message>"}` and the response status is `400`

---

## Posts Management

### POST /posts

Create a new post (requires authentication).

**Request:**

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

`slug`, `title`, `content` and `category` are required. `slug` must be a URL-safe string (lowercase letters/digits, hyphen-separated, no leading/trailing/consecutive hyphens). `category` accepts a category name or ID; `status` is one of `draft` / `pending` / `published`.

**Error Responses:**

- `400`: Invalid or empty slug
- `409`: `{"error": "Slug is already taken by another post", "code": "slug_taken"}` (duplicate slug)

**Response (201):**

```json
{
  "message": "Post created successfully",
  "post": { "id": 1, "slug": "my-post", "title": "My Post", "status": "draft" }
}
```

---

### GET /posts/user/my-posts

Get the current user's posts (requires authentication).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)
- `status` (string, optional): Filter by post status

**Response:**

```json
{ "posts": [...], "pagination": { "total": 10, "page": 1, "limit": 10, "totalPages": 1 } }
```

---

### GET /posts/drafts

Get the current user's draft posts (requires authentication).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{ "posts": [...], "pagination": { "total": 3, "page": 1, "limit": 10, "totalPages": 1 } }
```

---

### PUT /posts/:id

Update a post (requires authentication; author or admin only).

**Request:** Same fields as `POST /posts`; all optional.

**Response:**

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

**Error Responses:**

- `400`: Invalid slug format
- `403`: `{"error": "Not authorized to modify this post"}`
- `404`: `{"error": "Post does not exist"}`
- `409`: `{"error": "Slug is already taken by another post", "code": "slug_taken"}`

---

### DELETE /posts/:id

Delete a post (requires authentication; author or admin only).

**Response:**

```json
{ "message": "Post deleted successfully" }
```

---

## Comments

### POST /comments

Create a comment (requires authentication).

**Request:**

```json
{ "postId": 1, "content": "This is a comment", "parentId": null }
```

`postId` accepts a number or a numeric string; `content` is limited to 100 characters; `parentId` is optional for reply comments.

**Response (201):**

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

`requiresModeration` is `true` when the comment was created with status `pending` (moderation enabled).

---

### DELETE /comments/:id

Delete a comment (requires authentication; comment author or admin only).

**Response:**

```json
{ "message": "Comment deleted", "commentsCount": 1 }
```

---

## Likes

### POST /likes/:postId

Toggle a like on a post (requires authentication).

**Response:**

```json
{
  "message": "Liked successfully",
  "postId": 1,
  "isLiked": true,
  "likesCount": 42
}
```

Calling it again removes the like:

```json
{ "message": "Like removed", "postId": 1, "isLiked": false, "likesCount": 41 }
```

---

## File Upload

### POST /upload/file

Upload a single file (requires authentication).

**Form Data:**

- `file` (file): The file to upload

**Response:**

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

Files are stored with a UUID filename. Storage is local disk or S3 depending on configuration.

---

### POST /upload/files

Upload multiple files (requires authentication).

**Form Data:**

- `files` (file[]): Multiple files

**Response:**

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

Get the current user's uploaded files (requires authentication).

**Response:**

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

Delete an uploaded file (requires authentication; uploader or admin only).

**Response:**

```json
{ "message": "File deleted" }
```

---

## Notifications

### GET /notifications

Get the current user's notifications (requires authentication).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)
- `type` (string, optional): Filter by notification type (`comment`, `like`, `reply`, `review`, `role`)
- `is_read` (string, optional): Filter by read status (`true` or `false`)

**Response:**

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

Get the current user's unread notification count (requires authentication).

**Response:**

```json
{ "unreadCount": 5 }
```

---

### PUT /notifications/:id/read

Mark a notification as read (requires authentication).

**Response:**

```json
{ "message": "Notification marked as read" }
```

---

### PUT /notifications/read-all

Mark all notifications as read (requires authentication).

**Response:**

```json
{ "message": "All notifications marked as read" }
```

---

### DELETE /notifications/:id

Delete a notification (requires authentication; notification owner only).

**Response:**

```json
{ "message": "Notification deleted" }
```

---

## Moderation

All moderation endpoints require the `admin` or `super_admin` role.

### Comments Moderation

#### GET /moderation/comments/pending

Get pending comments for moderation.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

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

Get approved (`published`) comments. Same parameters and shape as above.

#### GET /moderation/comments/rejected

Get rejected comments. Same parameters and shape as above.

#### PUT /moderation/comments/approve/:id

Approve a comment (status → `published`).

**Response:**

```json
{ "message": "Comment approved", "comment": { "id": 1, "status": "published" } }
```

#### PUT /moderation/comments/reject/:id

Reject a comment (status → `rejected`). No request body.

**Response:**

```json
{ "message": "Comment rejected", "comment": { "id": 1, "status": "rejected" } }
```

#### GET /moderation/comments/config

Get the comment moderation configuration (API key masked).

**Response:**

```json
{
  "enabled": false,
  "modelProvider": "",
  "apiKey": "",
  "apiEndpoint": "",
  "modelName": "gpt-3.5-turbo",
  "moderationPrompt": "Please review the following comment for compliance. ...",
  "blockKeywords": "spam,advertisement",
  "autoApproveEnabled": true,
  "minScoreThreshold": 0.5
}
```

**Notes:**

- When `enabled` is `true`, new comments are created with status `pending` and run through the moderation engine
- When `autoApproveEnabled` is `true`, comments pass automatically when moderation is disabled
- The current moderation engine is keyword-based (blocked keywords plus simple content checks); an AI API integration is planned

#### PUT /moderation/comments/config

Update the comment moderation configuration.

**Request:** Same fields as the GET response, plus `apiKey` (leave empty to keep the existing key).

**Response:**

```json
{
  "message": "Comment moderation configuration updated successfully",
  "config": {
    "enabled": true,
    "modelProvider": "openai",
    "modelName": "gpt-3.5-turbo",
    "autoApproveEnabled": true,
    "minScoreThreshold": 0.5
  }
}
```

### Posts Moderation

#### GET /moderation/pending

Get posts with status `pending`.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)
- `search` (string, optional)

**Response:**

```json
{ "posts": [...], "pagination": { "total": 2, "page": 1, "limit": 10, "totalPages": 1 } }
```

#### GET /moderation/approved

Get posts with status `published`. Same parameters and shape as above.

#### GET /moderation/rejected

Get posts with status `rejected`. Same parameters and shape as above.

#### PUT /moderation/approve/:id

Approve a post (status → `published`).

**Response:**

```json
{ "message": "Post approved", "post": { "id": 1, "status": "published" } }
```

#### PUT /moderation/reject/:id

Reject a post (status → `rejected`).

**Request:**

```json
{ "rejectionReason": "Reason for rejection" }
```

**Response:**

```json
{
  "message": "Post has been rejected",
  "post": { "id": 1, "status": "rejected" }
}
```

#### PUT /moderation/resubmit/:id

Resubmit a rejected post (status → `pending`).

**Response:**

```json
{
  "message": "Post resubmitted for moderation",
  "post": { "id": 1, "status": "pending" }
}
```

Only rejected posts can be resubmitted.

---

## User Management

All endpoints in this section require the `admin` or `super_admin` role.

### GET /users

Get a paginated user list.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10, max: 100)
- `search` (string, optional): Search by username or email

**Response:**

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

Update a user's role.

**Request:**

```json
{ "role": "admin" }
```

Valid roles: `super_admin`, `admin`, `author`, `contributor`, `guest`.

**Response:**

```json
{
  "message": "User role updated successfully",
  "user": { "id": 2, "role": "admin" }
}
```

**Notes:**

- A user cannot modify their own role
- Only a `super_admin` can modify a `super_admin` account

### DELETE /users/:id

Delete a user and all their posts and comments.

**Response:**

```json
{ "message": "User deleted successfully" }
```

**Notes:**

- A user cannot delete their own account
- A non-`super_admin` cannot delete a `super_admin` account

---

## Creator Applications

### POST /users/apply-creator

Submit a creator application (requires authentication).

Only users with the `guest` or `contributor` role can apply for a role upgrade.

**Request:**

```json
{ "reason": "I want to publish posts" }
```

**Response:**

```json
{ "message": "Application submitted successfully", "applicationId": 1 }
```

**Error Responses:**

- `400`: `{"error": "only guest and contributor users can apply for role upgrade"}`
- `400`: `{"error": "..."}` (an application is already pending)

### GET /users/creator-applications

Get creator applications for review (admin only).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10, max: 100)
- `status` (string, default: `pending`): `pending`, `approved`, `rejected`

**Response:**

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

Review a creator application (admin only).

**Request:**

```json
{ "action": "approve", "reason": "Optional review note" }
```

`action` is `approve` or `reject`.

**Response:**

```json
{ "message": "Application reviewed successfully" }
```

---

## Configuration

All endpoints in this section require the `admin` or `super_admin` role, except `GET /config/general` and `GET /config/theme`, which are public.

### SMTP Configuration

#### GET /config/smtp

Get the SMTP configuration (password not returned).

**Response:**

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

Update the SMTP configuration. Returns the updated config.

**Request:**

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

Leave `password` empty to keep the existing one.

**Response:** The updated SMTP configuration (same shape as `GET /config/smtp`).

#### POST /config/smtp/test

Send a test email. No request body — the recipient is the configured `testEmail`, falling back to the acting admin's email.

**Response:**

```json
{
  "message": "Test email has been sent to your inbox",
  "to": "test@example.com"
}
```

### AI Configuration

#### GET /config/ai

Get the AI configuration (API key not returned).

**Response:**

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

Update the AI configuration.

**Request:**

```json
{
  "enabled": true,
  "provider": "openai",
  "apiEndpoint": "https://api.openai.com/v1",
  "apiKey": "sk-...",
  "modelName": "gpt-3.5-turbo"
}
```

Leave `apiKey` empty to keep the existing one.

**Response:**

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

Test the AI connection. No request body; uses a built-in test prompt.

**Response:**

```json
{ "message": "AI connection test successful!", "response": "This is a test." }
```

#### GET /config/ai/models

List the models available from the configured AI endpoint.

**Response:**

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

### General Settings

#### GET /config/general

Get the general settings (public access).

**Response:**

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

Update the general settings (admin only).

**Request:** Same fields as the GET response.

**Response:**

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

### Theme Configuration

#### GET /config/theme

Get the currently active theme (public access).

**Response:**

```json
{ "activeTheme": "default" }
```

#### PUT /config/theme

Set the globally active theme (admin only).

**Request:**

```json
{ "activeTheme": "theme_id" }
```

**Response:**

```json
{ "message": "Theme updated successfully", "activeTheme": "theme_id" }
```

#### POST /themes/upload

Upload and install a new theme (admin only).

**Form Data:**

- `theme` (file): ZIP archive containing the theme files and a `vexgo-theme.json` metadata file

**Response:**

```json
{ "message": "Theme uploaded successfully" }
```

**Theme structure:**

```text
theme.zip
└── theme-id/
    ├── vexgo-theme.json (metadata, required)
    ├── preview.png (optional preview image)
    └── dist/ (built frontend assets, index.html, ...)
```

**vexgo-theme.json:**

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

Installed themes are extracted to `data/theme/<id>/` and served by the renderer; the embedded default theme (`default`) is always available.

---

## Error Responses

All endpoints may return the following error responses:

**400 Bad Request:**

```json
{ "error": "Invalid request parameters" }
```

**401 Unauthorized:**

```json
{ "error": "Unauthorized" }
```

**403 Forbidden:**

```json
{ "error": "Insufficient permissions" }
```

**404 Not Found:**

```json
{ "error": "Resource not found" }
```

**409 Conflict:**

```json
{ "error": "Resource already exists" }
```

**500 Internal Server Error:**

```json
{ "error": "Internal server error" }
```

Note: some endpoints return `{"message": ...}` for auth-related failures instead of `{"error": ...}` — see the individual endpoint sections.

---

## Pagination

Paginated endpoints accept `page` and `limit` query parameters and return:

```json
{
  "data": [...],
  "pagination": { "total": 100, "page": 1, "limit": 10, "totalPages": 10 }
}
```

The actual list key varies per endpoint (`posts`, `comments`, `users`, `notifications`, `applications`).

---

## Data Types

### User Role Types

- `super_admin`: Super administrator (highest privilege, bypasses all permission checks)
- `admin`: Administrator
- `author`: Author (can publish posts)
- `contributor`: Contributor (can apply for a role upgrade)
- `guest`: Guest / newly registered user (limited access)

### Post Status

- `draft`: Draft (only visible to the author)
- `pending`: Pending moderation
- `published`: Published and visible
- `rejected`: Rejected by a moderator

### Comment Status

- `published`: Approved and visible
- `pending`: Pending moderation (when moderation is enabled)
- `rejected`: Rejected by a moderator

---

## Notes

1. All timestamps are in RFC 3339 format (ISO 8601 with timezone)
2. Authentication middleware validates JWT tokens and sets `userID` and `user` in the Gin context
3. Permission middleware checks the user's role (from the database) against the required roles; `super_admin` always passes
4. Privacy filtering is applied to user data based on the viewer's role and the target user's privacy settings
5. File uploads support both local storage and S3 (configurable)
6. SSO supports GitHub, Google, and any OpenID Connect (OIDC) provider
7. Comment moderation is configurable; the current engine is keyword-based, with an AI API integration planned
8. The theme system allows custom frontend themes via ZIP upload
9. Currently no rate limiting is implemented
