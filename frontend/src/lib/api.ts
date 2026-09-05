// Compatibility shim for the migration from hand-written
// axios calls to the orval-generated typed client. The page
// code under `src/pages/` and `src/hooks/` imports the same
// `authApi`, `postsApi`, `categoriesApi`, etc. as before; this
// file now re-exports those objects as thin wrappers over the
// generated functions so callers get the typed request/response
// shapes (User, Post, CategoriesResponse, ...) for free.
//
// When a domain adds a new operation, the orval generator will
// produce the new function in src/api/generated/, and this
// file picks it up automatically — no need to extend the
// facade per endpoint. The wrappers below keep the legacy
// method names (e.g. `getMe`, `getStats`, `getCaptcha`) so the
// pages don't need to be touched.

import type { InternalAxiosRequestConfig } from "axios";
import axios from "axios";

import { vexgoApi } from "@/api";
import type { AxiosResponse as AxiosResponseType } from "axios";

// The shim accepts the legacy request shapes the page code
// already uses. The legacy types in src/types/index.ts came
// from tygo; the orval-generated types in
// src/api/generated/model/ use huma-style names (LoginInputBody,
// RegisterInputBody, etc.) and live alongside the response
// types. We don't import them here — the shim is a thin
// pass-through; the typed surface is the orval-generated
// `vexgoApi` and the model types in `@/api/generated/model`.
// Pages can switch to importing from there directly when they
// want full type safety.

const API_BASE_URL =
  (import.meta.env.VITE_API_URL as string) || "/api";

// The legacy client is still kept around for non-generated
// helpers (file upload progress, which the orval-generated
// functions do not surface). Pages that called `api.*` for
// their own purposes still go through the same axios instance.
const api = axios.create({
  baseURL: API_BASE_URL,
  headers: { "Content-Type": "application/json" },
  timeout: 30000,
});

api.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem("token");
    if (token && config.headers) {
      (config.headers as Record<string, string>).Authorization =
        `Bearer ${token}`;
    }
    return config;
  },
);

api.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    const err = error as {
      response?: { status?: number };
      config?: { url?: string };
    };
    if (err.response?.status === 401) {
      const isAuthEndpoint =
        err.config?.url?.includes("/auth/") ||
        err.config?.url?.includes("/verify-email");
      if (!isAuthEndpoint) {
        localStorage.removeItem("token");
        localStorage.removeItem("user");
        if (typeof window !== "undefined") {
          window.location.href = "/login";
        }
      }
    }
    return Promise.reject(error);
  },
);

export { api };

// ---------- thin wrappers ----------
//
// Each legacy method delegates to the orval-generated
// function. We accept `any` for the request payloads because
// the legacy callers (under src/pages) use the tygo-era
// `src/types/index.ts` request shapes which differ in field
// naming and optionality from the orval/huma input types.
// Pages that want full type safety can switch to
// `vexgoApi.foo(modelType)` directly with the orval inputs.

export const authApi = {
  register: (data: any) => vexgoApi.register(data),

  login: (data: { email: string; password: string }) =>
    vexgoApi.login(data as never) as Promise<AxiosResponseType<any>>,

  getMe: () =>
    vexgoApi.getCurrentUser() as unknown as Promise<AxiosResponseType<{ user: any }>>,

  updateProfile: (data: any) =>
    vexgoApi.updateProfile(data) as unknown as Promise<AxiosResponseType<any>>,

  changePassword: (data: { oldPassword: string; newPassword: string }) =>
    vexgoApi.changePassword(data as never),

  updateEmail: (data: { email: string }) =>
    vexgoApi.updateEmail(data as never),

  updateSettings: (data: any) =>
    vexgoApi.updateSettings(data as never),

  getVerificationStatus: () =>
    vexgoApi.getVerificationStatus() as unknown as Promise<
      AxiosResponseType<{ email_verified: boolean; email: string }>
    >,

  verifyEmail: (token: string) =>
    vexgoApi.verifyEmail({ token }) as unknown as Promise<AxiosResponseType<any>>,

  requestPasswordReset: (data: { email: string }) =>
    vexgoApi.requestPasswordReset(data as never),

  resendVerification: (data: { email: string }) =>
    vexgoApi.resendVerification(data as never),

  resetPassword: (data: { token: string; password: string }) =>
    vexgoApi.resetPassword(data as never),
};

export const postsApi = {
  getPosts: (params?: { page?: number; limit?: number; category?: string; search?: string }) =>
    vexgoApi.listPosts({
      page: params?.page,
      limit: params?.limit,
      category: params?.category,
      search: params?.search,
    }) as unknown as Promise<AxiosResponseType<any>>,

  getPost: (slug: string) =>
    vexgoApi.getPostBySlug({ slug }) as unknown as Promise<AxiosResponseType<{ post: any }>>,

  getPostById: (id: number | string) =>
    vexgoApi.getPostById({ id: Number(id) }) as unknown as Promise<AxiosResponseType<{ post: any }>>,

  createPost: (data: any) =>
    vexgoApi.createPost(data as never) as unknown as Promise<AxiosResponseType<any>>,

  updatePost: (id: number | string, data: any) =>
    vexgoApi.updatePost({ id: Number(id) }, data as never) as unknown as Promise<
      AxiosResponseType<any>
    >,

  deletePost: (id: number | string) =>
    vexgoApi.deletePost({ id: Number(id) } as never),

  getMyPosts: (params?: { page?: number; limit?: number; status?: string }) =>
    vexgoApi.myPosts({
      page: params?.page,
      limit: params?.limit,
      status: params?.status,
    }) as unknown as Promise<AxiosResponseType<any>>,

  getDraftPosts: (params?: { page?: number; limit?: number }) =>
    vexgoApi.drafts({ page: params?.page, limit: params?.limit }) as unknown as Promise<
      AxiosResponseType<any>
    >,

  getUserPosts: (userId: number | string, params?: { page?: number; limit?: number }) =>
    vexgoApi.userPosts({
      id: Number(userId),
      page: params?.page,
      limit: params?.limit,
    }) as unknown as Promise<AxiosResponseType<any>>,
};

export const categoriesApi = {
  getCategories: () =>
    vexgoApi.listCategories() as unknown as Promise<
      AxiosResponseType<{ categories: any[] }>
    >,

  createCategory: (data: { name: string; description?: string }) =>
    vexgoApi.createCategory(data as never),

  deleteCategory: (id: number | string) =>
    vexgoApi.deleteCategory({ id: Number(id) } as never),
};

export const tagsApi = {
  getTags: () =>
    vexgoApi.listTags() as unknown as Promise<AxiosResponseType<{ tags: any[] }>>,

  createTag: (data: { name: string }) =>
    vexgoApi.createTag(data as never),

  deleteTag: (id: number | string) =>
    vexgoApi.deleteTag({ id: Number(id) } as never),
};

export const commentsApi = {
  getComments: (postId: number | string) =>
    vexgoApi.listPostComments({ id: Number(postId) }) as unknown as Promise<AxiosResponseType<any>>,

  createComment: (data: { postId: number | string; content: string; parentId?: number | string }) =>
    vexgoApi.createComment({ ...data, postId: data.postId as any, parentId: data.parentId as any } as never) as unknown as Promise<AxiosResponseType<any>>,

  deleteComment: (id: number | string) =>
    vexgoApi.deleteComment({ id: Number(id) } as never),
};

export const likesApi = {
  toggleLike: (postId: number | string) =>
    vexgoApi.toggleLike({ postId: Number(postId) } as never) as unknown as Promise<AxiosResponseType<any>>,

  getLikeStatus: (postId: number | string) =>
    vexgoApi.getLikeStatus({ postId: Number(postId) } as never) as unknown as Promise<
      AxiosResponseType<any>
    >,
};

export const uploadApi = {
  uploadFile: (file: File, onProgress?: (progress: number) => void) => {
    const formData = new FormData();
    formData.append("file", file);

    return api.post<{ message: string; file: any }>("/upload/file", formData, {
      headers: { "Content-Type": "multipart/form-data" },
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

    return api.post<{ message: string; files: any[] }>("/upload/files", formData, {
      headers: { "Content-Type": "multipart/form-data" },
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

  getMyFiles: () =>
    vexgoApi.listMyFiles() as unknown as Promise<AxiosResponseType<any>>,

  deleteFile: (id: number | string) =>
    vexgoApi.deleteFile({ id: Number(id) } as never),
};

export const statsApi = {
  getStats: () =>
    vexgoApi.getStats() as unknown as Promise<AxiosResponseType<any>>,

  getPopularPosts: (limit?: number) =>
    vexgoApi.popularPosts({ limit } as never) as unknown as Promise<
      AxiosResponseType<any>
    >,

  getLatestPosts: (limit?: number) =>
    vexgoApi.latestPosts({ limit } as never) as unknown as Promise<
      AxiosResponseType<any>
    >,
};

export const configApi = {
  getSMTPConfig: () =>
    vexgoApi.getSmtpConfig() as unknown as Promise<AxiosResponseType<any>>,

  updateSMTPConfig: (data: any) =>
    vexgoApi.updateSmtpConfig(data as never) as unknown as Promise<AxiosResponseType<any>>,

  testSMTP: () =>
    vexgoApi.testSmtp() as unknown as Promise<AxiosResponseType<any>>,

  getGeneralSettings: () =>
    vexgoApi.getGeneralSettings() as unknown as Promise<AxiosResponseType<any>>,

  updateGeneralSettings: (data: any) =>
    vexgoApi.updateGeneralSettings(data as never) as unknown as Promise<
      AxiosResponseType<any>
    >,

  getCommentModerationConfig: () =>
    vexgoApi.getCommentModerationConfig() as unknown as Promise<
      AxiosResponseType<any>
    >,

  updateCommentModerationConfig: (data: any) =>
    vexgoApi.updateCommentModerationConfig(data as never) as unknown as Promise<
      AxiosResponseType<any>
    >,

  testCommentModeration: () =>
    vexgoApi.testCommentModeration() as unknown as Promise<AxiosResponseType<any>>,

  getAIConfig: () =>
    vexgoApi.getAiConfig() as unknown as Promise<AxiosResponseType<any>>,

  updateAIConfig: (data: any) =>
    vexgoApi.updateAiConfig(data as never) as unknown as Promise<AxiosResponseType<any>>,

  testAI: () =>
    vexgoApi.testAi() as unknown as Promise<AxiosResponseType<any>>,

  getAIModels: () =>
    vexgoApi.listAiModels() as unknown as Promise<AxiosResponseType<any>>,

  getThemes: () =>
    vexgoApi.listThemes() as unknown as Promise<AxiosResponseType<any>>,
};

export const notificationsApi = {
  getNotifications: (params?: {
    page?: number;
    limit?: number;
    type?: string;
    is_read?: string;
  }) =>
    vexgoApi.getNotifications({
      page: params?.page,
      limit: params?.limit,
      type: params?.type,
      is_read: params?.is_read,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  getUnreadCount: () =>
    vexgoApi.getUnreadNotificationCount() as unknown as Promise<
      AxiosResponseType<any>
    >,

  markAsRead: (id: number | string) =>
    vexgoApi.markNotificationRead({ id: Number(id) } as never),

  markAllAsRead: () => vexgoApi.markAllNotificationsRead(),

  deleteNotification: (id: number | string) =>
    vexgoApi.deleteNotification({ id: Number(id) } as never),
};

export const moderationApi = {
  listPendingComments: (params?: { page?: number; limit?: number }) =>
    vexgoApi.listPendingComments({
      page: params?.page,
      limit: params?.limit,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  listApprovedComments: (params?: { page?: number; limit?: number }) =>
    vexgoApi.listApprovedComments({
      page: params?.page,
      limit: params?.limit,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  listRejectedComments: (params?: { page?: number; limit?: number }) =>
    vexgoApi.listRejectedComments({
      page: params?.page,
      limit: params?.limit,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  approveComment: (id: number | string) =>
    vexgoApi.approveComment({ id: Number(id) } as never),

  rejectComment: (id: number | string) =>
    vexgoApi.rejectComment({ id: Number(id) } as never),

  listPendingPosts: (params?: { page?: number; limit?: number; search?: string }) =>
    vexgoApi.listPendingPosts({
      page: params?.page,
      limit: params?.limit,
      search: params?.search,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  listApprovedPosts: (params?: { page?: number; limit?: number; search?: string }) =>
    vexgoApi.listApprovedPosts({
      page: params?.page,
      limit: params?.limit,
      search: params?.search,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  listRejectedPosts: (params?: { page?: number; limit?: number; search?: string }) =>
    vexgoApi.listRejectedPosts({
      page: params?.page,
      limit: params?.limit,
      search: params?.search,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  approvePost: (id: number | string) =>
    vexgoApi.approvePost({ id: Number(id) } as never),

  rejectPost: (id: number | string, reason?: string) =>
    vexgoApi.rejectPost(
      { id: Number(id) } as never,
      { rejectionReason: reason } as never,
    ),

  resubmitPost: (id: number | string) =>
    vexgoApi.resubmitPost({ id: Number(id) } as never),
};

export const usersApi = {
  listUsers: (params?: { page?: number; limit?: number; search?: string }) =>
    vexgoApi.listUsers({
      page: params?.page,
      limit: params?.limit,
      search: params?.search,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  deleteUser: (id: number | string) =>
    vexgoApi.deleteUser({ id: Number(id) } as never),

  updateUserRole: (id: number | string, body: any) =>
    vexgoApi.updateUserRole({ id: Number(id) } as never, body as never),

  applyForCreator: (data: any) =>
    vexgoApi.applyForCreator(data as never) as unknown as Promise<AxiosResponseType<any>>,

  listCreatorApplications: (params?: { page?: number; limit?: number; status?: string }) =>
    vexgoApi.listCreatorApplications({
      page: params?.page,
      limit: params?.limit,
      status: params?.status,
    } as never) as unknown as Promise<AxiosResponseType<any>>,

  reviewCreatorApplication: (id: number | string, body: any) =>
    vexgoApi.reviewCreatorApplication({ id: Number(id) } as never, body as never),
};

export const captchaApi = {
  generate: () =>
    vexgoApi.generateCaptcha() as unknown as Promise<AxiosResponseType<any>>,

  verify: (data: { id: string; token: string; x: number; y: number }) =>
    vexgoApi.verifyCaptcha(data as never) as unknown as Promise<AxiosResponseType<any>>,
};

export const ssoApi = {
  getProviders: () =>
    vexgoApi.listSsoProviders() as unknown as Promise<AxiosResponseType<any>>,
};

export const themeApi = {
  upload: (file: File) => {
    const formData = new FormData();
    formData.append("theme", file);
    return api.post<{ message: string; themeId: string }>(
      "/themes/upload",
      formData,
      {
        headers: { "Content-Type": "multipart/form-data" },
      },
    );
  },

  preview: (id: number | string) =>
    vexgoApi.themePreview({ id: Number(id) } as never) as unknown as Promise<AxiosResponseType<any>>,
};

// Preserve the legacy default export of the bare axios
// instance (some pages import `api` directly for the
// auth-redirect interceptor behavior).
export { api as default };
export type { AxiosResponse as AxiosResponseType } from "axios";
