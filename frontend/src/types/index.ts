// API types are generated from the Go backend source by tygo
// (see tygo.yaml at the repository root; regenerate with `just api-types`).
// The generated files are the single source of truth for the JSON shapes.
export * from "./generated/model";
export * from "./generated/api";

// Hand-written types with no Go counterpart. These cannot be generated: they
// are either frontend-only helpers or shapes the backend passes through
// verbatim from third-party APIs.
export interface ApiResponse<T> {
  message?: string;
  data?: T;
  error?: string;
}

// OpenAI-style model descriptor returned by GET /api/config/ai/models. The
// list is passed through from the configured provider, so it is described
// here rather than in the Go API package.
export interface AIModel {
  id: string;
  object: string;
  created: number;
  owned_by: string;
}
