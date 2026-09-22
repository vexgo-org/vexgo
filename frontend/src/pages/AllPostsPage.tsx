import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import type { Category, Post } from "@/types";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SkeletonRows } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { PageHeader } from "@/components/PageHeader";
import { Edit, Eye, FileText, Plus } from "lucide-react";
import { toast } from "sonner";
import { normalizeTagsArray } from "@/lib/utils";

const PAGE_SIZE = 10;

/**
 * Every published post on the site, as a paged table. The list comes from
 * `GET /api/posts`, which only ever returns published rows and orders them by
 * creation time — drafts and the moderation queue live on their own screens.
 */
export function AllPostsPage() {
  const { t } = useTranslation();
  const [posts, setPosts] = useState<Post[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("all");
  const [currentPage, setCurrentPage] = useState(1);
  const [pagination, setPagination] = useState({
    total: 0,
    page: 1,
    totalPages: 1,
    limit: PAGE_SIZE,
  });

  useEffect(() => {
    unwrap(getVexGoAPI().getCategories())
      .then((response) =>
        setCategories((response.categories as Category[]) || []),
      )
      .catch((error) => console.error("Failed to load categories:", error));
  }, []);

  const loadPosts = useCallback(async () => {
    setLoading(true);
    try {
      const response = await unwrap(
        getVexGoAPI().getPosts({
          page: currentPage,
          limit: PAGE_SIZE,
          search: search.trim() || undefined,
          category: category === "all" ? undefined : category,
        }),
      );
      setPosts(
        (response.posts || []).map((p) => ({
          ...p,
          tags: normalizeTagsArray(p.tags),
        })) as Post[],
      );
      setPagination({
        total: response.pagination?.total ?? 0,
        page: response.pagination?.page ?? 1,
        totalPages: response.pagination?.totalPages ?? 1,
        limit: response.pagination?.limit ?? PAGE_SIZE,
      });
    } catch (error) {
      console.error("Failed to load posts:", error);
      toast.error(t("allPostsPage.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [currentPage, search, category, t]);

  useEffect(() => {
    loadPosts();
  }, [loadPosts]);

  // A new filter invalidates the current offset, so the query goes back to
  // page 1 instead of paging into an empty window.
  const handleSearchChange = (value: string) => {
    setSearch(value);
    setCurrentPage(1);
  };

  const handleCategoryChange = (value: string) => {
    setCategory(value);
    setCurrentPage(1);
  };

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    // Paging to the top is disorienting under reduced motion.
    const reduceMotion = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    window.scrollTo({ top: 0, behavior: reduceMotion ? "auto" : "smooth" });
  };

  // Posts store the category as either its name or its id, depending on when
  // they were written; both resolve to the readable name here.
  const categoryLabel = (value?: string) => {
    if (!value) return t("allPostsPage.uncategorized");
    const match = categories.find(
      (item) => String(item.id) === value || item.name === value,
    );
    return match?.name ?? value;
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  if (loading && posts.length === 0) {
    return (
      <div className="space-y-6">
        <PageHeader title={t("allPostsPage.title")} />
        <SkeletonRows rows={5} />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("allPostsPage.title")}
        description={t("allPostsPage.description")}
        actions={
          <Button render={<Link to="/admin/write" />}>
            <Plus className="size-4" />
            {t("allPostsPage.writePost")}
          </Button>
        }
      />

      <div className="flex flex-col gap-2 sm:flex-row">
        <Input
          placeholder={t("allPostsPage.searchPlaceholder")}
          aria-label={t("allPostsPage.searchPlaceholder")}
          value={search}
          onChange={(e) => handleSearchChange(e.target.value)}
          className="sm:max-w-xs"
        />
        <Select value={category} onValueChange={handleCategoryChange}>
          <SelectTrigger className="w-full sm:w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">
              {t("allPostsPage.allCategories")}
            </SelectItem>
            {categories.map((item) => (
              <SelectItem key={item.id} value={item.name || String(item.id)}>
                {item.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {posts.length === 0 ? (
        <div className="border-t border-border">
          <EmptyState
            icon={FileText}
            title={t("allPostsPage.noPosts")}
            description={t("allPostsPage.noPostsDesc")}
          />
        </div>
      ) : (
        <>
          <div className="bg-card overflow-hidden rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("posts.title")}</TableHead>
                  <TableHead className="w-32">{t("posts.author")}</TableHead>
                  <TableHead className="w-32">
                    {t("allPostsPage.category")}
                  </TableHead>
                  <TableHead className="w-24">
                    {t("allPostsPage.views")}
                  </TableHead>
                  <TableHead className="w-28">{t("common.date")}</TableHead>
                  <TableHead className="w-28 text-right">
                    {t("common.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {posts.map((post) => (
                  <TableRow key={post.id}>
                    <TableCell className="max-w-0">
                      {/* The post page is served by the public theme. */}
                      <a
                        href={`/post/${post.slug}`}
                        className="block truncate font-medium hover:underline"
                        title={post.title}
                      >
                        {post.title}
                      </a>
                    </TableCell>
                    <TableCell className="text-muted-foreground max-w-0">
                      <span className="block truncate">
                        {post.author?.username}
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {categoryLabel(post.category)}
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular-nums">
                      {post.viewCount ?? 0}
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular-nums">
                      {formatDate(post.createdAt || "")}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-0.5">
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={t("posts.list")}
                          render={<a href={`/post/${post.slug}`} />}
                        >
                          <Eye className="size-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          aria-label={t("posts.edit")}
                          render={<Link to={`/admin/edit-post/${post.id}`} />}
                        >
                          <Edit className="size-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          {pagination.totalPages > 1 && (
            <Pagination className="justify-end">
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevious
                    label={t("common.previous")}
                    onClick={() => handlePageChange(currentPage - 1)}
                    className={
                      currentPage === 1
                        ? "pointer-events-none opacity-50"
                        : "cursor-pointer"
                    }
                  />
                </PaginationItem>

                {Array.from({ length: pagination.totalPages }, (_, i) => i + 1)
                  .filter(
                    (page) =>
                      page === 1 ||
                      page === pagination.totalPages ||
                      Math.abs(page - currentPage) <= 1,
                  )
                  .map((page, index, array) => (
                    <div key={page} className="flex items-center">
                      {index > 0 && array[index - 1] !== page - 1 && (
                        <span className="text-muted-foreground px-2 text-xs tabular-nums">
                          …
                        </span>
                      )}
                      <PaginationItem>
                        <PaginationLink
                          isActive={page === currentPage}
                          onClick={() => handlePageChange(page)}
                        >
                          {page}
                        </PaginationLink>
                      </PaginationItem>
                    </div>
                  ))}

                <PaginationItem>
                  <PaginationNext
                    label={t("common.next")}
                    onClick={() => handlePageChange(currentPage + 1)}
                    className={
                      currentPage === pagination.totalPages
                        ? "pointer-events-none opacity-50"
                        : "cursor-pointer"
                    }
                  />
                </PaginationItem>
              </PaginationContent>
            </Pagination>
          )}
        </>
      )}
    </div>
  );
}
