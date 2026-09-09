import { createAuth } from "./auth.js";
import type { AuthClient, GeneratedAPI } from "./auth.js";
import { createCaptcha } from "./captcha.js";
import type { CaptchaClient } from "./captcha.js";
import { createComments } from "./comments.js";
import type { CommentsClient } from "./comments.js";
import { createHome } from "./home.js";
import type { HomeClient } from "./home.js";
import { getVexGoAPI } from "./generated/endpoints.js";
import { createNotifications } from "./notifications.js";
import type { NotificationsClient } from "./notifications.js";
import { createPosts } from "./posts.js";
import type { PostsClient } from "./posts.js";
import {
  configureRuntime,
  getRuntime,
  type VexgoClientOptions,
} from "./runtime.js";
import { createSettings } from "./settings.js";
import type { SettingsClient } from "./settings.js";
import { createSso } from "./sso.js";
import type { SsoClient } from "./sso.js";
import { clearAuthStorage, getStoredToken, setStoredToken } from "./storage.js";
import { createUpload } from "./upload.js";
import type { UploadClient } from "./upload.js";
import { createUsers } from "./users.js";
import type { UsersClient } from "./users.js";

export type { VexgoClientOptions };

export interface VexgoClient {
  auth: AuthClient;
  captcha: CaptchaClient;
  posts: PostsClient;
  comments: CommentsClient;
  users: UsersClient;
  upload: UploadClient;
  notifications: NotificationsClient;
  settings: SettingsClient;
  sso: SsoClient;
  home: HomeClient;
  readonly baseURL: string;
  getToken: () => string | null;
  setToken: (token: string) => void;
  clearToken: () => void;
}

function readToken(): string | null {
  const runtime = getRuntime();
  const injected = runtime.getToken?.();
  if (injected) return injected;
  try {
    return getStoredToken();
  } catch {
    return null;
  }
}

/**
 * Create the SDK client. The client is browser-only (localStorage token
 * storage) and process-wide: calling this twice reconfigures the shared
 * runtime, which matches the single-token localStorage model. One theme
 * talks to one backend, so a single active client is the intended use.
 */
export function createVexgoClient(options: VexgoClientOptions): VexgoClient {
  configureRuntime(options);
  const api: GeneratedAPI = getVexGoAPI();
  const runtime = getRuntime();
  return {
    auth: createAuth(api),
    captcha: createCaptcha(api),
    posts: createPosts(api),
    comments: createComments(api),
    users: createUsers(api),
    upload: createUpload(api),
    notifications: createNotifications(api),
    settings: createSettings(api),
    sso: createSso(api),
    home: createHome(api),
    baseURL: runtime.baseURL,
    getToken: readToken,
    setToken: setStoredToken,
    clearToken: clearAuthStorage,
  };
}
