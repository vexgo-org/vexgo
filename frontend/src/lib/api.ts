import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios";
import type {
  User,
  Post,
  Category,
  Tag,
  Comment,
  MediaFile,
  AuthResponse,
  PostsResponse,
  CommentsResponse,
  LikeResponse,
  UploadResponse,
  StatsResponse,
  SMTPConfig,
  GeneralSettings,
  CommentModerationConfig,
  AIConfig,
  AIModel,
} from "@/types";

const API_BASE_URL = import.meta.env.VITE_API_URL || "/api";

// Create an axios instance
const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    "Content-Type": "application/json",
  },
  timeout: 30000,
});

// Request interceptor - attach the auth token
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem("token");
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error: AxiosError) => {
    return Promise.reject(error);
  },
);

// Response interceptor - handle errors
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      // Check whether this is an auth-related endpoint (login, register, etc.); 401s from these should not redirect automatically
      const isAuthEndpoint =
        error.config?.url?.includes("/auth/") ||
        error.config?.url?.includes("/verify-email");

      // Only redirect to the login page for 401 errors from non-auth endpoints
      if (!isAuthEndpoint) {
        localStorage.removeItem("token");
        localStorage.removeItem("user");
        window.location.href = "/login";
      }
    }
    return Promise.reject(error);
  },
);

// Auth-related APIs
export const authApi = {
  register: (data: { username: string; email: string; password: string }) =>
    api.post<AuthResponse>("/auth/register", data),

  login: (data: { email: string; password: string }) =>
    api.post<AuthResponse>("/auth/login", data),

  getMe: () => api.get<{ user: User }>("/auth/me"),

  updateProfile: (data: {
    username?: string;
    avatar?: string;
    birthday?: string;
    bio?: string;
  }) => api.put("/auth/profile", data),

  changePassword: (data: { oldPassword: string; newPassword: string }) =>
    api.put("/auth/password", data),

  updateEmail: (data: { email: string }) => api.put("/auth/email", data),

  updateSettings: (data: {
    profile_visibility?: string;
    hide_email?: boolean;
    hide_birthday?: boolean;
    hide_bio?: boolean;
  }) => api.put("/auth/settings", data),

  getVerificationStatus: () =>
    api.get<{ email_verified: boolean; email: string }>(
      "/auth/verification-status",
    ),

  verifyEmail: (token: string) =>
    api.get<{ message: string; require_relogin?: boolean; new_email?: string }>(
      `/verify-email?token=${token}`,
    ),

  requestPasswordReset: (data: { email: string }) =>
    api.post<{ message: string }>("/auth/request-password-reset", data),

  resendVerification: (data: { email: string }) =>
    api.post<{ message: string }>("/auth/resend-verification", data),

  resetPassword: (data: { token: string; password: string }) =>
    api.post<{ message: string }>("/auth/reset-password", data),
};

// Post-related APIs
export const postsApi = {
  getPosts: (params?: {
    page?: number;
    limit?: number;
    category?: string;
    tag?: string;
    search?: string;
    status?: string;
  }) => api.get<PostsResponse>("/posts", { params }),

  getPost: (slug: string) => api.get<{ post: Post }>(`/posts/${slug}`),

  getPostById: (id: string) => api.get<{ post: Post }>(`/posts/by-id/${id}`),

  createPost: (data: {
    title: string;
    content: string;
    category: string;
    tags?: string[];
    excerpt?: string;
    coverImage?: string;
    status?: "published" | "draft" | "pending";
  }) => api.post<{ message: string; post: Post }>("/posts", data),

  updatePost: (id: string, data: Partial<Post>) =>
    api.put<{ message: string; post: Post }>(`/posts/${id}`, data),

  deletePost: (id: string) => api.delete<{ message: string }>(`/posts/${id}`),

  getMyPosts: (params?: { page?: number; limit?: number; status?: string }) =>
    api.get<PostsResponse>("/posts/user/my-posts", { params }),

  getDraftPosts: (params?: { page?: number; limit?: number }) =>
    api.get<PostsResponse>("/posts/drafts", { params }),

  getUserPosts: (userId: string, params?: { page?: number; limit?: number }) =>
    api.get<PostsResponse>(`/posts/user/${userId}`, { params }),
};

// Category-related APIs
export const categoriesApi = {
  getCategories: () => api.get<{ categories: Category[] }>("/categories"),

  createCategory: (data: { name: string; description?: string }) =>
    api.post<{ message: string; category: Category }>("/categories", data),
};

// Tag-related APIs
export const tagsApi = {
  getTags: () => api.get<{ tags: Tag[] }>("/tags"),
};

// Comment-related APIs
export const commentsApi = {
  getComments: (postId: string) =>
    api.get<CommentsResponse>(`/comments/post/${postId}`),

  createComment: (data: {
    postId: string;
    content: string;
    parentId?: string;
  }) =>
    api.post<{ message: string; comment: Comment; commentsCount?: number }>(
      "/comments",
      data,
    ),

  deleteComment: (id: string) =>
    api.delete<{ message: string; commentsCount?: number }>(`/comments/${id}`),
};

// Like-related APIs
export const likesApi = {
  toggleLike: (postId: string) => api.post<LikeResponse>(`/likes/${postId}`),

  getLikeStatus: (postId: string) =>
    api.get<{ postId: string; likesCount: number; isLiked: boolean }>(
      `/likes/${postId}`,
    ),
};

// Upload-related APIs
export const uploadApi = {
  uploadFile: (file: File, onProgress?: (progress: number) => void) => {
    const formData = new FormData();
    formData.append("file", file);

    return api.post<UploadResponse>("/upload/file", formData, {
      headers: {
        "Content-Type": "multipart/form-data",
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round(
            (progressEvent.loaded * 100) / progressEvent.total,
          );
          onProgress(progress);
        }
      },
    });
  },

  uploadFiles: (files: File[], onProgress?: (progress: number) => void) => {
    const formData = new FormData();
    files.forEach((file) => formData.append("files", file));

    return api.post<UploadResponse>("/upload/files", formData, {
      headers: {
        "Content-Type": "multipart/form-data",
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round(
            (progressEvent.loaded * 100) / progressEvent.total,
          );
          onProgress(progress);
        }
      },
    });
  },

  getMyFiles: () => api.get<{ files: MediaFile[] }>("/upload/my-files"),

  deleteFile: (id: string) => api.delete<{ message: string }>(`/upload/${id}`),
};

// Stats-related APIs
export const statsApi = {
  getStats: () => api.get<StatsResponse>("/stats"),

  getPopularPosts: (limit?: number) =>
    api.get<{ posts: Post[] }>("/stats/popular-posts", { params: { limit } }),

  getLatestPosts: (limit?: number) =>
    api.get<{ posts: Post[] }>("/stats/latest-posts", { params: { limit } }),
};

// SMTP config-related APIs
export const configApi = {
  getSMTPConfig: () => api.get<SMTPConfig>("/config/smtp"),

  updateSMTPConfig: (data: Partial<SMTPConfig>) =>
    api.put<{ message: string; smtpConfig: SMTPConfig }>("/config/smtp", data),

  testSMTP: () =>
    api.post<{ message: string; to: string }>("/config/smtp/test"),

  // General settings-related APIs
  getGeneralSettings: () => api.get<GeneralSettings>("/config/general"),

  updateGeneralSettings: (data: Partial<GeneralSettings>) =>
    api.put<{ message: string; generalSettings: GeneralSettings }>(
      "/config/general",
      data,
    ),

  // Comment moderation config-related APIs
  getCommentModerationConfig: () =>
    api.get<CommentModerationConfig>("/moderation/comments/config"),

  updateCommentModerationConfig: (data: Partial<CommentModerationConfig>) =>
    api.put<{ message: string; config: CommentModerationConfig }>(
      "/moderation/comments/config",
      data,
    ),

  // AI config-related APIs
  getAIConfig: () => api.get<AIConfig>("/config/ai"),

  updateAIConfig: (data: Partial<AIConfig>) =>
    api.put<{ message: string; aiConfig: AIConfig }>("/config/ai", data),

  testAI: () =>
    api.post<{ message: string; response: string }>("/config/ai/test"),

  // AI model-related APIs
  getAIModels: () =>
    api.get<{ message: string; models: AIModel[] }>("/config/ai/models"),

  // Theme-related APIs
  getThemes: () =>
    api.get<{
      themes: Array<{
        id: string;
        name: string;
        author: string;
        version: string;
        description: string;
        url: string;
      }>;
    }>("/themes"),
};

// Notification-related APIs
export const notificationsApi = {
  getNotifications: (params?: {
    page?: number;
    limit?: number;
    type?: string;
    is_read?: string;
  }) =>
    api.get<{ notifications: unknown[]; pagination: unknown }>(
      "/notifications",
      {
        params,
      },
    ),

  getUnreadCount: () =>
    api.get<{ unreadCount: number }>("/notifications/unread-count"),

  markAsRead: (id: string) =>
    api.put<{ message: string }>(`/notifications/${id}/read`),

  markAllAsRead: () => api.put<{ message: string }>("/notifications/read-all"),

  deleteNotification: (id: string) =>
    api.delete<{ message: string }>(`/notifications/${id}`),
};

export default api;
