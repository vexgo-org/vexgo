import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import type { Post } from "@/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
  AlertCircle,
  CheckCircle,
  Clock,
  Edit,
  Eye,
  Search,
  Send,
  XCircle,
} from "lucide-react";
import { toast } from "sonner";

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

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-40" />
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-24 rounded-lg" />
        ))}
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
              <Search className="size-4" />
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
          <TabsTrigger value="pending" className="flex items-center gap-2">
            <Clock className="w-4 h-4" />
            {t("moderation.pending")} ({pendingPosts.length})
          </TabsTrigger>
          <TabsTrigger value="approved" className="flex items-center gap-2">
            <CheckCircle className="w-4 h-4" />
            {t("moderation.approved")}
          </TabsTrigger>
          <TabsTrigger value="rejected" className="flex items-center gap-2">
            <XCircle className="w-4 h-4" />
            {t("moderation.rejected")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <AlertCircle className="text-warning size-4" />
                {t("moderation.pendingPosts")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {pendingPosts.length === 0 ? (
                <EmptyState
                  icon={Clock}
                  title={t("moderation.noPendingPosts")}
                />
              ) : (
                <div className="space-y-4">
                  {pendingPosts.map((post) => (
                    <div
                      key={post.id}
                      className="flex items-start justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <Badge variant="secondary">
                            {t("moderation.pending")}
                          </Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt || "")}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">
                          {post.title}
                        </h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          {t("moderation.author")}: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground mb-2 line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                        {post.rejectionReason && (
                          <div className="border-destructive/35 bg-destructive/10 mt-2 rounded-md border p-2">
                            <p className="text-destructive text-[13px]">
                              <span className="font-medium">
                                {t("moderation.rejectionReasonInPost")}
                              </span>
                              {post.rejectionReason}
                            </p>
                          </div>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(String(post.slug))}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          {t("moderation.view")}
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleEditPost(post.id)}
                        >
                          <Edit className="w-4 h-4 mr-1" />
                          {t("moderation.edit")}
                        </Button>
                        <div className="flex gap-1">
                          <Button
                            size="sm"
                            onClick={() => handleApprovePost(post.id)}
                          >
                            <CheckCircle className="w-4 h-4 mr-1" />
                            {t("moderation.approve")}
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            onClick={() => handleRejectPost(post.id)}
                          >
                            <XCircle className="w-4 h-4 mr-1" />
                            {t("moderation.reject")}
                          </Button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="approved">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <CheckCircle className="text-success size-4" />
                {t("moderation.approvedPosts")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {approvedPosts.length === 0 ? (
                <EmptyState
                  icon={CheckCircle}
                  title={t("moderation.noApprovedPosts")}
                />
              ) : (
                <div className="space-y-4">
                  {approvedPosts.map((post) => (
                    <div
                      key={post.id}
                      className="flex items-start justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <Badge variant="success">
                            {t("moderation.approved")}
                          </Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt || "")}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">
                          {post.title}
                        </h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          {t("moderation.author")}: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(String(post.slug))}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          {t("moderation.view")}
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="rejected">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <XCircle className="text-destructive size-4" />
                {t("moderation.rejectedPosts")}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {rejectedPosts.length === 0 ? (
                <EmptyState
                  icon={XCircle}
                  title={t("moderation.noRejectedPosts")}
                />
              ) : (
                <div className="space-y-4">
                  {rejectedPosts.map((post) => (
                    <div
                      key={post.id}
                      className="flex items-start justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2 mb-2">
                          <Badge variant="destructive">
                            {t("moderation.rejected")}
                          </Badge>
                          <span className="text-sm text-muted-foreground">
                            {formatDate(post.createdAt || "")}
                          </span>
                        </div>
                        <h3 className="font-medium text-lg mb-1">
                          {post.title}
                        </h3>
                        <p className="text-sm text-muted-foreground mb-2">
                          {t("moderation.author")}: {post.author?.username}
                        </p>
                        {post.excerpt && (
                          <p className="text-sm text-muted-foreground line-clamp-2">
                            {post.excerpt}
                          </p>
                        )}
                      </div>
                      <div className="flex flex-col gap-2 ml-4">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleEditPost(post.id)}
                        >
                          <Edit className="w-4 h-4 mr-1" />
                          {t("moderation.edit")}
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => handleViewPost(String(post.slug))}
                        >
                          <Eye className="w-4 h-4 mr-1" />
                          {t("moderation.view")}
                        </Button>{" "}
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleResubmitPost(post.id)}
                        >
                          <Send className="w-4 h-4 mr-1" />
                          {t("moderation.resubmit")}
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
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
