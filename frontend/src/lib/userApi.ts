import api from './api';
import type { User, Pagination } from '@/types';

// 用户列表响应类型
export interface UsersResponse {
  users: User[];
  pagination: Pagination;
}

// 创作者申请响应类型
export interface CreatorApplicationResponse {
  message: string;
  applicationId?: string;
}

// 创作者申请列表响应类型
export interface CreatorApplicationsResponse {
  applications: CreatorApplication[];
  pagination: Pagination;
}

// 创作者申请类型
export interface CreatorApplication {
  id: string;
  userId: string;
  username: string;
  email: string;
  currentRole: string;
  status: 'pending' | 'approved' | 'rejected';
  reason?: string;
  createdAt: string;
  updatedAt: string;
}

// 获取用户列表
export const getUsers = (params?: { 
  page?: number; 
  limit?: number; 
  search?: string;
}) =>
  api.get<UsersResponse>('/users', { params });

// 更新用户角色
export const updateUserRole = (id: string, role: string) =>
  api.put<{ message: string; user: User }>(`/users/${id}/role`, { role });

// 删除用户
export const deleteUser = (id: string) =>
  api.delete<{ message: string }>(`/users/${id}`);

// 申请成为创作者
export const applyForCreator = (reason?: string) =>
  api.post<CreatorApplicationResponse>('/users/apply-creator', { reason });

// 获取创作者申请列表（管理员用）
export const getCreatorApplications = (params?: { 
  page?: number; 
  limit?: number;
  status?: string;
}) =>
  api.get<CreatorApplicationsResponse>('/users/creator-applications', { params });

// 审核创作者申请
export const reviewCreatorApplication = (applicationId: string, action: 'approve' | 'reject', reason?: string) =>
  api.put<{ message: string }>(`/users/creator-applications/${applicationId}/review`, { action, reason });