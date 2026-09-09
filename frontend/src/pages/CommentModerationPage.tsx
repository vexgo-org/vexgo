import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CheckCircle, XCircle, Clock, User } from "lucide-react";
import { toast } from "sonner";
import { sdk } from "@/lib/sdk";

import type { Comment } from "@/types";

export function CommentModerationPage() {
  const { t } = useTranslation();
  const [pendingComments, setPendingComments] = useState<Comment[]>([]);
  const [approvedComments, setApprovedComments] = useState<Comment[]>([]);
  const [rejectedComments, setRejectedComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState("pending");

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      if (activeTab === "pending") {
        const result = await sdk.comments.moderationPending();
        setPendingComments((result.comments || []) as Comment[]);
      } else if (activeTab === "approved") {
        const result = await sdk.comments.moderationApproved();
        setApprovedComments((result.comments || []) as Comment[]);
      } else if (activeTab === "rejected") {
        const result = await sdk.comments.moderationRejected();
        setRejectedComments((result.comments || []) as Comment[]);
      }
    } catch (error) {
      console.error("Failed to load comments:", error);
      toast.error(t("common.error"));
    } finally {
      setLoading(false);
    }
  }, [activeTab, t]);

  useEffect(() => {
    loadData();
  }, [activeTab, loadData]);

  const handleApproveComment = async (commentId: string) => {
    try {
      await sdk.comments.moderationApprove(commentId);
      toast.success(t("moderation.approveSuccess"));
      loadData();
    } catch (error) {
      console.error("Failed to approve:", error);
      toast.error(t("moderation.approveFailed"));
    }
  };

  const handleRejectComment = async (commentId: string) => {
    try {
      await sdk.comments.moderationReject(commentId);
      toast.success(t("moderation.rejectSuccess"));
      loadData();
    } catch (error) {
      console.error("Failed to reject comment:", error);
      toast.error(t("moderation.rejectFailed"));
    }
  };

  const getCurrentComments = () => {
    switch (activeTab) {
      case "pending":
        return pendingComments;
      case "approved":
        return approvedComments;
      case "rejected":
        return rejectedComments;
      default:
        return [];
    }
  };

  const comments = getCurrentComments();

  return (
    <div className="container mx-auto py-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{t("commentModeration.title")}</h1>
          <p className="text-muted-foreground">
            {t("adminData.manageComments")}
          </p>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="mb-4">
          <TabsTrigger value="pending" className="flex items-center gap-2">
            <Clock className="h-4 w-4" />
            {t("moderation.pending")}
            <Badge variant="secondary" className="ml-1">
              {pendingComments.length}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="approved" className="flex items-center gap-2">
            <CheckCircle className="h-4 w-4" />
            {t("moderation.approved")}
          </TabsTrigger>
          <TabsTrigger value="rejected" className="flex items-center gap-2">
            <XCircle className="h-4 w-4" />
            {t("moderation.rejected")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending">
          <Card>
            <CardHeader>
              <CardTitle>{t("commentModeration.pending")}</CardTitle>
              <CardDescription>
                {t("commentModeration.reviewNeeded")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t("common.loading")}
                </div>
              ) : comments.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t("commentModeration.noPendingComments")}
                </div>
              ) : (
                <div className="space-y-4">
                  {comments.map((comment) => (
                    <div key={comment.id} className="border rounded-lg p-4">
                      <div className="flex items-start justify-between">
                        <div className="flex items-center gap-2 mb-2">
                          <User className="h-4 w-4" />
                          <span className="font-medium">
                            {comment.author?.username ||
                              t("commentModeration.anonymous")}
                          </span>
                          <span className="text-muted-foreground text-sm">
                            {new Date(comment.createdAt || "").toLocaleString(
                              getLocale(),
                            )}
                          </span>
                        </div>
                        <div className="flex gap-2">
                          <Button
                            size="sm"
                            variant="default"
                            onClick={() =>
                              handleApproveComment(String(comment.id))
                            }
                          >
                            <CheckCircle className="h-4 w-4 mr-1" />
                            {t("moderation.approve")}
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            onClick={() =>
                              handleRejectComment(String(comment.id))
                            }
                          >
                            <XCircle className="h-4 w-4 mr-1" />
                            {t("moderation.reject")}
                          </Button>
                        </div>
                      </div>
                      <div className="bg-muted/50 rounded p-3 mt-2">
                        <p className="text-sm">{comment.content}</p>
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
              <CardTitle>{t("moderation.approved")}</CardTitle>
              <CardDescription>
                {t("commentModeration.approvedDesc")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t("common.loading")}
                </div>
              ) : comments.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t("commentModeration.noApprovedComments")}
                </div>
              ) : (
                <div className="space-y-4">
                  {comments.map((comment) => (
                    <div key={comment.id} className="border rounded-lg p-4">
                      <div className="flex items-center gap-2 mb-2">
                        <User className="h-4 w-4" />
                        <span className="font-medium">
                          {comment.author?.username ||
                            t("commentModeration.anonymous")}
                        </span>
                        <span className="text-muted-foreground text-sm">
                          {new Date(comment.createdAt || "").toLocaleString(
                            getLocale(),
                          )}
                        </span>
                        <Badge variant="default" className="ml-auto">
                          {t("moderation.approved")}
                        </Badge>
                      </div>
                      <div className="bg-muted/50 rounded p-3 mt-2">
                        <p className="text-sm">{comment.content}</p>
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
              <CardTitle>{t("moderation.rejected")}</CardTitle>
              <CardDescription>
                {t("commentModeration.rejectedDesc")}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t("common.loading")}
                </div>
              ) : comments.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  {t("commentModeration.noRejectedComments")}
                </div>
              ) : (
                <div className="space-y-4">
                  {comments.map((comment) => (
                    <div key={comment.id} className="border rounded-lg p-4">
                      <div className="flex items-center gap-2 mb-2">
                        <User className="h-4 w-4" />
                        <span className="font-medium">
                          {comment.author?.username ||
                            t("commentModeration.anonymous")}
                        </span>
                        <span className="text-muted-foreground text-sm">
                          {new Date(comment.createdAt || "").toLocaleString(
                            getLocale(),
                          )}
                        </span>
                        <Badge variant="destructive" className="ml-auto">
                          {t("moderation.rejected")}
                        </Badge>
                      </div>
                      <div className="bg-muted/50 rounded p-3 mt-2">
                        <p className="text-sm">{comment.content}</p>
                      </div>
                      {comment.moderationReason && (
                        <p className="text-sm text-destructive mt-2">
                          {t("commentModeration.reasonLabel")}
                          {comment.moderationReason}
                        </p>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
