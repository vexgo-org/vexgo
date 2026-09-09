import { createVexgoClient, isVexGoError, type VexgoClient } from "@vexgo/sdk";

const apiBaseUrl: string = import.meta.env.VITE_API_URL || "/api";

// One-time migration from the legacy axios client, which stored the session
// under `token`/`user`. The SDK owns `vexgo:token`/`vexgo:user` instead.
function migrateLegacyAuthStorage(): void {
  try {
    if (localStorage.getItem("vexgo:token") === null) {
      const legacyToken = localStorage.getItem("token");
      if (legacyToken !== null) {
        localStorage.setItem("vexgo:token", legacyToken);
        const legacyUser = localStorage.getItem("user");
        if (legacyUser !== null) {
          localStorage.setItem("vexgo:user", legacyUser);
        }
      }
    }
    localStorage.removeItem("token");
    localStorage.removeItem("user");
  } catch {
    // Storage unavailable: SDK calls surface errors to the UI as usual.
  }
}

migrateLegacyAuthStorage();

export const sdk: VexgoClient = createVexgoClient({
  baseURL: apiBaseUrl,
  onUnauthorized: () => {
    if (window.location.pathname !== "/login") {
      window.location.href = "/login";
    }
  },
});

// API errors already carry the normalized backend message;
// anything else falls back to the caller's UI string.
export function getErrorMessage(error: unknown, fallback: string): string {
  if (isVexGoError(error)) return error.message;
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}
