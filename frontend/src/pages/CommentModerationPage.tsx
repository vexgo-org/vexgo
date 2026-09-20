import { useCallback, useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge, type badgeVariants } from "@/components/ui/badge";
import type { VariantProps } from "class-variance-authority";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/PageHeader";
import { CheckCircle, XCircle, Clock } from "lucide-react";
import { toast } from "sonner";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import type { Comment } from "@/types";

type BadgeVariant = VariantProps<typeof badgeVariants>["variant"];

/**
 * One comment awaiting a decision. The body is set as quoted material on a
 * rule rather than in a tinted box, which is what it is: someone else's words,
 * reproduced under the moderator's eye.
 */
function CommentRow({
  comment,
  variant,
  status,
  actions,
}: {
  comment: Comment;
  variant: BadgeVariant;
  status: string;
  actions?: ReactNode;
}) {
  const { t } = useTranslation();

  return (
    <li className="flex flex-col gap-4 py-5 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
          <span className="text-sm font-medium">
            {comment.author?.username || t("commentModeration.anonymous")}
          </span>
          <span className="text-2xs text-muted-foreground tabular-nums">
            {new Date(comment.createdAt || "").toLocaleString(getLocale())}
          </span>
          <Badge variant={variant}>{status}</Badge>
        </div>
        <blockquote className="border-border mt-3 border-l-2 pl-3">
          <p className="text-sm leading-relaxed whitespace-pre-wrap">
            {comment.content}
          </p>
        </blockquote>
        {comment.moderationReason && (
          <p className="border-destructive mt-2 border-l-2 pl-3 text-xs leading-relaxed text-muted-foreground">
            <span className="text-destructive font-medium">
              {t("commentModeration.reasonLabel")}
            </span>{" "}
            {comment.moderationReason}
          </p>
        )}
      </div>
      {actions && (
        <div className="flex flex-wrap items-center gap-2 sm:shrink-0">
          {actions}
        </div>
      )}
    </li>
  );
}

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
        const response = await unwrap(
          getVexGoAPI().getModerationCommentsPending(),
        );
        setPendingComments((response.comments || []) as Comment[]);
      } else if (activeTab === "approved") {
        const response = await unwrap(
          getVexGoAPI().getModerationCommentsApproved(),
        );
        setApprovedComments((response.comments || []) as Comment[]);
      } else if (activeTab === "rejected") {
        const response = await unwrap(
          getVexGoAPI().getModerationCommentsRejected(),
        );
        setRejectedComments((response.comments || []) as Comment[]);
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
      await unwrap(getVexGoAPI().putModerationCommentsIdApprove(commentId));
      toast.success(t("moderation.approveSuccess"));
      loadData();
    } catch (error) {
      console.error("Failed to approve:", error);
      toast.error(t("moderation.approveFailed"));
    }
  };

  const handleRejectComment = async (commentId: string) => {
    try {
      await unwrap(getVexGoAPI().putModerationCommentsIdReject(commentId));
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

  /** Standfirst and empty state for the visible tab. Each tab used to carry
   * its own CardTitle, repeating the label that was already on the tab. */
  const panel =
    activeTab === "pending"
      ? {
          description: t("commentModeration.reviewNeeded"),
          empty: t("commentModeration.noPendingComments"),
          emptyIcon: Clock,
          variant: "outline-warning" as BadgeVariant,
          status: t("moderation.pending"),
        }
      : activeTab === "approved"
        ? {
            description: t("commentModeration.approvedDesc"),
            empty: t("commentModeration.noApprovedComments"),
            emptyIcon: CheckCircle,
            variant: "outline-success" as BadgeVariant,
            status: t("moderation.approved"),
          }
        : {
            description: t("commentModeration.rejectedDesc"),
            empty: t("commentModeration.noRejectedComments"),
            emptyIcon: XCircle,
            variant: "outline-destructive" as BadgeVariant,
            status: t("moderation.rejected"),
          };

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("commentModeration.title")}
        description={t("adminData.manageComments")}
      />

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="w-full">
          <TabsTrigger value="pending" className="gap-2">
            <Clock />
            {t("moderation.pending")}
            <span className="text-2xs text-muted-foreground tabular-nums">
              {pendingComments.length}
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

        <TabsContent value={activeTab} className="space-y-4">
          <p className="text-xs text-muted-foreground">{panel.description}</p>
          {loading ? (
            <div className="border-t border-border">
              {[1, 2, 3].map((i) => (
                <div key={i} className="space-y-3 border-b border-border py-5">
                  <Skeleton className="h-3.5 w-40" />
                  <Skeleton className="h-4 w-3/5" />
                </div>
              ))}
            </div>
          ) : comments.length === 0 ? (
            <EmptyState icon={panel.emptyIcon} title={panel.empty} />
          ) : (
            <ul className="divide-y divide-border">
              {comments.map((comment) => (
                <CommentRow
                  key={comment.id}
                  comment={comment}
                  variant={panel.variant}
                  status={panel.status}
                  actions={
                    activeTab === "pending" ? (
                      <>
                        <Button
                          size="sm"
                          onClick={() =>
                            handleApproveComment(String(comment.id))
                          }
                        >
                          <CheckCircle />
                          {t("moderation.approve")}
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() =>
                            handleRejectComment(String(comment.id))
                          }
                        >
                          <XCircle />
                          {t("moderation.reject")}
                        </Button>
                      </>
                    ) : undefined
                  }
                />
              ))}
            </ul>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
