import { getVexGoAPI } from "./generated/endpoints.js";
import {
  clearAuthStorage,
  setStoredToken,
  setStoredUserJson,
} from "./storage.js";

export type GeneratedAPI = ReturnType<typeof getVexGoAPI>;

export function createAuth(api: GeneratedAPI) {
  return {
    register: (
      body: Parameters<GeneratedAPI["postAuthRegister"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postAuthRegister"]>>> =>
      api.postAuthRegister(body),
    login: async (
      body: Parameters<GeneratedAPI["postAuthLogin"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postAuthLogin"]>>> => {
      const result = await api.postAuthLogin(body);
      if (result.token) setStoredToken(result.token);
      if (result.user) setStoredUserJson(JSON.stringify(result.user));
      return result;
    },
    loginWithToken: async (
      token: string,
    ): Promise<Awaited<ReturnType<GeneratedAPI["getAuthMe"]>>> => {
      setStoredToken(token);
      const result = await api.getAuthMe();
      if (result.user) setStoredUserJson(JSON.stringify(result.user));
      return result;
    },
    logout: (): void => {
      clearAuthStorage();
    },
    me: async (): Promise<Awaited<ReturnType<GeneratedAPI["getAuthMe"]>>> => {
      const result = await api.getAuthMe();
      if (result.user) setStoredUserJson(JSON.stringify(result.user));
      return result;
    },
    updateProfile: (
      body: Parameters<GeneratedAPI["putAuthProfile"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putAuthProfile"]>>> =>
      api.putAuthProfile(body),
    changePassword: (
      body: Parameters<GeneratedAPI["putAuthPassword"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putAuthPassword"]>>> =>
      api.putAuthPassword(body),
    updateEmail: (
      body: Parameters<GeneratedAPI["putAuthEmail"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putAuthEmail"]>>> =>
      api.putAuthEmail(body),
    verifyEmail: (
      params: Parameters<GeneratedAPI["getAuthEmailVerify"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getAuthEmailVerify"]>>> =>
      api.getAuthEmailVerify(params),
    resendVerification: (
      body: Parameters<GeneratedAPI["postAuthEmailVerifyResend"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["postAuthEmailVerifyResend"]>>
    > => api.postAuthEmailVerifyResend(body),
    verificationStatus: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getAuthEmailVerifyStatus"]>>
    > => api.getAuthEmailVerifyStatus(),
    updateSettings: (
      body: Parameters<GeneratedAPI["putAuthSettings"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putAuthSettings"]>>> =>
      api.putAuthSettings(body),
    requestPasswordReset: (
      body: Parameters<GeneratedAPI["postAuthPasswordResetRequest"]>[0],
    ): Promise<
      Awaited<ReturnType<GeneratedAPI["postAuthPasswordResetRequest"]>>
    > => api.postAuthPasswordResetRequest(body),
    resetPassword: (
      body: Parameters<GeneratedAPI["postAuthPasswordReset"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postAuthPasswordReset"]>>> =>
      api.postAuthPasswordReset(body),
  };
}

export type AuthClient = ReturnType<typeof createAuth>;
