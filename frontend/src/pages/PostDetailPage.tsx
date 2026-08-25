import { useCallback, useEffect, useState } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { postsApi, commentsApi, likesApi } from "@/lib/api";
import type { Post, Comment } from "@/types";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";

import { Textarea } from "@/components/ui/textarea";
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
  Heart,
  MessageCircle,
  Calendar,
  ArrowLeft,
  Share2,
  Edit,
  Trash2,
  Send,
  Clock,
  Eye,
  XCircle,
} from "lucide-react";
import { normalizeTagsArray } from "@/lib/utils";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";
import { getLocale } from "@/lib/i18n";

export function PostDetailPage() {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const { user, isAuthenticated } = useAuth();
  const { t } = useTranslation();
  // Check whether initial data is available
  const initialData = (
    window as Window & { __INITIAL_DATA__?: { post?: Post } }
  ).__INITIAL_DATA__;

  // Normalize tags in the initial data
  const processedInitialData = initialData?.post
    ? {
        ...initialData.post,
        tags: normalizeTagsArray(initialData.post.tags),
      }
    : null;

  // Initialize state with the processed initialData so client and server rendering match
  const [post, setPost] = useState<Post | null>(processedInitialData || null);
  const [comments, setComments] = useState<Comment[]>([]);
  // When SSR already provided the post, render directly without waiting for the API; otherwise show the loading state
  const [loading, setLoading] = useState(!processedInitialData);
  const [commentContent, setCommentContent] = useState("");
  const [isLiked, setIsLiked] = useState(false);
  const [likesCount, setLikesCount] = useState(
    initialData?.post?.likesCount || 0,
  );
  const [submittingComment, setSubmittingComment] = useState(false);
  const [shareSuccess, setShareSuccess] = useState(false);

  const loadPost = useCallback(async () => {
    try {
      console.log("Loading post by slug:", slug);
      const response = await postsApi.getPost(slug!);
      console.log("Post loaded successfully:", response.data);
      const p = response.data.post;
      p.tags = normalizeTagsArray(p.tags);
      setPost(p);
      setLikesCount(response.data.post.likesCount || 0);
    } catch (error: unknown) {
      console.error("Failed to load post:", error);
      const axiosError = error as {
        message?: string;
        response?: { status?: number; data?: unknown };
      };
      console.error("Error details:", {
        message: axiosError.message,
        response: axiosError.response,
        status: axiosError.response?.status,
        data: axiosError.response?.data,
      });
      // Only redirect back to the homepage if you truly cannot find the article.
      if (axiosError.response?.status === 404) {
        navigate("/");
      }
      // For other errors, still set loading to false so that users can see the error message.
    } finally {
      setLoading(false);
    }
  }, [slug, navigate]);

  const loadComments = useCallback(async () => {
    if (!post?.id) return [] as Comment[];
    try {
      const response = await commentsApi.getComments(post.id);
      setComments(response.data.comments);
      return response.data.comments;
    } catch (error) {
      console.error("Failed to load comments:", error);
      return [] as Comment[];
    }
  }, [post?.id]);

  const loadLikeStatus = useCallback(async () => {
    if (!post?.id) return;
    try {
      const response = await likesApi.getLikeStatus(post.id);
      setIsLiked(response.data.isLiked);
      setLikesCount(response.data.likesCount);
    } catch (error) {
      console.error("Failed to load like status:", error);
    }
  }, [post?.id]);

  useEffect(() => {
    const loadData = async () => {
      if (slug) {
        try {
          // Load from the API when there is no initial data
          if (!initialData?.post) {
            setLoading(true);
            await loadPost();
            setLoading(false);
          }
        } catch (error) {
          console.error("Failed to load data:", error);
          if (!initialData?.post) {
            setLoading(false);
          }
        }
      }
    };

    loadData();
  }, [slug, loadPost, initialData?.post]);

  // Load comments and likes when post is loaded
  useEffect(() => {
    if (post?.id) {
      loadComments();
      loadLikeStatus();
    }
  }, [post?.id, loadComments, loadLikeStatus]);

  const handleLike = async () => {
    if (!isAuthenticated) {
      navigate("/login");
      return;
    }
    if (!post?.id) return;

    try {
      const response = await likesApi.toggleLike(post.id);
      setIsLiked(response.data.isLiked);
      setLikesCount(response.data.likesCount);
      // Notify other pages (e.g. the home page) to update the post's like status
      try {
        window.dispatchEvent(
          new CustomEvent("like-changed", {
            detail: {
              postId: post.id,
              isLiked: response.data.isLiked,
              likesCount: response.data.likesCount,
            },
          }),
        );
      } catch {
        // ignore
      }
    } catch (error) {
      console.error("Failed to like:", error);
    }
  };

  const handleSubmitComment = async () => {
    if (!isAuthenticated) {
      navigate("/login");
      return;
    }
    if (!post?.id) return;

    if (!commentContent.trim()) return;

    setSubmittingComment(true);
    try {
      const response = await commentsApi.createComment({
        postId: post.id,
        content: commentContent.trim(),
      });
      setCommentContent("");
      await loadComments();
      // Sync the home page using the commentsCount returned by the backend
      const newCount = response.data.commentsCount ?? comments.length + 1;
      try {
        window.dispatchEvent(
          new CustomEvent("comment-changed", {
            detail: { postId: post.id, commentsCount: newCount },
          }),
        );
      } catch {
        // ignore
      }
    } catch (error) {
      console.error("Failed to post comment:", error);
    } finally {
      setSubmittingComment(false);
    }
  };

  const handleDeletePost = async () => {
    if (!post?.id) return;
    try {
      await postsApi.deletePost(post.id);
      navigate("/");
    } catch (error) {
      console.error("Failed to delete post:", error);
    }
  };

  const handleDeleteComment = async (commentId: string) => {
    if (!post?.id) return;
    try {
      const response = await commentsApi.deleteComment(commentId);
      await loadComments();
      const newCount =
        response.data.commentsCount ??
        (comments.length > 0 ? comments.length - 1 : 0);
      try {
        window.dispatchEvent(
          new CustomEvent("comment-changed", {
            detail: { postId: post.id, commentsCount: newCount },
          }),
        );
      } catch {
        // ignore
      }
    } catch (error) {
      console.error("Failed to delete comment:", error);
    }
  };

  const handleShare = async () => {
    try {
      const postUrl = `${window.location.origin}/post/${post?.slug || slug}`;
      await navigator.clipboard.writeText(postUrl);
      setShareSuccess(true);
      // Hide the success hint after 3 seconds
      setTimeout(() => {
        setShareSuccess(false);
      }, 3000);
    } catch (error) {
      console.error("Failed to copy link:", error);
    }
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const canEditPost =
    user &&
    post &&
    (user.id === post.authorId ||
      user.role === "admin" ||
      user.role === "super_admin");
  const canDeletePost =
    user &&
    post &&
    (user.id === post.authorId ||
      user.role === "admin" ||
      user.role === "super_admin");

  if (loading || !post) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <Skeleton className="h-8 w-3/4 mb-4" />
        <Skeleton className="h-4 w-1/2 mb-8" />
        <Skeleton className="h-64 w-full mb-8" />
        <Skeleton className="h-4 w-full mb-2" />
        <Skeleton className="h-4 w-full mb-2" />
        <Skeleton className="h-4 w-2/3" />
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* Back button */}
      <Button variant="ghost" size="sm" asChild className="mb-6">
        <Link to="/" className="flex items-center gap-2">
          <ArrowLeft className="w-4 h-4" />
          {t("postDetailPage.backToHome")}
        </Link>
      </Button>

      {/* Post header */}
      <div className="mb-8">
        {/* Post status and rejection reason */}
        {post.status === "rejected" && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
            <div className="flex items-center gap-2 mb-1">
              <XCircle className="w-5 h-5 text-red-500" />
              <span className="font-medium text-red-800">
                {t("postDetailPage.rejectedArticle")}
              </span>
            </div>
            {post.rejectionReason && (
              <p className="text-sm text-red-700">
                {t("postDetailPage.rejectionReason")}
                {post.rejectionReason}
              </p>
            )}
          </div>
        )}

        {/* Category and tags */}
        <div className="flex flex-wrap items-center gap-2 mb-4">
          {post.categoryInfo && (
            <Badge variant="secondary">{post.categoryInfo.name}</Badge>
          )}
          {post.tags?.map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
        </div>

        {/* Title */}
        <h1 className="text-3xl md:text-4xl font-bold mb-6">{post.title}</h1>

        {/* Author info */}
        <div className="flex items-center justify-between flex-wrap gap-4">
          <Link
            to={`/user/${post.author?.id}`}
            className="flex items-center gap-4 hover:no-underline"
          >
            <Avatar className="w-12 h-12">
              {post.author?.avatar ? (
                <img
                  src={post.author.avatar}
                  alt="Avatar"
                  className="w-full h-full object-cover"
                />
              ) : (
                <AvatarFallback className="bg-primary/10 text-primary text-lg">
                  {post.author?.username?.charAt(0).toUpperCase()}
                </AvatarFallback>
              )}
            </Avatar>
            <div>
              <p className="font-medium hover:text-primary transition-colors">
                {post.author?.username}
              </p>
              <div className="flex items-center gap-3 text-sm text-muted-foreground">
                <span className="flex items-center gap-1">
                  <Calendar className="w-4 h-4" />
                  {formatDate(post.createdAt)}
                </span>
                {post.updatedAt !== post.createdAt && (
                  <span className="flex items-center gap-1">
                    <Clock className="w-4 h-4" />
                    {t("postDetailPage.updatedAt")} {formatDate(post.updatedAt)}
                  </span>
                )}
              </div>
            </div>
          </Link>

          {/* Action buttons */}
          <div className="flex items-center gap-2">
            {canEditPost && (
              <Button variant="outline" size="sm" asChild>
                <Link to={`/edit-post/${post.id}`}>
                  <Edit className="w-4 h-4 mr-1" />
                  {t("postDetailPage.edit")}
                </Link>
              </Button>
            )}
            {canDeletePost && (
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive" size="sm">
                    <Trash2 className="w-4 h-4 mr-1" />
                    {t("postDetailPage.delete")}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      {t("postDetailPage.confirmDelete")}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      {t("postDetailPage.cannotUndo")}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>
                      {t("postDetailPage.cancel")}
                    </AlertDialogCancel>
                    <AlertDialogAction
                      onClick={handleDeletePost}
                      className="bg-destructive"
                    >
                      {t("postDetailPage.deleteButton")}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            )}
          </div>
        </div>
      </div>

      {/* Cover image */}
      {post.coverImage && (
        <div className="mb-8 rounded-lg overflow-hidden">
          <img
            src={post.coverImage}
            alt={post.title}
            className="w-full max-h-[400px] object-fill"
          />
        </div>
      )}

      {/* Post content */}
      <div className="mb-12">
        <MarkdownRenderer content={post.content} />
      </div>

      {/* Interaction area */}
      <div className="flex items-center justify-between py-6 border-y">
        <div className="flex items-center gap-4">
          <Button
            variant={isLiked ? "default" : "outline"}
            size="lg"
            onClick={handleLike}
            className="flex items-center gap-2"
          >
            <Heart className={`w-5 h-5 ${isLiked ? "fill-current" : ""}`} />
            <span>{likesCount}</span>
          </Button>
          <Button
            variant="outline"
            size="lg"
            className="flex items-center gap-2"
            onClick={() =>
              document
                .getElementById("comments")
                ?.scrollIntoView({ behavior: "smooth" })
            }
          >
            <MessageCircle className="w-5 h-5" />
            <span>
              {t("postDetailPage.comments")} ({comments.length})
            </span>
          </Button>
          <div className="flex items-center gap-1 text-sm text-muted-foreground">
            <Eye className="w-5 h-5" />
            <span>{post.viewCount || 0}</span>
          </div>
        </div>
        <div className="relative">
          <Button variant="ghost" size="icon" onClick={handleShare}>
            <Share2 className="w-5 h-5" />
          </Button>
          {shareSuccess && (
            <div className="absolute -bottom-6 left-1/2 transform -translate-x-1/2 text-[10px] text-muted-foreground bg-muted px-2 py-1 rounded whitespace-nowrap">
              {t("postDetailPage.copyLink")}
            </div>
          )}
        </div>
      </div>

      {/* Comments section */}
      <div id="comments" className="mt-12">
        <h2 className="text-2xl font-bold mb-6 flex items-center gap-2">
          <MessageCircle className="w-6 h-6" />
          {t("postDetailPage.comments")} ({comments.length})
        </h2>

        {/* Write a comment */}
        {isAuthenticated ? (
          <Card className="mb-8">
            <CardContent className="p-4">
              <div className="relative">
                <Textarea
                  placeholder={t("postDetailPage.commentPlaceholder")}
                  value={commentContent}
                  onChange={(e) => setCommentContent(e.target.value)}
                  className="mb-4 min-h-[100px]"
                  maxLength={100}
                />
                <div className="absolute bottom-6 right-4 text-sm text-muted-foreground">
                  {commentContent.length}/100
                </div>
              </div>
              <div className="flex justify-end">
                <Button
                  onClick={handleSubmitComment}
                  disabled={!commentContent.trim() || submittingComment}
                >
                  <Send className="w-4 h-4 mr-2" />
                  {submittingComment
                    ? t("postDetailPage.submitting")
                    : t("postDetailPage.submitComment")}
                </Button>
              </div>
            </CardContent>
          </Card>
        ) : (
          <Card className="mb-8">
            <CardContent className="p-6 text-center">
              <p className="text-muted-foreground mb-4">
                {t("postDetailPage.loginToComment")}
              </p>
              <Button asChild>
                <Link to="/login">{t("auth.login")}</Link>
              </Button>
            </CardContent>
          </Card>
        )}

        {/* Comment list */}
        <div className="space-y-4">
          {comments.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              {t("postDetailPage.noComments")}
            </div>
          ) : (
            comments.map((comment) => (
              <Card key={comment.id}>
                <CardContent className="p-4">
                  <div className="flex items-start gap-4">
                    <Avatar className="w-10 h-10">
                      {comment.author?.avatar ? (
                        <img
                          src={comment.author.avatar}
                          alt="Avatar"
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <AvatarFallback className="bg-primary/10 text-primary text-sm">
                          {comment.author?.username?.charAt(0).toUpperCase()}
                        </AvatarFallback>
                      )}
                    </Avatar>
                    <div className="flex-1">
                      <div className="flex items-center justify-between mb-2">
                        <div className="flex items-center gap-2">
                          <span className="font-medium">
                            {comment.author?.username}
                          </span>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(comment.createdAt)}
                          </span>
                        </div>
                        {user &&
                          (user.id === comment.userId ||
                            user.role === "admin" ||
                            user.role === "super_admin") && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleDeleteComment(comment.id)}
                            >
                              <Trash2 className="w-4 h-4" />
                            </Button>
                          )}
                      </div>
                      <p className="text-gray-700">{comment.content}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
