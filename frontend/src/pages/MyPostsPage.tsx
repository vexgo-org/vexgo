import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { postsApi } from "@/lib/api";
import type { Post } from "@/types";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
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
import {
  PenLine,
  Edit,
  Trash2,
  Eye,
  Clock,
  FileX,
  Plus,
  CheckCircle,
  XCircle,
  AlertCircle,
} from "lucide-react";
import { normalizeTagsArray } from "@/lib/utils";

export function MyPostsPage() {
  const { t } = useTranslation();
  const { user, isAuthenticated } = useAuth();
  const navigate = useNavigate();
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
      navigate("/");
    }
  }, [isAuthenticated, user, navigate]);

  const loadPosts = useCallback(async () => {
    setLoading(true);
    try {
      const response = await postsApi.getMyPosts({
        page: currentPage,
        limit: 10,
      });
      setPosts(
        response.data.posts.map((p) => ({
          ...p,
          tags: normalizeTagsArray(p.tags),
        })),
      );
      setPagination(response.data.pagination);
    } catch (error) {
      console.error("Failed to load posts:", error);
    } finally {
      setLoading(false);
    }
  }, [currentPage]);

  useEffect(() => {
    loadPosts();
  }, [currentPage, loadPosts]);

  const handleDeletePost = async (postId: number | string) => {
    try {
      await postsApi.deletePost(postId);
      loadPosts();
    } catch (error) {
      console.error("Failed to delete post:", error);
    }
  };

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "published":
        return {
          variant: "default" as const,
          label: t("myPostsPage.published"),
          icon: CheckCircle,
          className: "bg-green-600 hover:bg-green-700",
        };
      case "draft":
        return {
          variant: "secondary" as const,
          label: t("myPostsPage.draft"),
          icon: FileX,
          className: "",
        };
      case "pending":
        return {
          variant: "outline" as const,
          label: t("myPostsPage.pending"),
          icon: Clock,
          className: "text-yellow-600 border-yellow-600",
        };
      case "rejected":
        return {
          variant: "destructive" as const,
          label: t("myPostsPage.rejected"),
          icon: XCircle,
          className: "",
        };
      default:
        return {
          variant: "secondary" as const,
          label: status,
          icon: AlertCircle,
          className: "",
        };
    }
  };

  if (loading && posts.length === 0) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <div className="flex items-center justify-between mb-6">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="h-10 w-24" />
        </div>
        {[1, 2, 3].map((i) => (
          <Card key={i} className="mb-4">
            <CardContent className="p-6">
              <Skeleton className="h-6 w-3/4 mb-4" />
              <Skeleton className="h-4 w-1/2" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <PenLine className="w-6 h-6" />
          {t("myPostsPage.myPosts")}
        </h1>
        {user?.role !== "guest" && (
          <Button asChild>
            <Link to="/write">
              <Plus className="w-4 h-4 mr-2" />
              {t("myPostsPage.writePost")}
            </Link>
          </Button>
        )}
      </div>

      {/* Post list */}
      {posts.length === 0 ? (
        <Card>
          <CardContent className="p-12 text-center">
            <div className="w-16 h-16 bg-muted rounded-full flex items-center justify-center mx-auto mb-4">
              <FileX className="w-8 h-8 text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold mb-2">
              {t("myPostsPage.noPosts")}
            </h3>
            <p className="text-muted-foreground mb-4">
              {t("myPostsPage.noPostsDesc")}
            </p>
            {user?.role !== "guest" && (
              <Button asChild>
                <Link to="/write">
                  <Plus className="w-4 h-4 mr-2" />
                  {t("myPostsPage.writePost")}
                </Link>
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="space-y-4">
            {posts.map((post) => (
              <Card key={post.id}>
                <CardContent className="p-6">
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1">
                      {/* Status badge */}
                      <div className="flex items-center gap-2 mb-2">
                        {(() => {
                          const status = getStatusBadge(post.status);
                          const IconComponent = status.icon;
                          return (
                            <Badge
                              variant={status.variant}
                              className={status.className}
                            >
                              <IconComponent className="w-3 h-3 mr-1" />
                              {status.label}
                            </Badge>
                          );
                        })()}
                        <span className="text-sm text-muted-foreground flex items-center gap-1">
                          <Clock className="w-3 h-3" />
                          {formatDate(post.createdAt)}
                        </span>
                      </div>

                      {/* Title */}
                      <Link to={`/post/${post.slug}`}>
                        <h2 className="text-lg font-semibold mb-2 hover:text-primary transition-colors">
                          {post.title}
                        </h2>
                      </Link>

                      {/* Excerpt */}
                      <p className="text-muted-foreground text-sm line-clamp-2 mb-3">
                        {post.excerpt}
                      </p>

                      {/* Tags */}
                      {post.tags && post.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1">
                          {post.tags.slice(0, 3).map((tag) => (
                            <Badge
                              key={tag}
                              variant="outline"
                              className="text-xs"
                            >
                              {tag}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </div>

                    {/* Action buttons */}
                    <div className="flex flex-col gap-2">
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/post/${post.slug}`}>
                          <Eye className="w-4 h-4" />
                        </Link>
                      </Button>
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/edit-post/${post.id}`}>
                          <Edit className="w-4 h-4" />
                        </Link>
                      </Button>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="destructive" size="sm">
                            <Trash2 className="w-4 h-4" />
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
                              onClick={() => handleDeletePost(post.id)}
                              className="bg-destructive"
                            >
                              {t("myPostsPage.delete")}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          {/* Pagination */}
          {pagination.totalPages > 1 && (
            <Pagination className="mt-6">
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevious
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
                        <span className="px-2 text-muted-foreground">...</span>
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
