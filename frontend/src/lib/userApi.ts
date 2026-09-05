// User-domain API surface. Mirrors the shape of the
// legacy userApi.ts but delegates to the orval-generated
// `vexgoApi` so the request/response types stay in sync with
// the backend's huma schema.
//
// Pages that imported from `@/lib/userApi` (UserManagement,
// ApplyForCreator) continue to do so; this file re-exports
// the function names they use, now backed by the typed client.

import { vexgoApi } from "@/api";
import type { User, Pagination } from "@/types";

export interface UsersResponse {
  users: User[];
  pagination: Pagination;
}

export interface CreatorApplicationResponse {
  message: string;
  applicationId?: number;
}

export interface CreatorApplicationsResponse {
  applications: CreatorApplication[];
  pagination: Pagination;
}

export interface CreatorApplication {
  id: number;
  userId: number;
  username: string;
  email: string;
  currentRole: string;
  status: "pending" | "approved" | "rejected";
  reason?: string;
  createdAt: string;
  updatedAt: string;
}

export const getUsers = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  vexgoApi.listUsers({
    page: params?.page,
    limit: params?.limit,
    search: params?.search,
  } as never) as unknown as Promise<{ data: UsersResponse }>;

export const updateUserRole = (id: number | string, role: string) =>
  vexgoApi.updateUserRole(
    { id: Number(id) } as never,
    { role } as never,
  ) as unknown as Promise<{ data: { message: string; user: User } }>;

export const deleteUser = (id: number | string) =>
  vexgoApi.deleteUser({ id: Number(id) } as never) as unknown as Promise<{
    data: { message: string };
  }>;

export const applyForCreator = (reason?: string) =>
  vexgoApi.applyForCreator({ reason } as never) as unknown as Promise<{
    data: CreatorApplicationResponse;
  }>;

export const getCreatorApplications = (params?: {
  page?: number;
  limit?: number;
  status?: string;
}) =>
  vexgoApi.listCreatorApplications({
    page: params?.page,
    limit: params?.limit,
    status: params?.status,
  } as never) as unknown as Promise<{ data: CreatorApplicationsResponse }>;

export const reviewCreatorApplication = (
  applicationId: number | string,
  action: "approve" | "reject",
  reason?: string,
) =>
  vexgoApi.reviewCreatorApplication(
    { id: Number(applicationId) } as never,
    { action, reason } as never,
  ) as unknown as Promise<{ data: { message: string } }>;
