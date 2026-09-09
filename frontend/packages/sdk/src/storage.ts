export const TOKEN_KEY = "vexgo:token";
export const USER_KEY = "vexgo:user";

function browserStorage(): Storage {
  if (typeof localStorage === "undefined") {
    throw new Error(
      "@vexgo/sdk requires a browser environment with localStorage.",
    );
  }
  return localStorage;
}

export function getStoredToken(): string | null {
  return browserStorage().getItem(TOKEN_KEY);
}

export function setStoredToken(token: string): void {
  browserStorage().setItem(TOKEN_KEY, token);
}

export function getStoredUserJson(): string | null {
  return browserStorage().getItem(USER_KEY);
}

export function setStoredUserJson(json: string): void {
  browserStorage().setItem(USER_KEY, json);
}

export function clearAuthStorage(): void {
  const storage = browserStorage();
  storage.removeItem(TOKEN_KEY);
  storage.removeItem(USER_KEY);
}
