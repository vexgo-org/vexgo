// Wire shapes are generated from docs/swagger.json by orval (`just generate`).
// These aliases exist so pages keep their existing import names; never
// hand-write an API model here. Page-local form state (fields the API accepts
// but never returns, like passwords) is declared next to the page that uses it.
import type {
  AuthUserResponse,
  CommentCommentResponse,
  PostCategoryResponse,
  PostPostResponse,
  PostTagResponse,
} from "@/api/generated/model";

export type User = AuthUserResponse;
export type Post = PostPostResponse;
export type Category = PostCategoryResponse;
export type Tag = PostTagResponse;
export type Comment = CommentCommentResponse;
