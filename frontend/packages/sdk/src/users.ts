import type { GeneratedAPI } from "./auth.js";

export function createUsers(api: GeneratedAPI) {
  return {
    list: (
      params?: Parameters<GeneratedAPI["getUsers"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getUsers"]>>> =>
      api.getUsers(params),
    updateRole: (
      id: Parameters<GeneratedAPI["putUsersIdRole"]>[0],
      body: Parameters<GeneratedAPI["putUsersIdRole"]>[1],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putUsersIdRole"]>>> =>
      api.putUsersIdRole(id, body),
    remove: (
      id: Parameters<GeneratedAPI["deleteUsersId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deleteUsersId"]>>> =>
      api.deleteUsersId(id),
    applyCreator: (
      body: Parameters<GeneratedAPI["postUsersApplyCreator"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postUsersApplyCreator"]>>> =>
      api.postUsersApplyCreator(body),
    listCreatorApplications: (
      params?: Parameters<GeneratedAPI["getUsersCreatorApplications"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["getUsersCreatorApplications"]>>
    > => api.getUsersCreatorApplications(params),
    reviewCreatorApplication: (
      id: Parameters<GeneratedAPI["putUsersCreatorApplicationsIdReview"]>[0],
      body: Parameters<GeneratedAPI["putUsersCreatorApplicationsIdReview"]>[1],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["putUsersCreatorApplicationsIdReview"]>>
    > => api.putUsersCreatorApplicationsIdReview(id, body),
  };
}

export type UsersClient = ReturnType<typeof createUsers>;
