// User types
export interface User {
  id: string | number;
  username: string;
  email: string;
  role: string;
  avatar?: string | null;
  emailVerified?: boolean;
  createdAt?: string;
  birthday?: string;
  bio?: string;
  profile_visibility?: string;
  hide_email?: boolean;
  hide_birthday?: boolean;
  hide_bio?: boolean;
}

// SMTP config types
export interface SMTPConfig {
  id?: string | number;
  enabled?: boolean;
  host?: string;
  port?: number;
  username?: string;
  password?: string;
  fromEmail?: string;
  fromName?: string;
  testEmail?: string;
  createdAt?: string;
  updatedAt?: string;
}

// General settings types
export interface GeneralSettings {
  id?: string | number;
  captchaEnabled?: boolean;
  registrationEnabled?: boolean;
  allowGuestViewPosts?: boolean;
  siteName?: string;
  siteDescription?: string;
  siteIcon?: string;
  itemsPerPage?: number;
  siteLanguage?: string;
  createdAt?: string;
  updatedAt?: string;
}

// Post types
export interface Post {
  id: string | number;
  slug?: string;
  title?: string;
  content?: string;
  excerpt?: string;
  category?: string;
  categoryInfo?: Category;
  tags?: string[];
  coverImage?: string | null;
  status?: string;
  authorId?: string | number;
  author?: User;
  createdAt?: string;
  updatedAt?: string;
  viewCount?: number;
  rejectionReason?: string;
  likesCount?: number;
  isLiked?: boolean;
  commentsCount?: number;
}

// Category types
export interface Category {
  id: string | number;
  name?: string;
  description?: string;
  postCount?: number;
  createdAt?: string;
}

// Tag types
export interface Tag {
  id: string | number;
  name?: string;
  postCount?: number;
  createdAt?: string;
}

// Comment types
export interface Comment {
  id: string | number;
  postId?: string | number;
  userId?: string | number;
  author?: User;
  content?: string;
  parentId?: string | number | null;
  moderationReason?: string;
  createdAt?: string;
  updatedAt?: string;
}

// Comment moderation config types
export interface CommentModerationConfig {
  id?: string | number;
  manualReviewEnabled?: boolean;
  keywordFilterEnabled?: boolean;
  llmReviewEnabled?: boolean;
  modelProvider?: string;
  apiKey?: string;
  apiEndpoint?: string;
  modelName?: string;
  moderationPrompt?: string;
  blockKeywords?: string;
  createdAt?: string;
  updatedAt?: string;
}

// AI model info types
export interface AIModel {
  id: string;
  object?: string;
  created?: number;
  owned_by?: string;
}

// AI config types
export interface AIConfig {
  id?: string | number;
  enabled?: boolean;
  provider?: string;
  apiEndpoint?: string;
  apiKey?: string;
  modelName?: string;
  createdAt?: string;
  updatedAt?: string;
}

// Media file types
export interface MediaFile {
  id: string | number;
  url?: string;
  type?: string;
  size?: number;
  createdAt?: string;
}

// Pagination types
export interface Pagination {
  total?: number;
  page?: number;
  totalPages?: number;
  limit?: number;
}

// API response types
export interface ApiResponse<T> {
  message?: string;
  data?: T;
  error?: string;
}

// Login/register response
export interface AuthResponse {
  message?: string;
  user?: User;
  token?: string;
  email_verified?: boolean;
  requires_verification?: boolean;
}

// Post list response
export interface PostsResponse {
  posts?: Post[];
  pagination?: Pagination;
}

// Comment list response
export interface CommentsResponse {
  comments?: Comment[];
}

// Like response
export interface LikeResponse {
  message?: string;
  isLiked?: boolean;
  likesCount?: number;
}

// Upload response
export interface UploadResponse {
  message?: string;
  file?: MediaFile;
  files?: MediaFile[];
  errors?: string[];
}

// Stats response
export interface StatsResponse {
  stats?: {
    posts?: number;
    users?: number;
    comments?: number;
    categories?: number;
    tags?: number;
  };
}
