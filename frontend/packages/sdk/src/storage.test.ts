import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  TOKEN_KEY,
  USER_KEY,
  clearAuthStorage,
  getStoredToken,
  getStoredUserJson,
  setStoredToken,
  setStoredUserJson,
} from "./storage.js";

function installMemoryStorage(): Map<string, string> {
  const store = new Map<string, string>();
  const storage = {
    getItem: (key: string): string | null => store.get(key) ?? null,
    setItem: (key: string, value: string): void => {
      store.set(key, value);
    },
    removeItem: (key: string): void => {
      store.delete(key);
    },
    clear: (): void => {
      store.clear();
    },
    key: (index: number): string | null =>
      Array.from(store.keys())[index] ?? null,
    get length(): number {
      return store.size;
    },
  };
  vi.stubGlobal("localStorage", storage);
  return store;
}

describe("storage", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });

  it("round-trips the token under the namespaced key", () => {
    const store = installMemoryStorage();
    expect(getStoredToken()).toBeNull();
    setStoredToken("jwt-123");
    expect(getStoredToken()).toBe("jwt-123");
    expect(store.get(TOKEN_KEY)).toBe("jwt-123");
  });

  it("does not touch the legacy frontend keys", () => {
    const store = installMemoryStorage();
    store.set("token", "legacy");
    setStoredToken("sdk-token");
    expect(store.get("token")).toBe("legacy");
    expect(store.get(TOKEN_KEY)).toBe("sdk-token");
  });

  it("round-trips the user JSON and clears both keys", () => {
    installMemoryStorage();
    setStoredToken("jwt-123");
    setStoredUserJson(JSON.stringify({ id: 1 }));
    expect(getStoredUserJson()).toBe(JSON.stringify({ id: 1 }));
    clearAuthStorage();
    expect(getStoredToken()).toBeNull();
    expect(getStoredUserJson()).toBeNull();
  });

  it("exposes the key names for theme authors", () => {
    expect(TOKEN_KEY).toBe("vexgo:token");
    expect(USER_KEY).toBe("vexgo:user");
  });

  it("throws a browser-only error when localStorage is missing", () => {
    expect(() => getStoredToken()).toThrow(/browser environment/);
  });
});
