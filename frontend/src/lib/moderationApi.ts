import api from './api';
import type { Post, PostsResponse } from '@/types';

// 获取待审核文章列表
export const getPendingPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  api.get<PostsResponse>('/moderation/pending', { params });

// 获取已通过文章列表
export const getApprovedPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  api.get<PostsResponse>('/moderation/approved', { params });

// 获取已拒绝文章列表
export const getRejectedPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  api.get<PostsResponse>('/moderation/rejected', { params });

// 审核通过文章
export const approvePost = (id: string) =>
  api.put<{ message: string; post: Post }>(`/moderation/approve/${id}`);

// 拒绝文章
export const rejectPost = (id: string, rejectionReason?: string) =>
  api.put<{ message: string; post: Post }>(`/moderation/reject/${id}`, { rejectionReason });

// 重新提交文章审核
export const resubmitPost = (id: string) =>
  api.put<{ message: string; post: Post }>(`/moderation/resubmit/${id}`);