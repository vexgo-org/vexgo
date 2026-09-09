import type { GeneratedAPI } from "./auth.js";

export function createPosts(api: GeneratedAPI) {
  return {
    list: (
      params?: Parameters<GeneratedAPI["getPosts"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getPosts"]>>> =>
      api.getPosts(params),
    create: (
      body: Parameters<GeneratedAPI["postPosts"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postPosts"]>>> =>
      api.postPosts(body),
    getBySlug: (
      slug: Parameters<GeneratedAPI["getPostsSlug"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getPostsSlug"]>>> =>
      api.getPostsSlug(slug),
    getById: (
      id: Parameters<GeneratedAPI["getPostsByIdId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getPostsByIdId"]>>> =>
      api.getPostsByIdId(id),
    update: (
      id: Parameters<GeneratedAPI["putPostsId"]>[0],
      body: Parameters<GeneratedAPI["putPostsId"]>[1],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putPostsId"]>>> =>
      api.putPostsId(id, body),
    remove: (
      id: Parameters<GeneratedAPI["deletePostsId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deletePostsId"]>>> =>
      api.deletePostsId(id),
    drafts: (
      params?: Parameters<GeneratedAPI["getPostsDrafts"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getPostsDrafts"]>>> =>
      api.getPostsDrafts(params),
    myPosts: (
      params?: Parameters<GeneratedAPI["getPostsUserMyPosts"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getPostsUserMyPosts"]>>> =>
      api.getPostsUserMyPosts(params),
    listByUser: (
      id: Parameters<GeneratedAPI["getPostsUserId"]>[0],
      params?: Parameters<GeneratedAPI["getPostsUserId"]>[1],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getPostsUserId"]>>> =>
      api.getPostsUserId(id, params),
    listCategories: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getCategories"]>>
    > => api.getCategories(),
    createCategory: (
      body: Parameters<GeneratedAPI["postCategories"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postCategories"]>>> =>
      api.postCategories(body),
    deleteCategory: (
      id: Parameters<GeneratedAPI["deleteCategoriesId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deleteCategoriesId"]>>> =>
      api.deleteCategoriesId(id),
    listTags: (): Promise<Awaited<ReturnType<GeneratedAPI["getTags"]>>> =>
      api.getTags(),
    createTag: (
      body: Parameters<GeneratedAPI["postTags"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postTags"]>>> =>
      api.postTags(body),
    deleteTag: (
      id: Parameters<GeneratedAPI["deleteTagsId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deleteTagsId"]>>> =>
      api.deleteTagsId(id),
    likeStatus: (
      postId: Parameters<GeneratedAPI["getLikesPostId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getLikesPostId"]>>> =>
      api.getLikesPostId(postId),
    like: (
      postId: Parameters<GeneratedAPI["postLikesPostId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postLikesPostId"]>>> =>
      api.postLikesPostId(postId),
    moderationPending: (
      params?: Parameters<GeneratedAPI["getModerationPending"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getModerationPending"]>>> =>
      api.getModerationPending(params),
    moderationApproved: (
      params?: Parameters<GeneratedAPI["getModerationApproved"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getModerationApproved"]>>> =>
      api.getModerationApproved(params),
    moderationRejected: (
      params?: Parameters<GeneratedAPI["getModerationRejected"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getModerationRejected"]>>> =>
      api.getModerationRejected(params),
    moderationApprove: (
      id: Parameters<GeneratedAPI["putModerationApproveId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putModerationApproveId"]>>> =>
      api.putModerationApproveId(id),
    moderationReject: (
      id: Parameters<GeneratedAPI["putModerationRejectId"]>[0],
      body: Parameters<GeneratedAPI["putModerationRejectId"]>[1],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putModerationRejectId"]>>> =>
      api.putModerationRejectId(id, body),
    moderationResubmit: (
      id: Parameters<GeneratedAPI["putModerationResubmitId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putModerationResubmitId"]>>> =>
      api.putModerationResubmitId(id),
  };
}

export type PostsClient = ReturnType<typeof createPosts>;
