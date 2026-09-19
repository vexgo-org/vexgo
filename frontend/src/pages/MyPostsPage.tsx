import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import type { Post } from "@/types";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Edit, Eye, PenLine, Plus, Trash2 } from "lucide-react";
import { normalizeTagsArray } from "@/lib/utils";

export function MyPostsPage() {
  const { t } = useTranslation();
  const { user, isAuthenticated } = useAuth();
  const [posts, setPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [pagination, setPagination] = useState({
    total: 0,
    page: 1,
    totalPages: 1,
    limit: 10,
  });

  // Check if user is guest
  useEffect(() => {
    if (isAuthenticated && user?.role === "guest") {
      // Guests cannot manage posts; send them to the public site.
      window.location.href = "/";
    }
  }, [isAuthenticated, user]);

  const loadPosts = useCallback(async () => {
    setLoading(true);
    try {
      const response = await unwrap(
        getVexGoAPI().getPostsUserMyPosts({
          page: currentPage,
          limit: 10,
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
        limit: response.pagination?.limit ?? 10,
      });
    } catch (error) {
      console.error("Failed to load posts:", error);
    } finally {
      setLoading(false);
    }
  }, [currentPage]);

  useEffect(() => {
    loadPosts();
  }, [currentPage, loadPosts]);

  const handleDeletePost = async (postId: string) => {
    try {
      await unwrap(getVexGoAPI().deletePostsId(postId));
      loadPosts();
    } catch (error) {
      console.error("Failed to delete post:", error);
    }
  };

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    // Paging to the top is disorienting under reduced motion.
    const reduceMotion = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches;
    window.scrollTo({ top: 0, behavior: reduceMotion ? "auto" : "smooth" });
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
        <Skeleton className="h-8 w-40" />
        {[1, 2, 3, 4, 5].map((i) => (
          <Skeleton key={i} className="h-12 rounded-sm" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("myPostsPage.myPosts")}
        actions={
          user?.role !== "guest" ? (
            <Button asChild>
              <Link to="/admin/write">
                <Plus className="size-4" />
                {t("myPostsPage.writePost")}
              </Link>
            </Button>
          ) : undefined
        }
      />

      {posts.length === 0 ? (
        <Card className="py-0">
          <EmptyState
            icon={PenLine}
            title={t("myPostsPage.noPosts")}
            description={t("myPostsPage.noPostsDesc")}
            action={
              user?.role !== "guest" ? (
                <Button asChild>
                  <Link to="/admin/write">
                    <Plus className="size-4" />
                    {t("myPostsPage.writePost")}
                  </Link>
                </Button>
              ) : undefined
            }
          />
        </Card>
      ) : (
        <>
          <div className="bg-card overflow-hidden rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("posts.title")}</TableHead>
                  <TableHead className="w-28">{t("posts.status")}</TableHead>
                  <TableHead className="w-40">{t("posts.tags")}</TableHead>
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
                    <TableCell>
                      <StatusBadge status={post.status} />
                    </TableCell>
                    <TableCell className="text-muted-foreground max-w-0">
                      <span className="block truncate">
                        {(post.tags ?? []).slice(0, 3).join(", ")}
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground tabular-nums">
                      {formatDate(post.createdAt || "")}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-0.5">
                        <Button variant="ghost" size="icon-sm" asChild>
                          <a
                            href={`/post/${post.slug}`}
                            aria-label={t("posts.list")}
                          >
                            <Eye className="size-4" />
                          </a>
                        </Button>
                        <Button variant="ghost" size="icon-sm" asChild>
                          <Link
                            to={`/admin/edit-post/${post.id}`}
                            aria-label={t("posts.edit")}
                          >
                            <Edit className="size-4" />
                          </Link>
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              aria-label={t("posts.delete")}
                            >
                              <Trash2 className="size-4" />
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>
                                {t("myPostsPage.confirmDelete")}
                              </AlertDialogTitle>
                              <AlertDialogDescription>
                                {t("myPostsPage.cannotUndo")}
                              </AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>
                                {t("myPostsPage.cancel")}
                              </AlertDialogCancel>
                              <AlertDialogAction
                                onClick={() =>
                                  handleDeletePost(String(post.id))
                                }
                                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                              >
                                {t("myPostsPage.delete")}
                              </AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
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
                        <span className="text-muted-foreground px-2 text-[13px]">
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
