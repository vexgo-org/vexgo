import type { FetchOptions } from "ofetch";

export interface VexgoClientOptions {
  baseURL: string;
  getToken?: () => string | null;
  onUnauthorized?: () => void;
  fetchOptions?: FetchOptions;
}

interface RuntimeConfig {
  baseURL: string;
  getToken?: () => string | null;
  onUnauthorized?: () => void;
  fetchOptions?: FetchOptions;
}

let runtime: RuntimeConfig | null = null;

export function configureRuntime(options: VexgoClientOptions): void {
  const baseURL = options.baseURL.trim().replace(/\/+$/, "");
  if (baseURL.length === 0) {
    throw new Error("@vexgo/sdk: baseURL is required.");
  }
  runtime = {
    baseURL,
    getToken: options.getToken,
    onUnauthorized: options.onUnauthorized,
    fetchOptions: options.fetchOptions,
  };
}

export function getRuntime(): RuntimeConfig {
  if (runtime === null) {
    throw new Error(
      "@vexgo/sdk: client is not configured. Call createVexgoClient() first.",
    );
  }
  return runtime;
}

export function resetRuntime(): void {
  runtime = null;
}
