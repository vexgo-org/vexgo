# VexGo API Documentation

Base URL: `/api`

Authentication: JWT Bearer token (except for public endpoints)
All authenticated requests should include: `Authorization: Bearer <token>`

---

## Table of Contents

- [Public APIs](#public-apis)
- [Authentication](#authentication)
- [SSO (Single Sign-On)](#sso-single-sign-on)
- [Posts Management](#posts-management)
- [Comments](#comments)
- [Likes](#likes)
- [File Upload](#file-upload)
- [Moderation](#moderation)
- [User Management](#user-management)
- [Configuration](#configuration)
- [Themes](#themes)
- [Statistics](#statistics)
- [Categories & Tags](#categories--tags)
- [Captcha](#captcha)

---

## Public APIs

### GET /posts

Get paginated list of posts with filtering support.

**Query Parameters:**

- `page` (int, default: 1): Page number
- `limit` (int, default: 10, max: 100): Items per page
- `category` (string): Filter by category ID
- `status` (string): Filter by status (published, pending, rejected, draft)
- `search` (string): Search in title and content

**Response:**

```json
{
  "pagination": { "limit": 10, "page": 1, "total": 1, "totalPages": 1 },
  "posts": [
    {
      "id": 1,
      "title": "tests",
      "content": "testdvsdf\n# safsf\n",
      "excerpt": "",
      "coverImage": "",
      "viewCount": 5,
      "authorId": 1,
      "author": {
        "id": 1,
        "username": "admin",
        "email": "admin@example.com",
        "role": "super_admin",
        "email_verified": true,
        "verification_token": "",
        "token_expires_at": null,
        "createdAt": "2026-03-16T22:10:01.484631752+08:00",
        "profile_visibility": "public"
      },
      "category": "test1",
      "tags": [],
      "status": "published",
      "rejectionReason": "",
      "createdAt": "2026-03-17T21:14:35.026603945+08:00",
      "updatedAt": "2026-03-17T21:14:35.026603945+08:00",
      "likesCount": 0,
      "isLiked": false,
      "commentsCount": 0
    }
  ]
}
```

**Notes:**

- Guests can only see published posts (if `allow_guest_view_posts` is enabled)
- Non-admin users can only see published posts from other users
- Admins can see all posts regardless of status

---

### GET /posts/:id

Get a single post by ID.

**Response:**

```json
{
  "post": {
    "id": 1,
    "title": "tests",
    "content": "testdvsdf\n# safsf\n",
    "excerpt": "",
    "coverImage": "",
    "viewCount": 5,
    "authorId": 1,
    "author": {
      "id": 1,
      "username": "admin",
      "email": "admin@example.com",
      "role": "super_admin",
      "email_verified": true,
      "verification_token": "",
      "token_expires_at": null,
      "createdAt": "2026-03-16T22:10:01.484631752+08:00",
      "profile_visibility": "public"
    },
    "category": "test1",
    "tags": [],
    "status": "published",
    "rejectionReason": "",
    "createdAt": "2026-03-17T21:14:35.026603945+08:00",
    "updatedAt": "2026-03-17T21:14:35.026603945+08:00",
    "likesCount": 0,
    "isLiked": false,
    "commentsCount": 0
  }
}
```

---

### GET /verify-email

Verify email address using token.

**Query Parameters:**

- `token` (string, required): Verification token

**Response (email verification):**

```json
{
  "message": "Email verification successful! You can now log in."
}
```

**Response (email change - success with user found):**

```json
{
  "message": "Email change successful! Your new email is now active.",
  "require_relogin": true,
  "new_email": "newemail@example.com"
}
```

**Response (email change - success but user not found):**

```json
{
  "message": "Email change successful! Your new email is now active.",
  "require_relogin": true
}
```

**Error Response:**

```json
{
  "error": "Verification token cannot be empty"
}
```

**Notes:**

- Token prefix `email-change-` indicates email change verification
- Normal tokens are for initial email verification
- After email change, user needs to re-login (`require_relogin: true`)

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
  "expires_at": "2024-01-01T00:00:00Z"
}
```

**Field Descriptions:**

- `id`: Captcha unique identifier
- `token`: Verification token (used with `/captcha/verify`)
- `bg_image`: Base64 encoded background image with缺口 (missing puzzle piece area)
- `puzzle_img`: Base64 encoded puzzle piece image (the draggable piece)
- `y`: Y coordinate of the puzzle position (for reference, not used in verification)
- `expires_at`: Expiration timestamp (5 minutes from generation)

**Notes:**

- Only the X coordinate (`x`) is verified; Y is provided for frontend reference
- Images are returned as Base64 encoded PNG data URLs
- Captcha can only be verified once and expires after 5 minutes

---

### POST /captcha/verify

Verify captcha token and position.

**Request:**

```json
{
  "id": "uuid",
  "token": "token",
  "x": 150
}
```

**Response (success):**

```json
{
  "success": true,
  "message": "Verification successful"
}
```

**Error Responses:**

Verification failed (wrong position):

```json
{
  "error": "Verification failed, please try again"
}
```

Captcha already used:

```json
{
  "error": "Captcha already used"
}
```

Captcha expired:

```json
{
  "error": "Captcha has expired"
}
```

Captcha not found:

```json
{
  "error": "Captcha does not exist or has expired"
}
```

**Notes:**

- X coordinate verification allows ±10 pixel tolerance
- Captcha can only be verified once
- Captcha expires after 5 minutes

---

### GET /categories

Get all categories.

**Response:**

```json
{
  "categories": [
    {
      "id": 1,
      "name": "Default",
      "description": "Default category for articles without a specified category"
    },
    { "id": 2, "name": "test", "description": "" },
    { "id": 3, "name": "test1", "description": "" },
    { "id": 4, "name": "12", "description": "" }
  ]
}
```

**Notes:**

- Returns all categories from database
- Guests can view categories only if `allow_guest_view_posts` is enabled
- If guest viewing is disabled and user is not logged in, returns empty array

---

### GET /tags

Get all tags.

**Response:**

```json
{
  "tags": [
    {
      "id": 1,
      "name": "golang"
    }
  ]
}
```

**Notes:**

- Returns all tags from database
- Guests can view tags only if `allow_guest_view_posts` is enabled
- If guest viewing is disabled and user is not logged in, returns empty array

---

### GET /stats

Get site statistics.

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

Get most popular posts by like count.

**Query Parameters:**

- `limit` (int, default: 5): Number of posts to return

**Response:**

```json
{
  "posts": [...]
}
```

---

### GET /stats/latest-posts

Get latest posts by creation date.

**Query Parameters:**

- `limit` (int, default: 5): Number of posts to return

**Response:**

```json
{
  "posts": [...]
}
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
      "name": "Default Theme",
      "description": "Default VexGo theme",
      "author": "VexGo Team",
      "version": "1.0.0",
      "preview": "/path/to/preview.png"
    }
  ]
}
```

---

### GET /theme/:id/preview

Get preview image for a specific theme.

**Response:** Image file (PNG/JPG)

---

### GET /comments/post/:id

Get published comments for a post.

**Response:**

```json
{
  "comments": [
    {
      "id": 1,
      "content": "Comment text",
      "user": {
        "id": 1,
        "username": "user1"
      },
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

### GET /likes/:postId

Get like status and count for a post.

**Response:**

```json
{
  "postId": 1,
  "likesCount": 42,
  "isLiked": false
}
```

---

### GET /posts/user/:id

Get posts by a specific user.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{
  "posts": [...],
  "pagination": {
    "total": 20,
    "page": 1,
    "limit": 10,
    "totalPages": 2
  }
}
```

---

## Authentication

All endpoints under `/api/auth` are for authentication and user management.

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

**Response (Success):**

```json
{
  "message": "Registration successful! Please check your email to verify your account."
}
```

**Response (Error):**

```json
{
  "error": "User already exists"
}
```

**Notes:**

- Registration may be disabled by admin
- Captcha verification required if enabled
- Email verification required before login

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

**Response (Success):**

```json
{
  "token": "jwt_token_here",
  "user": {
    "id": 1,
    "email": "user@example.com",
    "username": "username",
    "role": "user",
    "email_verified": true
  }
}
```

**Response (Error):**

```json
{
  "error": "Invalid credentials"
}
```

**Notes:**

- Captcha may be required based on settings
- Returns JWT token for authenticated requests

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
    "role": "user",
    "email_verified": true,
    "avatar": "url_to_avatar",
    "bio": "My bio",
    "birthday": "2023-01-01",
    "created_at": "2024-01-01T00:00:00Z",
    "profile_visibility": "public",
    "hide_email": false,
    "hide_birthday": false,
    "hide_bio": false
  }
}
```

---

### GET /auth/user

Alias for `/auth/me` (requires authentication).

---

### PUT /auth/profile

Update user profile (requires authentication).

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
    "email": "user@example.com",
    "username": "new_username",
    "role": "user",
    "email_verified": true,
    "avatar": "url_to_avatar",
    "bio": "My bio",
    "birthday": "2023-01-01",
    "created_at": "2024-01-01T00:00:00Z",
    "profile_visibility": "public",
    "hide_email": false,
    "hide_birthday": false,
    "hide_bio": false
  }
}
```

---

### PUT /auth/password

Change user password (requires authentication).

**Request:**

```json
{
  "current_password": "old_password",
  "new_password": "new_password"
}
```

**Response:**

```json
{
  "message": "Password changed successfully"
}
```

---

### PUT /auth/email

Request email change (requires authentication).

**Request:**

```json
{
  "email": "newemail@example.com"
}
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

Update user settings (requires authentication).

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
    "email": "user@example.com",
    "username": "username",
    "role": "user",
    "email_verified": true,
    "avatar": "url_to_avatar",
    "bio": "My bio",
    "birthday": "2023-01-01",
    "created_at": "2024-01-01T00:00:00Z",
    "profile_visibility": "public",
    "hide_email": true,
    "hide_birthday": false,
    "hide_bio": false
  }
}
```

---

### POST /auth/request-password-reset

Request password reset email.

**Request:**

```json
{
  "email": "user@example.com"
}
```

**Response:**

```json
{
  "message": "If the email exists, a reset link has been sent"
}
```

---

### POST /auth/reset-password

Reset password with token.

**Request:**

```json
{
  "token": "reset_token",
  "new_password": "new_password"
}
```

**Response:**

```json
{
  "message": "Password reset successful"
}
```

---

### GET /auth/verification-status

Get email verification status (requires authentication).

**Response:**

```json
{
  "email_verified": true,
  "email": "user@example.com"
}
```

---

### POST /auth/resend-verification

Resend verification email (requires authentication).

**Response:**

```json
{
  "message": "Verification email sent"
}
```

---

## SSO (Single Sign-On)

### GET /sso/providers

Get enabled SSO providers.

**Response:**

```json
{
  "providers": [
    {
      "name": "github",
      "display_name": "GitHub",
      "icon": "/path/to/github.svg",
      "enabled": true
    },
    {
      "name": "google",
      "display_name": "Google",
      "icon": "/path/to/google.svg",
      "enabled": true
    }
  ]
}
```

---

### GET /sso/:provider/login

Initiate SSO login (redirects to provider).

**Query Parameters:**

- `method` (string, optional): `sso_get_token` or `get_sso_id`

**Response:** Redirect to SSO provider

---

### GET /sso/:provider/callback

SSO callback endpoint (provider redirects here).

**Query Parameters:**

- `code` (string): Authorization code
- `state` (string): State parameter for CSRF protection
- `method` (string): Same as in login request

**Response:** Redirects to frontend with token or SSO ID

---

## Posts Management

### POST /api/posts

Create a new post (requires authentication).

**Request:**

```json
{
  "title": "My Post",
  "content": "Post content in markdown...",
  "category": 1,
  "tags": ["golang", "programming"],
  "excerpt": "Post excerpt",
  "coverImage": "/uploads/image.jpg",
  "status": "draft" | "published"
}
```

**Response:**

```json
{
  "message": "Post created successfully",
  "post": {
    "id": 1,
    "title": "My Post",
    "content": "Post content...",
    "status": "draft",
    "category": 1,
    "tags": [...],
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

---

### GET /api/posts/user/my-posts

Get current user's posts (requires authentication).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{
  "posts": [...],
  "pagination": {...}
}
```

---

### GET /api/posts/drafts

Get current user's draft posts (requires authentication).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{
  "posts": [...],
  "pagination": {...}
}
```

---

### PUT /api/posts/:id

Update a post (requires authentication, author only).

**Request:**

```json
{
  "title": "Updated Title",
  "content": "Updated content...",
  "category": 1,
  "tags": ["golang", "programming"],
  "excerpt": "Post excerpt",
  "coverImage": "/uploads/image.jpg",
  "status": "draft" | "published"
}
```

**Response:**

```json
{
  "message": "Post updated successfully",
  "post": {
    "id": 1,
    "title": "Updated Title",
    "content": "Updated content...",
    "status": "published",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

---

### DELETE /api/posts/:id

Delete a post (requires authentication, author or admin only).

**Response:**

```json
{
  "message": "Post deleted successfully"
}
```

---

## Comments

### POST /api/comments

Create a comment (requires authentication).

**Request:**

```json
{
  "postId": 1,
  "content": "This is a comment",
  "parentId": null
}
```

**Response:**

```json
{
  "message": "Comment created successfully",
  "comment": {
    "id": 1,
    "content": "This is a comment",
    "post_id": 1,
    "user_id": 1,
    "parent_id": null,
    "status": "published",
    "created_at": "2024-01-01T00:00:00Z"
  },
  "commentsCount": 1,
  "requiresModeration": false
}
```

**Notes:**

- Comment content limited to 100 characters
- `parentId` is optional for reply comments

---

### DELETE /api/comments/:id

Delete a comment (requires authentication, comment author or admin only).

**Response:**

```json
{
  "message": "Comment deleted",
  "commentsCount": 1
}
```

---

## Likes

### POST /api/likes/:postId

Toggle like on a post (requires authentication).

**Response:**

```json
{
  "message": "Liked successfully" | "Like removed",
  "postId": 1,
  "isLiked": true | false,
  "likesCount": 42
}
```

---

## File Upload

### POST /api/upload/file

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
    "user_id": 1,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

**Notes:**

- Supports local storage or S3 (based on configuration)
- Files are stored with UUID filename to avoid conflicts

---

### POST /api/upload/files

Upload multiple files (requires authentication).

**Form Data:**

- `files[]` (files): Multiple files

**Response:**

```json
{
  "message": "File upload completed",
  "files": [
    {
      "id": 1,
      "url": "/uploads/uuid1.ext",
      "size": 1024,
      "type": "unknown",
      "user_id": 1,
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": 2,
      "url": "/uploads/uuid2.ext",
      "size": 2048,
      "type": "unknown",
      "user_id": 1,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

### GET /api/upload/my-files

Get current user's uploaded files (requires authentication).

**Response:**

```json
{
  "files": [
    {
      "id": 1,
      "url": "/uploads/uuid.ext",
      "size": 1024,
      "type": "unknown",
      "user_id": 1,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

### DELETE /api/upload/:id

Delete a file (requires authentication, file owner or admin only).

**Response:**

```json
{
  "message": "File deleted"
}
```

---

## Moderation

All moderation endpoints require `admin` or `super_admin` role.

### Comments Moderation

#### GET /api/moderation/comments/pending

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
      "user": {...},
      "post": {...},
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {...}
}
```

---

#### GET /api/moderation/comments/approved

Get approved comments.

---

#### GET /api/moderation/comments/rejected

Get rejected comments.

---

#### PUT /api/moderation/comments/approve/:id

Approve a comment.

**Response:**

```json
{
  "message": "Comment approved"
}
```

---

#### PUT /api/moderation/comments/reject/:id

Reject a comment.

**Request:**

```json
{
  "rejection_reason": "Reason for rejection"
}
```

**Response:**

```json
{
  "message": "Comment rejected"
}
```

---

#### GET /api/moderation/comments/config

Get comment moderation configuration.

**Response:**

```json
{
  "enabled": true,
  "model_provider": "openai",
  "api_endpoint": "https://api.openai.com/v1/chat/completions",
  "model_name": "gpt-3.5-turbo",
  "moderation_prompt": "Please review...",
  "block_keywords": "spam,advertisement",
  "auto_approve_enabled": true,
  "min_score_threshold": 0.5
}
```

---

#### PUT /api/moderation/comments/config

Update comment moderation configuration.

**Request:**

```json
{
  "enabled": true,
  "model_provider": "openai",
  "api_key": "new_api_key",
  "api_endpoint": "https://api.openai.com/v1/chat/completions",
  "model_name": "gpt-3.5-turbo",
  "moderation_prompt": "Please review...",
  "block_keywords": "spam,advertisement",
  "auto_approve_enabled": true,
  "min_score_threshold": 0.5
}
```

**Response:**

```json
{
  "message": "Configuration updated"
}
```

---

### Posts Moderation

#### GET /api/moderation/pending

Get pending posts for moderation.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{
  "posts": [...],
  "pagination": {...}
}
```

---

#### GET /api/moderation/approved

Get approved posts.

---

#### GET /api/moderation/rejected

Get rejected posts.

---

#### PUT /api/moderation/approve/:id

Approve a post (status changes to `published`).

**Response:**

```json
{
  "message": "Post approved",
  "post": {...}
}
```

---

#### PUT /api/moderation/reject/:id

Reject a post.

**Request:**

```json
{
  "rejection_reason": "Reason for rejection"
}
```

**Response:**

```json
{
  "message": "Post has been rejected",
  "post": {...}
}
```

---

#### PUT /api/moderation/resubmit/:id

Resubmit a rejected post (status changes back to `pending`).

**Response:**

```json
{
  "message": "Post resubmitted for review"
}
```

---

## User Management

All user management endpoints require `admin` or `super_admin` role.

### GET /api/users

Get user list with pagination.

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)

**Response:**

```json
{
  "users": [
    {
      "id": 1,
      "username": "user1",
      "email": "user@example.com",
      "role": "user",
      "email_verified": true,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {...}
}
```

---

### PUT /api/users/:id/role

Update user role.

**Request:**

```json
{
  "role": "admin" | "user" | "contributor" | "super_admin"
}
```

**Response:**

```json
{
  "message": "User role updated"
}
```

**Notes:**

- Cannot modify own role
- Super admin can only be assigned/deassigned by another super admin

---

### DELETE /api/users/:id

Delete a user.

**Response:**

```json
{
  "message": "User deleted successfully"
}
```

**Notes:**

- Cannot delete self
- Super admin cannot be deleted by non-super admin

---

## Configuration

All configuration endpoints require `admin` or `super_admin` role.

### SMTP Configuration

#### GET /api/config/smtp

Get SMTP configuration (password not returned).

**Response:**

```json
{
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "user@example.com",
  "from_email": "noreply@example.com",
  "from_name": "VexGo"
}
```

---

#### PUT /api/config/smtp

Update SMTP configuration.

**Request:**

```json
{
  "enabled": true,
  "host": "smtp.example.com",
  "port": 587,
  "username": "user@example.com",
  "password": "password", // optional, leave empty to keep existing
  "from_email": "noreply@example.com",
  "from_name": "VexGo"
}
```

**Response:**

```json
{
  "message": "SMTP configuration updated"
}
```

---

#### POST /api/config/smtp/test

Test SMTP connection.

**Request:**

```json
{
  "test_email": "test@example.com"
}
```

**Response:**

```json
{
  "message": "Test email sent successfully"
}
```

---

### AI Configuration

#### GET /api/config/ai

Get AI configuration (API key not returned).

**Response:**

```json
{
  "enabled": true,
  "provider": "openai",
  "api_endpoint": "https://api.openai.com/v1/chat/completions",
  "api_key": "",
  "model_name": "gpt-3.5-turbo"
}
```

---

#### PUT /api/config/ai

Update AI configuration.

**Request:**

```json
{
  "enabled": true,
  "provider": "openai",
  "api_key": "sk-...",
  "api_endpoint": "https://api.openai.com/v1/chat/completions",
  "model_name": "gpt-3.5-turbo"
}
```

**Response:**

```json
{
  "message": "AI configuration updated"
}
```

---

#### POST /api/config/ai/test

Test AI connection.

**Request:**

```json
{
  "test_prompt": "Hello, AI!"
}
```

**Response:**

```json
{
  "response": "Hello! How can I help you today?"
}
```

---

#### GET /api/config/ai/models

Get available AI models (for configured provider).

**Response:**

```json
{
  "models": ["gpt-3.5-turbo", "gpt-4"]
}
```

---

### General Settings

#### GET /api/config/general

Get general settings (public access).

**Response:**

```json
{
  "site_name": "VexGo",
  "site_description": "A modern blog platform",
  "registration_enabled": true,
  "allow_guest_view_posts": true,
  "captcha_enabled": false,
  "default_language": "en"
}
```

---

#### PUT /api/config/general

Update general settings (admin only).

**Request:**

```json
{
  "site_name": "VexGo",
  "site_description": "...",
  "registration_enabled": true,
  "allow_guest_view_posts": true,
  "captcha_enabled": false,
  "default_language": "en"
}
```

**Response:**

```json
{
  "message": "Settings updated"
}
```

---

### Theme Configuration

### GET /api/config/theme

Get current theme configuration.

**Response:**

```json
{
  "active_theme": "default"
}
```

---

### PUT /api/config/theme

Set active theme.

**Request:**

```json
{
  "active_theme": "theme_id"
}
```

**Response:**

```json
{
  "message": "Theme updated successfully",
  "active_theme": "theme_id"
}
```

---

#### POST /api/themes/upload

Upload and install a new theme (admin only).

**Form Data:**

- `theme` (file): ZIP archive containing theme files

**Response:**

```json
{
  "message": "Theme uploaded successfully"
}
```

**Theme Structure:**

```
theme.zip
└── theme-id/
    ├── vexgo-theme.json (metadata)
    ├── preview.png (optional preview image)
    ├── assets/
    │   ├── style.css
    │   └── script.js
    └── templates/
        └── ...
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

---

## Statistics

### GET /api/stats

Get site statistics (total counts).

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

### GET /api/stats/popular-posts

Get most popular posts by like count.

**Query Parameters:**

- `limit` (int, default: 5)

**Response:**

```json
{
  "posts": [
    {
      "id": 1,
      "title": "Popular Post",
      "likes_count": 100,
      ...
    }
  ]
}
```

---

### GET /api/stats/latest-posts

Get latest posts by creation date.

**Query Parameters:**

- `limit` (int, default: 5)

**Response:**

```json
{
  "posts": [...]
}
```

---

## Categories & Tags

### POST /api/categories

Create a new category (admin only).

**Request:**

```json
{
  "name": "Technology",
  "slug": "technology",
  "description": "Tech related posts"
}
```

**Response:**

```json
{
  "id": 1,
  "name": "Technology",
  "slug": "technology",
  "description": "Tech related posts"
}
```

---

### POST /api/tags

Create a new tag (admin only).

**Request:**

```json
{
  "name": "golang"
}
```

**Response:**

```json
{
  "message": "Tag created successfully",
  "tag": {
    "id": 1,
    "name": "golang"
  }
}
```

---

## Captcha

### GET /api/captcha

Generate a sliding puzzle captcha.

**Response:**

```json
{
  "id": "uuid",
  "token": "captcha_token",
  "bg_image": "base64_encoded_image",
  "puzzle_img": "base64_encoded_image",
  "y": 100,
  "expires_at": "2024-01-01T00:00:00Z"
}
```

**Fields:**

- `id`: Captcha ID for verification
- `token`: Token for verification
- `bg_image`: Base64 encoded background image with缺口
- `puzzle_img`: Base64 encoded puzzle piece image
- `y`: Y coordinate of the puzzle position (for reference, not used in verification)
- `expires_at`: Expiration timestamp (5 minutes from generation)

---

### POST /api/captcha/verify

Verify captcha solution.

**Request:**

```json
{
  "id": "uuid",
  "token": "token",
  "x": 150
}
```

**Response:**

```json
{
  "success": true,
  "message": "Verification successful"
}
```

**Notes:**

- Captcha can only be verified once (marked as used after first verification)
- Captcha expires after configured time (default 5 minutes)
- X position verification allows ±10 pixel tolerance

---

## Error Responses

All endpoints may return the following error responses:

**400 Bad Request:**

```json
{
  "error": "Invalid request parameters"
}
```

**401 Unauthorized:**

```json
{
  "error": "Unauthorized"
}
```

**403 Forbidden:**

```json
{
  "error": "Insufficient permissions"
}
```

**404 Not Found:**

```json
{
  "error": "Resource not found"
}
```

**409 Conflict:**

```json
{
  "error": "Resource already exists"
}
```

**500 Internal Server Error:**

```json
{
  "error": "Internal server error"
}
```

---

## Rate Limiting

Currently no rate limiting is implemented. Consider adding rate limiting in production.

---

## Pagination

All paginated endpoints accept `page` and `limit` query parameters and return:

```json
{
  "data": [...],
  "pagination": {
    "total": 100,
    "page": 1,
    "limit": 10,
    "totalPages": 10
  }
}
```

---

## Data Types

### User Role Types

- `user`: Regular user
- `admin`: Administrator
- `super_admin`: Super administrator (highest privilege)
- `guest`: Guest user (limited access)

### Post Status

- `draft`: Draft (only visible to author)
- `pending`: Pending moderation
- `published`: Published and visible
- `rejected`: Rejected by moderator

### Comment Status

- `published`: Approved and visible
- `pending`: Pending moderation (if AI moderation enabled)
- `rejected`: Rejected by moderator

---

## Notes

1. All timestamps are in RFC 3339 format (ISO 8601 with timezone)
2. Authentication middleware validates JWT tokens and sets `userID` and `user` in context
3. Permission middleware checks user role against required roles
4. Privacy filtering is applied to user data based on viewer's role and target user's privacy settings
5. File uploads support both local storage and S3 (configurable)
6. SSO implementation supports GitHub and Google OAuth2
7. AI moderation can be enabled for comments and posts
8. Theme system allows custom frontend themes via ZIP upload

---

## Messages

### GET /api/messages

Get current user's messages (requires authentication).

**Query Parameters:**

- `page` (int, default: 1)
- `limit` (int, default: 10)
- `type` (string, optional): Filter by message type
- `is_read` (string, optional): Filter by read status ('true' or 'false')

**Response:**

```json
{
  "notifications": [
    {
      "id": 1,
      "title": "New notification",
      "content": "You have a new notification",
      "type": "comment",
      "related_id": "1",
      "related_type": "post",
      "is_read": false,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "total": 20,
    "page": 1,
    "limit": 10,
    "totalPages": 2
  }
}
```

---

### GET /api/messages/unread-count

Get unread message count for current user (requires authentication).

**Response:**

```json
{
  "unreadCount": 5
}
```

---

### PUT /api/messages/:id/read

Mark a message as read (requires authentication).

**Response:**

```json
{
  "message": "Message marked as read"
}
```

---

### PUT /api/messages/read-all

Mark all messages as read (requires authentication).

**Response:**

```json
{
  "message": "All messages marked as read"
}
```

---

### DELETE /api/messages/:id

Delete a message (requires authentication, message owner only).

**Response:**

```json
{
  "message": "Message deleted"
}
```

---

## Version

API Version: 0.4.0

Last Updated: 2026-03-25