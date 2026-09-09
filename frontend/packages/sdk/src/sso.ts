import type { GeneratedAPI } from "./auth.js";

export function createSso(api: GeneratedAPI) {
  return {
    providers: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getSsoProviders"]>>
    > => api.getSsoProviders(),
    loginUrl: (
      provider: Parameters<GeneratedAPI["getSsoProviderLogin"]>[0],
      params?: Parameters<GeneratedAPI["getSsoProviderLogin"]>[1],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getSsoProviderLogin"]>>> =>
      api.getSsoProviderLogin(provider, params),
    callback: (
      provider: Parameters<GeneratedAPI["getSsoProviderCallback"]>[0],
      params: Parameters<GeneratedAPI["getSsoProviderCallback"]>[1],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getSsoProviderCallback"]>>> =>
      api.getSsoProviderCallback(provider, params),
  };
}

export type SsoClient = ReturnType<typeof createSso>;
