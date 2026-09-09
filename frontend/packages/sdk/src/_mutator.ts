import { $fetch } from "ofetch";
import type { FetchOptions } from "ofetch";
import { VexGoError, normalizeFetchError } from "./errors.js";
import { getRuntime } from "./runtime.js";
import { clearAuthStorage, getStoredToken } from "./storage.js";

export interface MutatorRequestConfig {
  url?: string;
  method?: string;
  params?: Record<string, unknown>;
  headers?: Record<string, string>;
  data?: unknown;
  signal?: AbortSignal;
  responseType?: string;
}

function resolveToken(): string | null {
  const runtime = getRuntime();
  if (runtime.getToken) return runtime.getToken();
  try {
    return getStoredToken();
  } catch {
    return null;
  }
}

function readErrorStatus(error: unknown): number | undefined {
  if (typeof error !== "object" || error === null) return undefined;
  const response = (error as { response?: unknown }).response;
  if (typeof response !== "object" || response === null) return undefined;
  const status = (response as { status?: unknown }).status;
  return typeof status === "number" ? status : undefined;
}

export async function customFetchInstance<T>(
  config: MutatorRequestConfig,
): Promise<T> {
  const runtime = getRuntime();
  if (!config.url) {
    throw new VexGoError("Request failed: missing URL.");
  }

  const token = resolveToken();
  const headers: Record<string, string> = { ...(config.headers ?? {}) };
  if (token && !headers["Authorization"]) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  try {
    const { headers: baseHeaders, ...restFetchOptions } =
      runtime.fetchOptions ?? {};
    const options: FetchOptions = {
      baseURL: runtime.baseURL,
      method: (config.method ?? "GET").toUpperCase() as FetchOptions["method"],
      query: config.params,
      body: config.data as BodyInit | Record<string, unknown> | undefined,
      signal: config.signal,
      ...restFetchOptions,
      headers: {
        ...(baseHeaders as Record<string, string> | undefined),
        ...headers,
      },
    };
    if (config.responseType) {
      options.responseType =
        config.responseType as FetchOptions["responseType"];
    }
    return (await $fetch(config.url, options)) as T;
  } catch (error) {
    if (readErrorStatus(error) === 401) {
      try {
        clearAuthStorage();
      } catch {
        // Storage is best-effort here; the normalized error below matters.
      }
      runtime.onUnauthorized?.();
    }
    throw normalizeFetchError(error);
  }
}

export default customFetchInstance;
