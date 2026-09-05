// Moderation-domain API surface. Mirrors the legacy
// moderationApi.ts but delegates to the orval-generated
// `vexgoApi`. The functions keep their original signatures
// so the moderation page (and any future admin queue) can
// keep using `getPendingPosts`, `approvePost`, etc.

import { vexgoApi } from "@/api";
import type { Post, PostsResponse } from "@/types";

export const getPendingPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  vexgoApi.listPendingPosts({
    page: params?.page,
    limit: params?.limit,
    search: params?.search,
  } as never) as unknown as Promise<{ data: PostsResponse }>;

export const getApprovedPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  vexgoApi.listApprovedPosts({
    page: params?.page,
    limit: params?.limit,
    search: params?.search,
  } as never) as unknown as Promise<{ data: PostsResponse }>;

export const getRejectedPosts = (params?: {
  page?: number;
  limit?: number;
  search?: string;
}) =>
  vexgoApi.listRejectedPosts({
    page: params?.page,
    limit: params?.limit,
    search: params?.search,
  } as never) as unknown as Promise<{ data: PostsResponse }>;

export const approvePost = (id: number | string) =>
  vexgoApi.approvePost({ id: Number(id) } as never) as unknown as Promise<{
    data: { message: string; post: Post };
  }>;

export const rejectPost = (id: number | string, rejectionReason?: string) =>
  vexgoApi.rejectPost(
    { id: Number(id) } as never,
    { rejectionReason } as never,
  ) as unknown as Promise<{ data: { message: string; post: Post } }>;

export const resubmitPost = (id: number | string) =>
  vexgoApi.resubmitPost({ id: Number(id) } as never) as unknown as Promise<{
    data: { message: string; post: Post };
  }>;
