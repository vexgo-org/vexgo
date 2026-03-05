import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios';
import type {
  User, Post, Category, Tag, Comment, MediaFile,
  AuthResponse, PostsResponse, CommentsResponse,
  LikeResponse, UploadResponse, StatsResponse,
  SMTPConfig
} from '@/types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api';

// 创建axios实例
const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  },
  timeout: 30000
});

// 请求拦截器 - 添加认证令牌
api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('token');
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error: AxiosError) => {
    return Promise.reject(error);
  }
);

// 响应拦截器 - 处理错误
api.interceptors.response.use(
  (response) => response,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      // 未授权，清除token
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// 认证相关API
export const authApi = {
  register: (data: { username: string; email: string; password: string }) =>
    api.post<AuthResponse>('/auth/register', data),
  
  login: (data: { email: string; password: string }) =>
    api.post<AuthResponse>('/auth/login', data),
  
  getMe: () =>
    api.get<{ user: User }>('/auth/me'),
  
  updateProfile: (data: { username?: string; avatar?: string }) =>
    api.put('/auth/profile', data),
  
  changePassword: (data: { oldPassword: string; newPassword: string }) =>
    api.put('/auth/password', data),

  getVerificationStatus: () =>
    api.get<{ email_verified: boolean; email: string }>('/auth/verification-status'),
  
  resendVerificationEmail: () =>
    api.post<{ message: string }>('/auth/resend-verification')
};

// 文章相关API
export const postsApi = {
  getPosts: (params?: { 
    page?: number; 
    limit?: number; 
    category?: string; 
    tag?: string;
    search?: string;
    status?: string;
  }) =>
    api.get<PostsResponse>('/posts', { params }),
  
  getPost: (id: string) =>
    api.get<{ post: Post }>(`/posts/${id}`),
  
  createPost: (data: {
    title: string;
    content: string;
    category: string;
    tags?: string[];
    excerpt?: string;
    coverImage?: string;
    status?: 'published' | 'draft' | 'pending';
  }) =>
    api.post<{ message: string; post: Post }>('/posts', data),
  
  updatePost: (id: string, data: Partial<Post>) =>
    api.put<{ message: string; post: Post }>(`/posts/${id}`, data),
  
  deletePost: (id: string) =>
    api.delete<{ message: string }>(`/posts/${id}`),
  
  getMyPosts: (params?: { page?: number; limit?: number; status?: string }) =>
    api.get<PostsResponse>('/posts/user/my-posts', { params }),
  
  getDraftPosts: (params?: { page?: number; limit?: number }) =>
    api.get<PostsResponse>('/posts/drafts', { params })
};

// 分类相关API
export const categoriesApi = {
  getCategories: () =>
    api.get<{ categories: Category[] }>('/categories'),
  
  createCategory: (data: { name: string; description?: string }) =>
    api.post<{ message: string; category: Category }>('/categories', data)
};

// 标签相关API
export const tagsApi = {
  getTags: () =>
    api.get<{ tags: Tag[] }>('/tags')
};

// 评论相关API
export const commentsApi = {
  getComments: (postId: string) =>
    api.get<CommentsResponse>(`/comments/post/${postId}`),
  
  createComment: (data: { postId: string; content: string; parentId?: string }) =>
    api.post<{ message: string; comment: Comment }>('/comments', data),
  
  deleteComment: (id: string) =>
    api.delete<{ message: string }>(`/comments/${id}`)
};

// 点赞相关API
export const likesApi = {
  toggleLike: (postId: string) =>
    api.post<LikeResponse>(`/likes/${postId}`),
  
  getLikeStatus: (postId: string) =>
    api.get<{ postId: string; likesCount: number; isLiked: boolean }>(`/likes/${postId}`)
};

// 上传相关API
export const uploadApi = {
  uploadFile: (file: File, onProgress?: (progress: number) => void) => {
    const formData = new FormData();
    formData.append('file', file);
    
    return api.post<UploadResponse>('/upload/file', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          onProgress(progress);
        }
      }
    });
  },
  
  uploadFiles: (files: File[], onProgress?: (progress: number) => void) => {
    const formData = new FormData();
    files.forEach(file => formData.append('files', file));
    
    return api.post<UploadResponse>('/upload/files', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        if (onProgress && progressEvent.total) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total);
          onProgress(progress);
        }
      }
    });
  },
  
  getMyFiles: () =>
    api.get<{ files: MediaFile[] }>('/upload/my-files'),
  
  deleteFile: (id: string) =>
    api.delete<{ message: string }>(`/upload/${id}`)
};

// 统计相关API
export const statsApi = {
  getStats: () =>
    api.get<StatsResponse>('/stats'),
  
  getPopularPosts: (limit?: number) =>
    api.get<{ posts: Post[] }>('/stats/popular-posts', { params: { limit } }),
  
  getLatestPosts: (limit?: number) =>
    api.get<{ posts: Post[] }>('/stats/latest-posts', { params: { limit } })
};

// SMTP 配置相关API
export const configApi = {
  getSMTPConfig: () =>
    api.get<SMTPConfig>('/config/smtp'),
  
  updateSMTPConfig: (data: Partial<SMTPConfig>) =>
    api.put<{ message: string; smtpConfig: SMTPConfig }>('/config/smtp', data),
  
  testSMTP: () =>
    api.post<{ message: string; to: string }>('/config/smtp/test')
};

export default api;
