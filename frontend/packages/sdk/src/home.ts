import type { GeneratedAPI } from "./auth.js";

export function createHome(api: GeneratedAPI) {
  return {
    stats: (): Promise<Awaited<ReturnType<GeneratedAPI["getStats"]>>> =>
      api.getStats(),
    latestPosts: (
      params?: Parameters<GeneratedAPI["getStatsLatestPosts"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getStatsLatestPosts"]>>> =>
      api.getStatsLatestPosts(params),
    popularPosts: (
      params?: Parameters<GeneratedAPI["getStatsPopularPosts"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getStatsPopularPosts"]>>> =>
      api.getStatsPopularPosts(params),
  };
}

export type HomeClient = ReturnType<typeof createHome>;
