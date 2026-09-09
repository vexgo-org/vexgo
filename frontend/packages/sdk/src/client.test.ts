import { beforeEach, describe, expect, it, vi } from "vitest";
import { createVexgoClient } from "./client.js";
import { resetRuntime } from "./runtime.js";
import { getStoredToken } from "./storage.js";

describe("createVexgoClient", () => {
  beforeEach(() => {
    resetRuntime();
    vi.unstubAllGlobals();
  });

  it("exposes one group per backend domain", () => {
    const client = createVexgoClient({
      baseURL: "https://cms.example.com/api",
    });
    expect(client.baseURL).toBe("https://cms.example.com/api");
    for (const group of [
      client.auth,
      client.captcha,
      client.posts,
      client.comments,
      client.users,
      client.upload,
      client.notifications,
      client.settings,
      client.sso,
      client.home,
    ]) {
      expect(group).toBeTypeOf("object");
    }
    expect(client.auth.login).toBeTypeOf("function");
    expect(client.posts.list).toBeTypeOf("function");
    expect(client.upload.uploadFile).toBeTypeOf("function");
  });

  it("rejects an empty baseURL", () => {
    expect(() => createVexgoClient({ baseURL: "" })).toThrow(/baseURL/);
  });

  it("prefers the injected token over storage", () => {
    const client = createVexgoClient({
      baseURL: "/api",
      getToken: () => "injected",
    });
    expect(client.getToken()).toBe("injected");
  });

  it("reads and writes the token through localStorage by default", () => {
    const store = new Map<string, string>();
    vi.stubGlobal("localStorage", {
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
      key: (): string | null => null,
      length: 0,
    });
    const client = createVexgoClient({ baseURL: "/api" });
    expect(client.getToken()).toBeNull();
    client.setToken("abc");
    expect(client.getToken()).toBe("abc");
    expect(getStoredToken()).toBe("abc");
    client.clearToken();
    expect(client.getToken()).toBeNull();
  });
});
