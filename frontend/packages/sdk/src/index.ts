export { createVexgoClient, type VexgoClient } from "./client.js";
export type { VexgoClientOptions } from "./runtime.js";
export {
  VexGoError,
  isVexGoError,
  normalizeFetchError,
  type VexGoErrorDetails,
} from "./errors.js";
export {
  TOKEN_KEY,
  USER_KEY,
  clearAuthStorage,
  getStoredToken,
  getStoredUserJson,
  setStoredToken,
  setStoredUserJson,
} from "./storage.js";
export type { AuthClient } from "./auth.js";
export type { CaptchaClient } from "./captcha.js";
export type { PostsClient } from "./posts.js";
export type { CommentsClient } from "./comments.js";
export type { UsersClient } from "./users.js";
export type { UploadClient } from "./upload.js";
export type { NotificationsClient } from "./notifications.js";
export type { SettingsClient } from "./settings.js";
export type { SsoClient } from "./sso.js";
export type { HomeClient } from "./home.js";
export { getVexGoAPI } from "./generated/endpoints.js";
export * from "./generated/model/index.js";
