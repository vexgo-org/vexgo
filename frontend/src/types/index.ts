// User types
// The id field matches the backend's model.User.ID (uint).
// Orval/huma types use number; this is kept in sync so the
// legacy @/types still type-checks against the typed
// @/api/generated surface.
export interface User {
  id: number;
  username: string;
  email: string;
  role: "super_admin" | "admin" | "author" | "contributor" | "guest";
  avatar: string | null;
  emailVerified?: boolean;
  createdAt?: string;
  birthday?: string;
  bio?: string;
  // Privacy settings
  profile_visibility?: "public" | "private";
  hide_email?: boolean;
  hide_birthday?: boolean;
  hide_bio?: boolean;
}

// SMTP config types
export interface SMTPConfig {
  id: string;
  enabled: boolean;
  host: string;
  port: number;
  username: string;
  password: string; // only used for setting, never returned on fetch
  fromEmail: string;
  fromName: string;
  testEmail: string; // recipient email for test messages
  createdAt: string;
  updatedAt: string;
}

// General settings types
export interface GeneralSettings {
  id: string;
  captchaEnabled: boolean; // whether the slider captcha is enabled
  registrationEnabled: boolean; // whether registration is allowed
  allowGuestViewPosts: boolean; // whether guests can view posts
  siteName: string; // site name
  siteDescription: string; // site description
  siteIcon: string; // site icon URL
  itemsPerPage: number; // number of items per page
  createdAt: string;
  updatedAt: string;
}

// Post types
// The id and authorId fields are uint in the backend model; the
// orval/huma generated client surfaces them as number. Keeping
// the legacy type in sync prevents string<->number comparison
// errors at call sites.
export interface Post {
  id: number;
  slug: string;
  title: string;
  content: string;
  excerpt: string;
  category: number | string;
  categoryInfo?: Category;
  tags: string[];
  coverImage: string | null;
  status: "published" | "draft" | "pending" | "rejected";
  authorId: number;
  author?: User;
  createdAt: string;
  updatedAt: string;
  viewCount?: number;
  rejectionReason?: string;
  likesCount?: number;
  isLiked?: boolean;
  commentsCount?: number;
}

// Category types
export interface Category {
  id: string;
  name: string;
  description: string;
  postCount?: number;
  createdAt?: string;
}

// Tag types
export interface Tag {
  id: string;
  name: string;
  postCount?: number;
  createdAt?: string;
}

// Comment types
// userId is uint in the backend model; the orval/huma types
// surface it as number. The legacy type used string — keeping
// it as number avoids string<->number comparison errors at
// call sites (e.g. PostDetailPage line 593).
export interface Comment {
  id: number;
  postId: number;
  userId: number;
  author?: User;
  content: string;
  parentId: number | null;
  status: "pending" | "published" | "rejected";
  moderationReason?: string;
  createdAt: string;
  updatedAt: string;
}

// Comment moderation config types
export interface CommentModerationConfig {
  id: string;
  manualReviewEnabled: boolean;
  keywordFilterEnabled: boolean;
  llmReviewEnabled: boolean;
  modelProvider: string;
  apiKey: string;
  apiEndpoint: string;
  modelName: string;
  moderationPrompt: string;
  blockKeywords: string;
  createdAt: string;
  updatedAt: string;
}

// AI model info types
export interface AIModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}

// AI config types
export interface AIConfig {
  id: string;
  enabled: boolean;
  provider: string; // provider (openai, azure, etc.)
  apiEndpoint: string; // API endpoint URL
  apiKey: string; // API key (only used for setting, never returned on fetch)
  modelName: string; // model name
  createdAt: string;
  updatedAt: string;
}

// Media file types
export interface MediaFile {
  id: string;
  url: string;
  type: "image" | "video";
  size: number;
  createdAt?: string;
}

// Pagination types
export interface Pagination {
  total: number;
  page: number;
  totalPages: number;
  limit: number;
}

// API response types
export interface ApiResponse<T> {
  message?: string;
  data?: T;
  error?: string;
}

// Login/register response
export interface AuthResponse {
  message: string;
  user: User;
  token?: string; // may be omitted when registration requires email verification
  email_verified?: boolean;
  requires_verification?: boolean;
}

// Post list response
export interface PostsResponse {
  posts: Post[];
  pagination: Pagination;
}

// Comment list response
export interface CommentsResponse {
  comments: Comment[];
}

// Like response
export interface LikeResponse {
  message: string;
  isLiked: boolean;
  likesCount: number;
}

// Upload response
export interface UploadResponse {
  message: string;
  file?: MediaFile;
  files?: MediaFile[];
  errors?: string[];
}

// Stats response
export interface StatsResponse {
  stats: {
    posts: number;
    users: number;
    comments: number;
    categories: number;
    tags: number;
  };
}
