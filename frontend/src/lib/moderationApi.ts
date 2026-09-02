import api from "./api";
import type {
  PostsResponse,
  PostMutationResponse,
  RejectPostRequest,
} from "@/types";

// Get the list of pending posts
export const getPendingPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) => api.get<PostsResponse>("/moderation/pending", { params });

// Get the list of approved posts
export const getApprovedPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) => api.get<PostsResponse>("/moderation/approved", { params });

// Get the list of rejected posts
export const getRejectedPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) => api.get<PostsResponse>("/moderation/rejected", { params });

// Approve a post
export const approvePost = (id: string | number) =>
  api.put<PostMutationResponse>(`/moderation/approve/${id}`);

// Reject a post
export const rejectPost = (id: string | number, rejectionReason?: string) =>
  api.put<PostMutationResponse>(`/moderation/reject/${id}`, {
    rejectionReason: rejectionReason ?? "",
  } satisfies RejectPostRequest);

// Resubmit a post for review
export const resubmitPost = (id: string | number) =>
  api.put<PostMutationResponse>(`/moderation/resubmit/${id}`);
