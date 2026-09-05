import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios";
import type {
  SMTPConfig,
  GeneralSettings,
  CommentModerationConfig,
  AIConfig,
  AIModel,
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  RegisterResponse,
  UpdateProfileRequest,
  UserResponse,
  ChangePasswordRequest,
  MessageResponse,
  UpdateSettingsRequest,
  UpdateEmailRequest,
  VerificationStatusResponse,
  VerifyEmailResponse,
  RequestPasswordResetRequest,
  ResendVerificationRequest,
  ResetPasswordRequest,
  PostsResponse,
  PostResponse,
  CreatePostRequest,
  UpdatePostRequest,
  PostMutationResponse,
  PostListResponse,
  LikeResponse,
  CategoryListResponse,
  CreateCategoryRequest,
  CategoryCreateResponse,
  TagListResponse,
  CreateTagRequest,
  TagCreateResponse,
  CommentsResponse,
  CreateCommentRequest,
  CommentCreateResponse,
  CommentDeleteResponse,
  CommentModerationUpdateResponse,
  UpdateCommentModerationConfigRequest,
  LLMTestResponse,
  UploadResponse,
  FilesResponse,
  StatsResponse,
  ThemesResponse,
  UpdateSMTPConfigRequest,
  SMTPConfigUpdateResponse,
  SMTPTestResponse,
  UpdateGeneralSettingsRequest,
  GeneralSettingsUpdateResponse,
  UpdateAIConfigRequest,
  AIConfigUpdateResponse,
  UpdateThemeConfigRequest,
  ThemeUpdateResponse,
  NotificationsResponse,
  UnreadCountResponse,
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
  register: (data: RegisterRequest) =>
    api.post<RegisterResponse>("/auth/register", data),

  login: (data: LoginRequest) => api.post<LoginResponse>("/auth/login", data),

  getMe: () => api.get<UserResponse>("/auth/me"),

  updateProfile: (data: UpdateProfileRequest) =>
    api.put<UserResponse>("/auth/profile", data),

  changePassword: (data: ChangePasswordRequest) =>
    api.put<MessageResponse>("/auth/password", data),

  updateEmail: (data: UpdateEmailRequest) => api.put("/auth/email", data),

  updateSettings: (data: UpdateSettingsRequest) =>
    api.put<UserResponse>("/auth/settings", data),

  getVerificationStatus: () =>
    api.get<VerificationStatusResponse>("/auth/verification-status"),

  verifyEmail: (token: string) =>
    api.get<VerifyEmailResponse>(`/verify-email?token=${token}`),

  requestPasswordReset: (data: RequestPasswordResetRequest) =>
    api.post<MessageResponse>("/auth/request-password-reset", data),

  resendVerification: (data: ResendVerificationRequest) =>
    api.post<MessageResponse>("/auth/resend-verification", data),

  resetPassword: (data: ResetPasswordRequest) =>
    api.post<MessageResponse>("/auth/reset-password", data),
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

  getPost: (slug: string) => api.get<PostResponse>(`/posts/${slug}`),

  getPostById: (id: string | number) =>
    api.get<PostResponse>(`/posts/by-id/${id}`),

  createPost: (data: CreatePostRequest) =>
    api.post<PostMutationResponse>("/posts", data),

  updatePost: (id: string | number, data: UpdatePostRequest) =>
    api.put<PostMutationResponse>(`/posts/${id}`, data),

  deletePost: (id: string | number) =>
    api.delete<MessageResponse>(`/posts/${id}`),

  getMyPosts: (params?: { page?: number; limit?: number; status?: string }) =>
    api.get<PostsResponse>("/posts/user/my-posts", { params }),

  getDraftPosts: (params?: { page?: number; limit?: number }) =>
    api.get<PostsResponse>("/posts/drafts", { params }),

  getUserPosts: (
    userId: string | number,
    params?: { page?: number; limit?: number },
  ) => api.get<PostsResponse>(`/posts/user/${userId}`, { params }),
};

// Category-related APIs
export const categoriesApi = {
  getCategories: () => api.get<CategoryListResponse>("/categories"),

  createCategory: (data: CreateCategoryRequest) =>
    api.post<CategoryCreateResponse>("/categories", data),

  deleteCategory: (id: string | number) =>
    api.delete<MessageResponse>(`/categories/${id}`),
};

// Tag-related APIs
export const tagsApi = {
  getTags: () => api.get<TagListResponse>("/tags"),

  createTag: (data: CreateTagRequest) =>
    api.post<TagCreateResponse>("/tags", data),

  deleteTag: (id: string | number) =>
    api.delete<MessageResponse>(`/tags/${id}`),
};

// Comment-related APIs
export const commentsApi = {
  getComments: (postId: string | number) =>
    api.get<CommentsResponse>(`/comments/post/${postId}`),

  createComment: (data: CreateCommentRequest) =>
    api.post<CommentCreateResponse>("/comments", data),

  deleteComment: (id: string | number) =>
    api.delete<CommentDeleteResponse>(`/comments/${id}`),
};

// Like-related APIs
export const likesApi = {
  toggleLike: (postId: string | number) =>
    api.post<LikeResponse>(`/likes/${postId}`),

  getLikeStatus: (postId: string | number) =>
    api.get<LikeResponse>(`/likes/${postId}`),
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

  getMyFiles: () => api.get<FilesResponse>("/upload/my-files"),

  deleteFile: (id: string | number) =>
    api.delete<MessageResponse>(`/upload/${id}`),
};

// Stats-related APIs
export const statsApi = {
  getStats: () => api.get<StatsResponse>("/stats"),

  getPopularPosts: (limit?: number) =>
    api.get<PostListResponse>("/stats/popular-posts", { params: { limit } }),

  getLatestPosts: (limit?: number) =>
    api.get<PostListResponse>("/stats/latest-posts", { params: { limit } }),
};

// SMTP config-related APIs
export const configApi = {
  getSMTPConfig: () => api.get<SMTPConfig>("/config/smtp"),

  updateSMTPConfig: (data: UpdateSMTPConfigRequest) =>
    api.put<SMTPConfigUpdateResponse>("/config/smtp", data),

  testSMTP: () => api.post<SMTPTestResponse>("/config/smtp/test"),

  // General settings-related APIs
  getGeneralSettings: () => api.get<GeneralSettings>("/config/general"),

  updateGeneralSettings: (data: UpdateGeneralSettingsRequest) =>
    api.put<GeneralSettingsUpdateResponse>("/config/general", data),

  // Comment moderation config-related APIs
  getCommentModerationConfig: () =>
    api.get<CommentModerationConfig>("/moderation/comments/config"),

  updateCommentModerationConfig: (data: UpdateCommentModerationConfigRequest) =>
    api.put<CommentModerationUpdateResponse>(
      "/moderation/comments/config",
      data,
    ),

  testCommentModeration: () =>
    api.post<LLMTestResponse>("/moderation/comments/config/test"),

  // AI config-related APIs
  getAIConfig: () => api.get<AIConfig>("/config/ai"),

  updateAIConfig: (data: UpdateAIConfigRequest) =>
    api.put<AIConfigUpdateResponse>("/config/ai", data),

  testAI: () => api.post<LLMTestResponse>("/config/ai/test"),

  // AI model-related APIs
  getAIModels: () =>
    api.get<{ message: string; models: AIModel[] }>("/config/ai/models"),

  // Theme-related APIs
  getThemes: () => api.get<ThemesResponse>("/themes"),

  updateThemeConfig: (activeTheme: string) =>
    api.put<ThemeUpdateResponse>("/config/theme", {
      activeTheme,
    } satisfies UpdateThemeConfigRequest),
};

// Notification-related APIs
export const notificationsApi = {
  getNotifications: (params?: {
    page?: number;
    limit?: number;
    type?: string;
    is_read?: string;
  }) => api.get<NotificationsResponse>("/notifications", { params }),

  getUnreadCount: () =>
    api.get<UnreadCountResponse>("/notifications/unread-count"),

  markAsRead: (id: string | number) =>
    api.put<MessageResponse>(`/notifications/${id}/read`),

  markAllAsRead: () => api.put<MessageResponse>("/notifications/read-all"),

  deleteNotification: (id: string | number) =>
    api.delete<MessageResponse>(`/notifications/${id}`),
};

export default api;
