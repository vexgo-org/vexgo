import api from "./api";
import type {
  User,
  UsersResponse,
  UserRoleUpdateResponse,
  UpdateUserRoleRequest,
  ApplyForCreatorRequest,
  ReviewCreatorApplicationRequest,
  MessageResponse,
  CreatorApplicationApplyResponse,
  CreatorApplicationsResponse,
  CreatorApplicationView,
} from "@/types";

// The review page consumes the flat application row rendered by the backend.
export type CreatorApplication = CreatorApplicationView;

// Get the user list
export const getUsers = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) => api.get<UsersResponse>("/users", { params });

// Update a user's role
export const updateUserRole = (id: string | number, role: string) =>
  api.put<UserRoleUpdateResponse>(`/users/${id}/role`, {
    role,
  } satisfies UpdateUserRoleRequest);

// Delete a user
export const deleteUser = (id: string | number) =>
  api.delete<MessageResponse>(`/users/${id}`);

// Apply to become a creator
export const applyForCreator = (reason?: string) =>
  api.post<CreatorApplicationApplyResponse>("/users/apply-creator", {
    reason: reason ?? "",
  } satisfies ApplyForCreatorRequest);

// Get the creator application list (for admins)
export const getCreatorApplications = (params?: {
  page?: number;
  limit?: number;
  status?: string;
}) =>
  api.get<CreatorApplicationsResponse>("/users/creator-applications", {
    params,
  });

// Review a creator application
export const reviewCreatorApplication = (
  applicationId: string | number,
  action: "approve" | "reject",
  reason?: string,
) =>
  api.put<MessageResponse>(
    `/users/creator-applications/${applicationId}/review`,
    { action, reason: reason ?? "" } satisfies ReviewCreatorApplicationRequest,
  );

// Re-export the user types assembled by this module for convenience.
export type { User };
