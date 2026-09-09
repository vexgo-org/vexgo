import type { GeneratedAPI } from "./auth.js";

export function createSettings(api: GeneratedAPI) {
  return {
    getGeneral: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getConfigGeneral"]>>
    > => api.getConfigGeneral(),
    updateGeneral: (
      body: Parameters<GeneratedAPI["putConfigGeneral"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putConfigGeneral"]>>> =>
      api.putConfigGeneral(body),
    getTheme: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getConfigTheme"]>>
    > => api.getConfigTheme(),
    updateTheme: (
      body: Parameters<GeneratedAPI["putConfigTheme"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putConfigTheme"]>>> =>
      api.putConfigTheme(body),
    listThemes: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getConfigThemes"]>>
    > => api.getConfigThemes(),
    previewTheme: (
      id: Parameters<GeneratedAPI["getConfigThemesIdPreview"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["getConfigThemesIdPreview"]>>> =>
      api.getConfigThemesIdPreview(id),
    uploadTheme: (
      theme: Blob | File,
    ): Promise<Awaited<ReturnType<GeneratedAPI["postConfigThemeUpload"]>>> =>
      api.postConfigThemeUpload({ theme }),
    getSmtp: (): Promise<Awaited<ReturnType<GeneratedAPI["getConfigSmtp"]>>> =>
      api.getConfigSmtp(),
    updateSmtp: (
      body: Parameters<GeneratedAPI["putConfigSmtp"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putConfigSmtp"]>>> =>
      api.putConfigSmtp(body),
    testSmtp: (): Promise<
      Awaited<ReturnType<GeneratedAPI["postConfigSmtpTest"]>>
    > => api.postConfigSmtpTest(),
    getAi: (): Promise<Awaited<ReturnType<GeneratedAPI["getConfigAi"]>>> =>
      api.getConfigAi(),
    updateAi: (
      body: Parameters<GeneratedAPI["putConfigAi"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["putConfigAi"]>>> =>
      api.putConfigAi(body),
    listAiModels: (): Promise<
      Awaited<ReturnType<GeneratedAPI["getConfigAiModels"]>>
    > => api.getConfigAiModels(),
    testAi: (): Promise<
      Awaited<ReturnType<GeneratedAPI["postConfigAiTest"]>>
    > => api.postConfigAiTest(),
  };
}

export type SettingsClient = ReturnType<typeof createSettings>;
