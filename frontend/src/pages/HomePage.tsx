import { useCallback, useEffect, useState } from "react";
import { Link, useSearchParams, useNavigate } from "react-router-dom";
import { postsApi, categoriesApi, statsApi, likesApi } from "@/lib/api";
import type { Post, Category } from "@/types";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  Heart,
  MessageCircle,
  Calendar,
  TrendingUp,
  Clock,
  Tag,
  SearchX,
  Eye,
} from "lucide-react";
import { normalizeTagsArray } from "@/lib/utils";

export function HomePage() {
  const { t } = useTranslation();
  const { isAuthenticated } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const [posts, setPosts] = useState<Post[]>([]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [popularPosts, setPopularPosts] = useState<Post[]>([]);
  const [popularTags, setPopularTags] = useState<
    { name: string; count: number }[]
  >([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({
    total: 0,
    page: 1,
    totalPages: 1,
    limit: 10,
  });

  const currentPage = parseInt(searchParams.get("page") || "1");
  const searchQuery = searchParams.get("search") || "";
  const selectedCategory = searchParams.get("category") || "";

  useEffect(() => {
    loadCategories();
    loadPopularPosts();
    loadPopularTags();
    // Listen for like events from the post detail page to keep the home page in sync
    const handler = (e: Event) => {
      try {
        const d = (e as CustomEvent).detail || {};
        const postId = Number(d.postId);
        setPosts((prev) =>
          prev.map((p) =>
            p.id === postId
              ? { ...p, isLiked: d.isLiked, likesCount: d.likesCount }
              : p,
          ),
        );
        setPopularPosts((prev) =>
          prev.map((p) =>
            p.id === postId
              ? { ...p, isLiked: d.isLiked, likesCount: d.likesCount }
              : p,
          ),
        );
      } catch {
        // ignore
      }
    };
    window.addEventListener("like-changed", handler as EventListener);

    const commentHandler = (e: Event) => {
      try {
        const d = (e as CustomEvent).detail || {};
        const postId = Number(d.postId);
        setPosts((prev) =>
          prev.map((p) =>
            p.id === postId ? { ...p, commentsCount: d.commentsCount } : p,
          ),
        );
      } catch {
        // ignore
      }
    };
    window.addEventListener("comment-changed", commentHandler as EventListener);

    return () => {
      window.removeEventListener("like-changed", handler as EventListener);
      window.removeEventListener(
        "comment-changed",
        commentHandler as EventListener,
      );
    };
  }, []);

  // Normalize posts returned by the backend: id/authorId as strings, timestamps as ISO strings
  const normalizePost = (raw: Partial<Post>): Post => {
    if (!raw) return raw as Post;
    return {
      ...raw,
      id: Number(raw.id),
      authorId:
        raw.authorId !== undefined && raw.authorId !== null
          ? Number(raw.authorId)
          : raw.authorId,
      createdAt: raw.createdAt
        ? new Date(raw.createdAt).toISOString()
        : raw.createdAt,
      updatedAt: raw.updatedAt
        ? new Date(raw.updatedAt).toISOString()
        : raw.updatedAt,
      tags: normalizeTagsArray(raw.tags),
    } as Post;
  };

  const loadPosts = useCallback(async () => {
    setLoading(true);
    try {
      if (searchQuery && searchQuery.trim()) {
        // Prefer the backend title search (pagination-friendly); also fetch extra posts for client-side tag matching as a fallback
        const [respSearch, respBulk] = await Promise.all([
          postsApi.getPosts({
            page: currentPage,
            limit: 10,
            search: searchQuery,
            category: selectedCategory || undefined,
          }),
          // Fetch more posts to match tags on the client (the backend may not support tag search)
          postsApi.getPosts({
            page: 1,
            limit: 200,
            category: selectedCategory || undefined,
          }),
        ]);
        const titleMatches = (respSearch.data.posts || []).map((p: any) =>
          normalizePost(p),
        );
        const bulk = (respBulk.data.posts || []).map((p: any) => normalizePost(p));
        const q = searchQuery.trim().toLowerCase();
        const tagMatches = bulk.filter((p) =>
          (p.tags || []).some((t: string) =>
            String(t).toLowerCase().includes(q),
          ),
        );
        const combinedMap = new Map<string, Post>();
        titleMatches.forEach((p) => combinedMap.set(p.id, p));
        tagMatches.forEach((p) => combinedMap.set(p.id, p));
        const combined = Array.from(combinedMap.values());
        setPosts(combined);
        // Use the title search pagination info as the page pagination reference
        setPagination(respSearch.data.pagination);
      } else {
        const response = await postsApi.getPosts({
          page: currentPage,
          limit: 10,
          category: selectedCategory || undefined,
        });
        const all = response.data.posts.map((p) => normalizePost(p));
        setPosts(all);
        setPagination(response.data.pagination);
      }
    } catch (error) {
      console.error("Failed to load posts:", error);
    } finally {
      setLoading(false);
    }
  }, [currentPage, searchQuery, selectedCategory]);

  useEffect(() => {
    loadPosts();
  }, [currentPage, searchQuery, selectedCategory, loadPosts]);

  const loadCategories = async () => {
    try {
      const response = await categoriesApi.getCategories();
      setCategories(response.data.categories);
    } catch (error) {
      console.error("Failed to load categories:", error);
    }
  };

  const loadPopularPosts = async () => {
    try {
      const response = await statsApi.getPopularPosts(5);
      setPopularPosts(response.data.posts.map((p) => normalizePost(p)));
    } catch (error) {
      console.error("Failed to load popular posts:", error);
    }
  };

  const loadPopularTags = async () => {
    try {
      // Fetch enough posts to tally tags
      const response = await postsApi.getPosts({ page: 1, limit: 200 });
      const allPosts = response.data.posts.map((p) => normalizePost(p));

      // Count how many times each tag appears
      const tagCounts: Record<string, number> = {};
      allPosts.forEach((post) => {
        post.tags?.forEach((tag: string) => {
          if (tag) {
            tagCounts[tag] = (tagCounts[tag] || 0) + 1;
          }
        });
      });

      // Convert to an array and sort by occurrence count
      const sortedTags = Object.entries(tagCounts)
        .map(([name, count]) => ({ name, count }))
        .sort((a, b) => b.count - a.count)
        .slice(0, 10); // keep the top 10

      setPopularTags(sortedTags);
    } catch (error) {
      console.error("Failed to load popular tags:", error);
    }
  };

  const handlePageChange = (page: number) => {
    const newParams = new URLSearchParams(searchParams);
    if (page === 1) {
      newParams.delete("page");
    } else {
      newParams.set("page", page.toString());
    }
    setSearchParams(newParams);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const handleCategoryClick = (categoryName: string) => {
    const newParams = new URLSearchParams(searchParams);
    if (selectedCategory === categoryName) {
      newParams.delete("category");
    } else {
      newParams.set("category", categoryName);
    }
    newParams.delete("page");
    setSearchParams(newParams);
  };

  const handleTagClick = (tagName: string) => {
    if (!isAuthenticated) {
      navigate("/login");
      return;
    }
    const newParams = new URLSearchParams(searchParams);
    newParams.set("search", tagName);
    newParams.delete("page");
    setSearchParams(newParams);
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  const handleToggleLike = async (postId: number | string) => {
    try {
      const response = await likesApi.toggleLike(postId);
      const { isLiked, likesCount } = response.data;
      setPosts((prev) =>
        prev.map((p) => (p.id === postId ? { ...p, isLiked, likesCount } : p)),
      );
      setPopularPosts((prev) =>
        prev.map((p) => (p.id === postId ? { ...p, isLiked, likesCount } : p)),
      );
    } catch (error) {
      console.error("Failed to toggle like:", error);
    }
  };

  if (loading && posts.length === 0) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
          <div className="lg:col-span-3 space-y-6">
            {[1, 2, 3].map((i) => (
              <Card key={i}>
                <CardContent className="p-6">
                  <Skeleton className="h-6 w-3/4 mb-4" />
                  <Skeleton className="h-4 w-full mb-2" />
                  <Skeleton className="h-4 w-2/3" />
                </CardContent>
              </Card>
            ))}
          </div>
          <div className="space-y-6">
            <Skeleton className="h-48" />
            <Skeleton className="h-48" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8">
      {/* Search hint */}
      {searchQuery && (
        <div className="mb-6 flex items-center gap-2">
          <SearchX className="w-5 h-5 text-muted-foreground" />
          <span className="text-muted-foreground">
            {t("homePage.searchResultsFor", { query: searchQuery })}
          </span>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              const newParams = new URLSearchParams(searchParams);
              newParams.delete("search");
              setSearchParams(newParams);
            }}
          >
            {t("homePage.clearSearch")}
          </Button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
        {/* Main content area */}
        <div className="lg:col-span-3 space-y-6">
          {posts.length === 0 ? (
            <Card>
              <CardContent className="p-12 text-center">
                <div className="w-16 h-16 bg-muted rounded-full flex items-center justify-center mx-auto mb-4">
                  <SearchX className="w-8 h-8 text-muted-foreground" />
                </div>
                <h3 className="text-lg font-semibold mb-2">
                  {t("homePage.noPostsFound")}
                </h3>
                <p className="text-muted-foreground">
                  {searchQuery
                    ? t("homePage.tryDifferentKeywords")
                    : t("homePage.noPostsAvailable")}
                </p>
              </CardContent>
            </Card>
          ) : (
            <>
              <div className="space-y-6">
                {posts.map((post) => (
                  <Card
                    key={post.id}
                    className="group hover:shadow-lg transition-shadow p-0 gap-0 overflow-hidden"
                  >
                    {/* Cover image (placed at the top of the Card so it sits flush with the card edge) */}
                    {post.coverImage && (
                      <Link to={`/post/${post.slug}`} className="block">
                        <div className="w-full overflow-hidden rounded-t-xl">
                          <img
                            src={post.coverImage}
                            alt={post.title}
                            className="w-full h-auto object-contain group-hover:scale-105 transition-transform duration-300"
                          />
                        </div>
                      </Link>
                    )}

                    <CardContent className="p-6">
                      {/* Category and tags */}
                      <div className="flex flex-wrap items-center gap-2 mb-3">
                        {post.categoryInfo && (
                          <Badge variant="secondary">
                            {post.categoryInfo.name}
                          </Badge>
                        )}
                        {post.tags?.slice(0, 3).map((tag) => (
                          <Badge
                            key={tag}
                            variant="outline"
                            className="text-xs"
                          >
                            {tag}
                          </Badge>
                        ))}
                      </div>

                      {/* Title */}
                      <Link to={`/post/${post.slug}`}>
                        <h2 className="text-xl font-bold mb-3 group-hover:text-primary transition-colors line-clamp-2">
                          {post.title}
                        </h2>
                      </Link>

                      {/* Excerpt */}
                      <p className="text-muted-foreground mb-4 line-clamp-2">
                        {post.excerpt}
                      </p>

                      {/* Author and stats */}
                      <div className="flex flex-col gap-2">
                        <Link
                          to={`/user/${post.author?.id}`}
                          className="flex items-center gap-3 hover:no-underline"
                        >
                          <Avatar className="w-8 h-8">
                            {post.author?.avatar ? (
                              <img
                                src={post.author.avatar}
                                alt="Avatar"
                                className="w-full h-full object-cover"
                              />
                            ) : (
                              <AvatarFallback className="bg-primary/10 text-primary text-sm">
                                {post.author?.username?.charAt(0).toUpperCase()}
                              </AvatarFallback>
                            )}
                          </Avatar>
                          <div className="flex flex-col gap-1">
                            <div className="flex items-center gap-2 text-sm">
                              <span className="text-muted-foreground hover:text-primary transition-colors">
                                {post.author?.username}
                              </span>
                              <span className="text-muted-foreground">·</span>
                              <span className="flex items-center gap-1 text-muted-foreground">
                                <Calendar className="w-3 h-3" />
                                {formatDate(post.createdAt)}
                              </span>
                            </div>
                            {/* Birthday and bio */}
                            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                              {post.author?.birthday &&
                                !JSON.parse(
                                  localStorage.getItem("userSettings") || "{}",
                                ).hideBirthday && (
                                  <span className="flex items-center gap-1">
                                    <Calendar className="w-2.5 h-2.5" />
                                    {new Date(
                                      post.author.birthday,
                                    ).toLocaleDateString(getLocale(), {
                                      month: "short",
                                      day: "numeric",
                                    })}
                                  </span>
                                )}
                              {post.author?.bio &&
                                !JSON.parse(
                                  localStorage.getItem("userSettings") || "{}",
                                ).hideBio && <span>{post.author.bio}</span>}
                            </div>
                          </div>
                        </Link>

                        <div className="flex items-center gap-4 text-sm">
                          <button
                            onClick={() => handleToggleLike(post.id)}
                            className={`flex items-center gap-1 ${post.isLiked ? "text-red-500" : "text-muted-foreground"}`}
                          >
                            <Heart
                              className={`w-4 h-4 ${post.isLiked ? "fill-current" : ""}`}
                            />
                            <span>{post.likesCount || 0}</span>
                          </button>
                          <span className="flex items-center gap-1 text-muted-foreground">
                            <MessageCircle className="w-4 h-4" />
                            {post.commentsCount || 0}
                          </span>
                          <span className="flex items-center gap-1 text-muted-foreground">
                            <Eye className="w-4 h-4" />
                            {post.viewCount || 0}
                          </span>
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>

              {/* Pagination */}
              {pagination.totalPages > 1 && (
                <Pagination>
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

                    {Array.from(
                      { length: pagination.totalPages },
                      (_, i) => i + 1,
                    )
                      .filter(
                        (page) =>
                          page === 1 ||
                          page === pagination.totalPages ||
                          Math.abs(page - currentPage) <= 1,
                      )
                      .map((page, index, array) => (
                        <div key={page} className="flex items-center">
                          {index > 0 && array[index - 1] !== page - 1 && (
                            <span className="px-2 text-muted-foreground">
                              ...
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

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Categories */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Tag className="w-4 h-4" />
                {t("homePage.categories")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {categories.map((category) => (
                  <Badge
                    key={category.id}
                    variant={
                      selectedCategory === category.name
                        ? "default"
                        : "secondary"
                    }
                    className="cursor-pointer"
                    onClick={() => handleCategoryClick(category.name)}
                  >
                    {category.name}
                  </Badge>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Popular tags */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Tag className="w-4 h-4" />
                {t("homePage.popularTags")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2">
                {popularTags.map((tag) => (
                  <Badge
                    key={tag.name}
                    variant="secondary"
                    className="cursor-pointer"
                    onClick={() => handleTagClick(tag.name)}
                  >
                    {tag.name} ({tag.count})
                  </Badge>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* Popular posts */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <TrendingUp className="w-4 h-4" />
                {t("homePage.popularPosts")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {popularPosts.map((post, index) => (
                  <Link
                    key={post.id}
                    to={`/post/${post.slug}`}
                    className="flex items-start gap-3 group"
                  >
                    <span className="text-lg font-bold text-muted-foreground w-6">
                      {index + 1}
                    </span>
                    <div>
                      <h4 className="text-sm font-medium line-clamp-2 group-hover:text-primary transition-colors">
                        {post.title}
                      </h4>
                      <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                        <div className="flex items-center gap-1">
                          <Heart className="w-3 h-3" />
                          {post.likesCount || 0}
                        </div>
                        <div className="flex items-center gap-1">
                          <Eye className="w-3 h-3" />
                          {post.viewCount || 0}
                        </div>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </CardContent>
          </Card>

          {/* About */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg flex items-center gap-2">
                <Clock className="w-4 h-4" />
                {t("homePage.aboutVexgo")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                {t("homePage.aboutVexgoDesc")}
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
