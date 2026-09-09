import { describe, expect, it } from "vitest";
import { VexGoError, isVexGoError, normalizeFetchError } from "./errors.js";

describe("VexGoError", () => {
  it("passes through instances untouched", () => {
    const original = new VexGoError("nope", { status: 404 });
    expect(normalizeFetchError(original)).toBe(original);
  });

  it("normalizes an ofetch-style error with a coded body", () => {
    const error = {
      message: "POST /api/posts failed",
      response: {
        status: 422,
        _data: { code: "slug_taken", error: "Slug is already taken" },
      },
    };
    const normalized = normalizeFetchError(error);
    expect(normalized).toBeInstanceOf(VexGoError);
    expect(normalized.message).toBe("Slug is already taken");
    expect(normalized.status).toBe(422);
    expect(normalized.code).toBe("slug_taken");
  });

  it("prefers the message field when no error field exists", () => {
    const normalized = normalizeFetchError({
      message: "ignored",
      response: { status: 500, _data: { message: "boom" } },
    });
    expect(normalized.message).toBe("boom");
    expect(normalized.status).toBe(500);
    expect(normalized.code).toBeUndefined();
  });

  it("handles plain errors and non-objects without throwing", () => {
    const fromError = normalizeFetchError(new Error("network down"));
    expect(fromError.message).toBe("network down");
    expect(fromError.status).toBeUndefined();

    const fromString = normalizeFetchError("kaboom");
    expect(fromString.message).toBe("Request failed.");
  });

  it("guards with isVexGoError", () => {
    expect(isVexGoError(new VexGoError("x"))).toBe(true);
    expect(isVexGoError(new Error("x"))).toBe(false);
    expect(isVexGoError(null)).toBe(false);
  });
});
