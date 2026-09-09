export interface VexGoErrorDetails {
  status?: number;
  code?: string;
  data?: unknown;
}

export class VexGoError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly data?: unknown;

  constructor(message: string, details: VexGoErrorDetails = {}) {
    super(message);
    this.name = "VexGoError";
    this.status = details.status;
    this.code = details.code;
    this.data = details.data;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function readStatus(value: unknown): number | undefined {
  if (typeof value === "number") return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isInteger(parsed) ? parsed : undefined;
  }
  return undefined;
}

function readMessage(value: unknown, fallback: string): string {
  return typeof value === "string" && value.length > 0 ? value : fallback;
}

export function normalizeFetchError(error: unknown): VexGoError {
  if (error instanceof VexGoError) return error;

  let status: number | undefined;
  let data: unknown;
  let message = "Request failed.";

  if (isRecord(error)) {
    const response = error["response"];
    if (isRecord(response)) {
      status = readStatus(response["status"]);
      const responseData = response["_data"];
      data = responseData !== undefined ? responseData : error["data"];
    } else {
      status = readStatus(error["status"]);
      data = error["data"];
    }
    message = readMessage(error["message"], message);
  }

  let code: string | undefined;
  if (isRecord(data)) {
    const rawCode = data["code"];
    if (typeof rawCode === "string") code = rawCode;
    const rawError = data["error"];
    if (typeof rawError === "string" && rawError.length > 0) {
      message = rawError;
    } else {
      const rawMessage = data["message"];
      if (typeof rawMessage === "string" && rawMessage.length > 0) {
        message = rawMessage;
      }
    }
  }

  return new VexGoError(message, { status, code, data });
}

export function isVexGoError(error: unknown): error is VexGoError {
  return error instanceof VexGoError;
}
