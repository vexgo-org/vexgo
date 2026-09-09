import type { GeneratedAPI } from "./auth.js";

export function createUpload(api: GeneratedAPI) {
  return {
    uploadFile: (
      file: Blob | File,
    ): Promise<Awaited<ReturnType<GeneratedAPI["postUpload"]>>> =>
      api.postUpload({ file }),
    uploadMultiple: (
      files: Blob | File,
    ): Promise<Awaited<ReturnType<GeneratedAPI["postUploadMultiple"]>>> =>
      api.postUploadMultiple({ files }),
    myFiles: (): Promise<Awaited<ReturnType<GeneratedAPI["getUploadMy"]>>> =>
      api.getUploadMy(),
    remove: (
      id: Parameters<GeneratedAPI["deleteUploadId"]>[0],
    ): Promise<Awaited<ReturnType<GeneratedAPI["deleteUploadId"]>>> =>
      api.deleteUploadId(id),
  };
}

export type UploadClient = ReturnType<typeof createUpload>;
