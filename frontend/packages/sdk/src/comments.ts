import type { GeneratedAPI } from "./auth.js";

export function createComments(api: GeneratedAPI) {
  return {
    create: (
      body: Parameters<GeneratedAPI["postComments"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postComments"]>>> =>
      api.postComments(body),
    listByPost: (
      id: Parameters<GeneratedAPI["getCommentsPostId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getCommentsPostId"]>>> =>
      api.getCommentsPostId(id),
    remove: (
      id: Parameters<GeneratedAPI["deleteCommentsId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deleteCommentsId"]>>> =>
      api.deleteCommentsId(id),
    moderationPending: (
      params?: Parameters<GeneratedAPI["getModerationCommentsPending"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["getModerationCommentsPending"]>>
    > => api.getModerationCommentsPending(params),
    moderationApproved: (
      params?: Parameters<GeneratedAPI["getModerationCommentsApproved"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["getModerationCommentsApproved"]>>
    > => api.getModerationCommentsApproved(params),
    moderationRejected: (
      params?: Parameters<GeneratedAPI["getModerationCommentsRejected"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["getModerationCommentsRejected"]>>
    > => api.getModerationCommentsRejected(params),
    moderationApprove: (
      id: Parameters<GeneratedAPI["putModerationCommentsIdApprove"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["putModerationCommentsIdApprove"]>>
    > => api.putModerationCommentsIdApprove(id),
    moderationReject: (
      id: Parameters<GeneratedAPI["putModerationCommentsIdReject"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["putModerationCommentsIdReject"]>>
    > => api.putModerationCommentsIdReject(id),
    getModerationConfig: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getModerationCommentsConfig"]>>
    > => api.getModerationCommentsConfig(),
    updateModerationConfig: (
      body: Parameters<GeneratedAPI["putModerationCommentsConfig"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["putModerationCommentsConfig"]>>
    > => api.putModerationCommentsConfig(body),
    testModerationConfig: (): Promise<
      Awaited<ReturnType<GeneratedAPI["postModerationCommentsConfigTest"]>>
    > => api.postModerationCommentsConfigTest(),
  };
}

export type CommentsClient = ReturnType<typeof createComments>;
