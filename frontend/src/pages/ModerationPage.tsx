import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import type { Post } from "@/types";
import { Button } from "@/components/ui/button";
import { Badge, type badgeVariants } from "@/components/ui/badge";
import type { VariantProps } from "class-variance-authority";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { PageHeader } from "@/components/PageHeader";
import {
  CheckCircle,
  Clock,
  Edit,
  Eye,
  Search,
  Send,
  XCircle,
} from "lucide-react";
import { toast } from "sonner";

type BadgeVariant = VariantProps<typeof badgeVariants>["variant"];

/**
 * One queued post. The row sits directly on the page rule instead of inside
 * its own bordered box: the queue is a list of items to get through, and
 * boxing each one nested a card inside the panel that already had a border.
 * The title takes the display face so scanning the queue reads like scanning
 * a table of contents.
 */
function ReviewRow({
  post,
  status,
  variant,
  date,
  actions,
}: {
  post: Post;
  status: string;
  variant: BadgeVariant;
  date: string;
  actions: ReactNode;
}) {
  const { t } = useTranslation();

  return (
    <li className="flex flex-col gap-4 py-5 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <Badge variant={variant}>{status}</Badge>
          <span className="text-2xs text-muted-foreground tabular-nums">
            {date}
          </span>
        </div>
        <h3 className="display text-subtitle mt-2">{post.title}</h3>
        <p className="text-xs text-muted-foreground mt-1">
          {t("moderation.author")}: {post.author?.username}
        </p>
        {post.excerpt && (
          <p className="text-sm leading-relaxed text-muted-foreground mt-2 line-clamp-2">
            {post.excerpt}
          </p>
        )}
        {post.rejectionReason && (
          <p className="border-destructive mt-3 border-l-2 pl-3 text-xs leading-relaxed text-muted-foreground">
            <span className="text-destructive font-medium">
              {t("moderation.rejectionReasonInPost")}
            </span>{" "}
            {post.rejectionReason}
          </p>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2 sm:shrink-0 sm:justify-end">
        {actions}
      </div>
    </li>
  );
}

export function ModerationPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [pendingPosts, setPendingPosts] = useState<Post[]>([]);
  const [approvedPosts, setApprovedPosts] = useState<Post[]>([]);
  const [rejectedPosts, setRejectedPosts] = useState<Post[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState("pending");
  const [rejectingPostId, setRejectingPostId] = useState<
    string | number | null
  >(null);
  const [rejectionReason, setRejectionReason] = useState("");
  const [showRejectDialog, setShowRejectDialog] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");

  const loadData = useCallback(
    async (search?: string) => {
      setLoading(true);
      try {
        if (activeTab === "pending") {
          const response = await unwrap(
            getVexGoAPI().getModerationPending({ limit: 100, search }),
          );
          setPendingPosts((response.posts as Post[]) || []);
        } else if (activeTab === "approved") {
          const response = await unwrap(
            getVexGoAPI().getModerationApproved({ limit: 100, search }),
          );
          setApprovedPosts((response.posts as Post[]) || []);
        } else if (activeTab === "rejected") {
          const response = await unwrap(
            getVexGoAPI().getModerationRejected({ limit: 100, search }),
          );
          setRejectedPosts((response.posts as Post[]) || []);
        }
      } catch (error) {
        console.error("Failed to load data:", error);
        toast.error(t("moderation.loadFailed"));
      } finally {
        setLoading(false);
      }
    },
    [activeTab, t],
  );

  useEffect(() => {
    loadData();
  }, [activeTab, loadData]);

  const handleSearch = async () => {
    await loadData(searchTerm);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  const handleTabChange = (tab: string) => {
    setActiveTab(tab);
    setSearchTerm("");
  };

  const handleApprovePost = async (postId: string | number) => {
    try {
      await unwrap(getVexGoAPI().putModerationApproveId(String(postId)));
      toast.success(t("moderation.approveSuccess"));
      loadData();
    } catch (error) {
      console.error("Failed to approve:", error);
      toast.error(t("moderation.approveFailed"));
    }
  };

  const handleRejectPost = async (postId: string | number) => {
    setRejectingPostId(postId);
    setShowRejectDialog(true);
    setRejectionReason("");
  };

  const confirmRejectPost = async () => {
    if (!rejectingPostId) return;

    try {
      await unwrap(
        getVexGoAPI().putModerationRejectId(String(rejectingPostId), {
          rejectionReason,
        }),
      );
      toast.success(t("moderation.rejectSuccess"));
      setShowRejectDialog(false);
      setRejectingPostId(null);
      setRejectionReason("");
      loadData();
    } catch (error) {
      console.error("Failed to reject post:", error);
      toast.error(t("moderation.rejectFailed"));
    }
  };

  const cancelRejectPost = () => {
    setShowRejectDialog(false);
    setRejectingPostId(null);
    setRejectionReason("");
  };

  const handleResubmitPost = async (postId: string | number) => {
    try {
      await unwrap(getVexGoAPI().putModerationResubmitId(String(postId)));
      toast.success(t("moderation.resubmitSuccess"));
      loadData();
    } catch (error) {
      console.error("Failed to resubmit for review:", error);
      toast.error(t("moderation.resubmitFailed"));
    }
  };

  const handleViewPost = (postSlug: string) => {
    // The post page is served by the public theme, outside this SPA.
    window.location.href = `/post/${postSlug}`;
  };

  const handleEditPost = (postId: string | number) => {
    navigate(`/admin/edit-post/${postId}`);
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const viewButton = (post: Post) => (
    <Button
      variant="outline"
      size="sm"
      onClick={() => handleViewPost(String(post.slug))}
    >
      <Eye />
      {t("moderation.view")}
    </Button>
  );

  const editButton = (post: Post) => (
    <Button variant="outline" size="sm" onClick={() => handleEditPost(post.id)}>
      <Edit />
      {t("moderation.edit")}
    </Button>
  );

  if (loading) {
    return (
      <div className="space-y-6">
        <PageHeader title={t("moderation.title")} />
        <div className="border-t border-border">
          {[1, 2, 3].map((i) => (
            <div key={i} className="space-y-3 border-b border-border py-5">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-5 w-2/5" />
              <Skeleton className="h-3 w-1/4" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("moderation.title")}
        actions={
          <div className="flex w-full items-center gap-2 sm:w-auto">
            <div className="relative w-full sm:w-56">
              <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                type="search"
                placeholder={t("moderation.searchPlaceholder")}
                aria-label={t("moderation.searchPlaceholder")}
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                onKeyUp={handleKeyPress}
                className="pl-9"
              />
            </div>
            <Button
              variant="outline"
              size="icon"
              onClick={handleSearch}
              aria-label={t("moderation.searchPlaceholder")}
            >
              <Search />
            </Button>
          </div>
        }
      />

      <Tabs
        value={activeTab}
        onValueChange={handleTabChange}
        className="w-full"
      >
        <TabsList className="w-full">
          <TabsTrigger value="pending" className="gap-2">
            <Clock />
            {t("moderation.pending")}
            <span className="text-2xs text-muted-foreground tabular-nums">
              {pendingPosts.length}
            </span>
          </TabsTrigger>
          <TabsTrigger value="approved" className="gap-2">
            <CheckCircle />
            {t("moderation.approved")}
          </TabsTrigger>
          <TabsTrigger value="rejected" className="gap-2">
            <XCircle />
            {t("moderation.rejected")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending">
          {pendingPosts.length === 0 ? (
            <EmptyState icon={Clock} title={t("moderation.noPendingPosts")} />
          ) : (
            <ul className="divide-y divide-border">
              {pendingPosts.map((post) => (
                <ReviewRow
                  key={post.id}
                  post={post}
                  status={t("moderation.pending")}
                  variant="outline-warning"
                  date={formatDate(post.createdAt || "")}
                  actions={
                    <>
                      {viewButton(post)}
                      {editButton(post)}
                      <Button
                        size="sm"
                        onClick={() => handleApprovePost(post.id)}
                      >
                        <CheckCircle />
                        {t("moderation.approve")}
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() => handleRejectPost(post.id)}
                      >
                        <XCircle />
                        {t("moderation.reject")}
                      </Button>
                    </>
                  }
                />
              ))}
            </ul>
          )}
        </TabsContent>

        <TabsContent value="approved">
          {approvedPosts.length === 0 ? (
            <EmptyState
              icon={CheckCircle}
              title={t("moderation.noApprovedPosts")}
            />
          ) : (
            <ul className="divide-y divide-border">
              {approvedPosts.map((post) => (
                <ReviewRow
                  key={post.id}
                  post={post}
                  status={t("moderation.approved")}
                  variant="outline-success"
                  date={formatDate(post.createdAt || "")}
                  actions={viewButton(post)}
                />
              ))}
            </ul>
          )}
        </TabsContent>

        <TabsContent value="rejected">
          {rejectedPosts.length === 0 ? (
            <EmptyState
              icon={XCircle}
              title={t("moderation.noRejectedPosts")}
            />
          ) : (
            <ul className="divide-y divide-border">
              {rejectedPosts.map((post) => (
                <ReviewRow
                  key={post.id}
                  post={post}
                  status={t("moderation.rejected")}
                  variant="outline-destructive"
                  date={formatDate(post.createdAt || "")}
                  actions={
                    <>
                      {editButton(post)}
                      {viewButton(post)}
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleResubmitPost(post.id)}
                      >
                        <Send />
                        {t("moderation.resubmit")}
                      </Button>
                    </>
                  }
                />
              ))}
            </ul>
          )}
        </TabsContent>
      </Tabs>

      <Dialog
        open={showRejectDialog}
        onOpenChange={(open) => {
          if (!open) cancelRejectPost();
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("moderation.rejectPost")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="rejectionReason">
              {t("moderation.rejectionReasonLabel")}
            </Label>
            <Textarea
              id="rejectionReason"
              value={rejectionReason}
              onChange={(e) => setRejectionReason(e.target.value)}
              placeholder={t("moderation.rejectionReasonPlaceholder")}
              rows={4}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={cancelRejectPost}>
              {t("moderation.cancel")}
            </Button>
            <Button variant="destructive" onClick={confirmRejectPost}>
              {t("moderation.confirmReject")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
