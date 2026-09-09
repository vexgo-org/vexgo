import type { GeneratedAPI } from "./auth.js";

export function createCaptcha(api: GeneratedAPI) {
  return {
    get: (): Promise<Awaited<ReturnType<GeneratedAPI["getCaptcha"]>>> =>
      api.getCaptcha(),
    verify: (
      body: Parameters<GeneratedAPI["postCaptchaVerify"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["postCaptchaVerify"]>>> =>
      api.postCaptchaVerify(body),
  };
}

export type CaptchaClient = ReturnType<typeof createCaptcha>;
