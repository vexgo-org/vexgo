import { beforeEach, describe, expect, it } from "vitest";
import { configureRuntime, getRuntime, resetRuntime } from "./runtime.js";

describe("runtime", () => {
  beforeEach(() => {
    resetRuntime();
  });

  it("throws before any client is created", () => {
    expect(() => getRuntime()).toThrow(/not configured/);
  });

  it("requires a non-empty baseURL", () => {
    expect(() => configureRuntime({ baseURL: "   " })).toThrow(/baseURL/);
  });

  it("strips trailing slashes from the baseURL", () => {
    configureRuntime({ baseURL: "https://cms.example.com/api///" });
    expect(getRuntime().baseURL).toBe("https://cms.example.com/api");
  });

  it("keeps the injected hooks", () => {
    const getToken = (): string | null => "t";
    const onUnauthorized = (): void => undefined;
    configureRuntime({ baseURL: "/api", getToken, onUnauthorized });
    const runtime = getRuntime();
    expect(runtime.getToken).toBe(getToken);
    expect(runtime.onUnauthorized).toBe(onUnauthorized);
  });
});
