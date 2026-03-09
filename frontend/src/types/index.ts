// 用户类型
export interface User {
  id: string;
  username: string;
  email: string;
  role: 'super_admin' | 'admin' | 'author' | 'contributor' | 'guest';
  avatar: string | null;
  emailVerified?: boolean;
  createdAt?: string;
  birthday?: string;
  bio?: string;
  // 隐私设置
  profile_visibility?: 'public' | 'private';
  hide_email?: boolean;
  hide_birthday?: boolean;
  hide_bio?: boolean;
}

// SMTP 配置类型
export interface SMTPConfig {
  id: string;
  enabled: boolean;
  host: string;
  port: number;
  username: string;
  password: string; // 仅用于设置，获取时不返回
  fromEmail: string;
  fromName: string;
  testEmail: string; // 测试邮件收件人邮箱
  createdAt: string;
  updatedAt: string;
}

// 通用设置类型
export interface GeneralSettings {
  id: string;
  captchaEnabled: boolean;      // 是否启用滑块验证
  registrationEnabled: boolean; // 是否允许注册
  siteName: string;             // 网站名称
  siteDescription: string;      // 网站描述
  itemsPerPage: number;         // 每页显示数量
  createdAt: string;
  updatedAt: string;
}

// 文章类型
export interface Post {
  id: string;
  title: string;
  content: string;
  excerpt: string;
  category: string;
  categoryInfo?: Category;
  tags: string[];
  coverImage: string | null;
  status: 'published' | 'draft' | 'pending' | 'rejected';
  authorId: string;
  author?: User;
  createdAt: string;
  updatedAt: string;
  viewCount?: number;
  rejectionReason?: string;
  likesCount?: number;
  isLiked?: boolean;
  commentsCount?: number;
}

// 分类类型
export interface Category {
  id: string;
  name: string;
  description: string;
  createdAt?: string;
}

// 标签类型
export interface Tag {
  id: string;
  name: string;
  createdAt?: string;
}

// 评论类型
export interface Comment {
  id: string;
  postId: string;
  userId: string;
  author?: User;
  content: string;
  parentId: string | null;
  createdAt: string;
  updatedAt: string;
}

// 评论审核配置类型
export interface CommentModerationConfig {
  id: string;
  enabled: boolean;
  modelProvider: string;
  apiKey: string;
  apiEndpoint: string;
  modelName: string;
  moderationPrompt: string;
  blockKeywords: string;
  autoApproveEnabled: boolean;
  minScoreThreshold: number;
  createdAt: string;
  updatedAt: string;
}

// AI 模型信息类型
export interface AIModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}

// AI 配置类型
export interface AIConfig {
  id: string;
  enabled: boolean;
  provider: string;       // 提供商 (openai, azure, etc.)
  apiEndpoint: string;    // API端点URL
  apiKey: string;         // API密钥（仅用于设置，获取时不返回）
  modelName: string;      // 模型名称
  createdAt: string;
  updatedAt: string;
}

// 媒体文件类型
export interface MediaFile {
  id: string;
  url: string;
  type: 'image' | 'video';
  originalName: string;
  size: number;
  createdAt?: string;
}

// 分页类型
export interface Pagination {
  total: number;
  page: number;
  totalPages: number;
  limit: number;
}

// API响应类型
export interface ApiResponse<T> {
  message?: string;
  data?: T;
  error?: string;
}

// 登录/注册响应
export interface AuthResponse {
  message: string;
  user: User;
  token?: string; // 注册时如果需要邮箱验证，可能不返回 token
  email_verified?: boolean;
  requires_verification?: boolean;
}

// 文章列表响应
export interface PostsResponse {
  posts: Post[];
  pagination: Pagination;
}

// 评论列表响应
export interface CommentsResponse {
  comments: Comment[];
}

// 点赞响应
export interface LikeResponse {
  message: string;
  isLiked: boolean;
  likesCount: number;
}

// 上传响应
export interface UploadResponse {
  message: string;
  file?: MediaFile;
  files?: MediaFile[];
  errors?: string[];
}

// 统计响应
export interface StatsResponse {
  stats: {
    posts: number;
    users: number;
    comments: number;
    categories: number;
    tags: number;
  };
}