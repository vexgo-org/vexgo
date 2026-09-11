import customInstance from "@/api/customAxios";

export interface PageItem {
  id: number;
  slug: string;
  title: string;
  content: string;
  showInNav: boolean;
  sortOrder: number;
  status: "draft" | "published";
  authorId?: number;
  createdAt?: string;
  updatedAt?: string;
}

interface PagesListData {
  pages?: PageItem[];
  pagination?: {
    total?: number;
    page?: number;
    limit?: number;
    totalPages?: number;
  };
}

function unwrapData<T>(p: Promise<{ data: T }>): Promise<T> {
  return p.then((r) => r.data);
}

export const pagesAPI = {
  list: (params?: {
    page?: number;
    limit?: number;
    status?: string;
    search?: string;
  }) => unwrapData<PagesListData>(customInstance.get("/pages", { params })),
  getBySlug: (slug: string) =>
    unwrapData<{ page: PageItem }>(
      customInstance.get(`/pages/${encodeURIComponent(slug)}`),
    ),
  create: (body: {
    slug: string;
    title: string;
    content: string;
    showInNav: boolean;
    sortOrder: number;
    status: string;
  }) => unwrapData<{ page: PageItem }>(customInstance.post("/pages", body)),
  update: (
    id: string | number,
    body: Partial<{
      slug: string;
      title: string;
      content: string;
      showInNav: boolean;
      sortOrder: number;
      status: string;
    }>,
  ) => unwrapData<{ page: PageItem }>(customInstance.put(`/pages/${id}`, body)),
  remove: (id: string | number) =>
    unwrapData<{ message: string }>(customInstance.delete(`/pages/${id}`)),
};

export const RESERVED_PAGE_SLUGS = new Set([
  "post",
  "posts",
  "user",
  "users",
  "admin",
  "api",
  "theme-assets",
  "themes",
  "assets",
  "favicon.ico",
  "login",
  "register",
  "reset-password",
  "verify-email",
  "write",
  "edit-post",
  "profile",
  "my-posts",
  "notifications",
  "settings",
  "moderation",
]);

export function normalizePageSlug(slug: string): string {
  return slug.trim().toLowerCase();
}

export function isValidPageSlug(slug: string): boolean {
  return /^[a-z0-9-]{1,100}$/.test(slug) && !RESERVED_PAGE_SLUGS.has(slug);
}
