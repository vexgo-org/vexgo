import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";
import { CheckCircle, XCircle } from "lucide-react";

interface CreatorApplication {
  id?: number;
  username?: string;
  email?: string;
  currentRole?: string;
  createdAt?: string;
  reason?: string;
  status?: string;
}

export function CreatorApplicationReviewPage() {
  const { user: currentUser, isLoading: isAuthLoading } = useAuth();
  const { t } = useTranslation();
  const [applications, setApplications] = useState<CreatorApplication[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedApplication, setSelectedApplication] =
    useState<CreatorApplication | null>(null);
  const [isApproveDialogOpen, setIsApproveDialogOpen] = useState(false);
  const [isRejectDialogOpen, setIsRejectDialogOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState("");
  const [isProcessing, setIsProcessing] = useState(false);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const response = await unwrap(
        getVexGoAPI().getUsersCreatorApplications({
          page: 1,
          limit: 100,
          status: "pending",
        }),
      );
      // Ensure applications is an array even if the backend returns null or undefined
      setApplications(response.applications || []);
    } catch (error) {
      console.error("Failed to load creator applications:", error);
      toast.error(t("errors.networkError"));
      // Also fall back to an empty array on error to avoid a blank screen
      setApplications([]);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Show loading state while auth is loading
  if (isAuthLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 rounded-lg" />
      </div>
    );
  }

  // Only allow admin and super_admin to access this page
  if (
    !currentUser ||
    (currentUser.role !== "admin" && currentUser.role !== "super_admin")
  ) {
    return (
      <div className="mx-auto max-w-lg">
        <Card className="py-10 text-center">
          <p className="text-sm font-semibold">{t("common.accessDenied")}</p>
          <p className="text-muted-foreground mt-1 text-[13px]">
            {t("common.insufficientPermissions")}
          </p>
        </Card>
      </div>
    );
  }

  const handleReview = async (action: "approve" | "reject") => {
    if (!selectedApplication) {
      toast.error("Invalid application data");
      return;
    }

    setIsProcessing(true);
    try {
      const reason = action === "reject" ? rejectReason : undefined;
      const response = await unwrap(
        getVexGoAPI().putUsersCreatorApplicationsIdReview(
          selectedApplication.id!,
          { action, reason },
        ),
      );
      toast.success(response.message);

      // Remove the processed application from the list
      setApplications((prev) =>
        prev.filter((app) => app.id !== selectedApplication.id),
      );

      // Reset states
      setSelectedApplication(null);
      setIsApproveDialogOpen(false);
      setIsRejectDialogOpen(false);
      setRejectReason("");
    } catch (error: unknown) {
      console.error("Failed to review creator application:", error);
      const apiError = error as { response?: { data?: { error?: string } } };
      toast.error(apiError.response?.data?.error || t("errors.networkError"));
    } finally {
      setIsProcessing(false);
    }
  };

  const openApproveDialog = (application: CreatorApplication) => {
    setSelectedApplication(application);
    setIsApproveDialogOpen(true);
  };

  const openRejectDialog = (application: CreatorApplication) => {
    setSelectedApplication(application);
    setIsRejectDialogOpen(true);
  };

  const formatDate = (dateString: string) => {
    if (!dateString) return "-";
    return new Date(dateString).toLocaleString();
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "pending":
        return (
          <Badge variant="outline-warning">
            {t("creatorApplication.status.pending")}
          </Badge>
        );
      case "approved":
        return (
          <Badge variant="outline-success">
            {t("creatorApplication.status.approved")}
          </Badge>
        );
      case "rejected":
        return (
          <Badge variant="outline-destructive">
            {t("creatorApplication.status.rejected")}
          </Badge>
        );
      default:
        return <Badge variant="outline">{status}</Badge>;
    }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-24 rounded-lg" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={t("creatorApplication.reviewTitle")}
        description={t("creatorApplication.pendingCount", {
          count: applications.length,
        })}
      />

      <Card className="py-0">
        <div className="border-border border-b px-5 py-3">
          <p className="text-[13px] font-semibold">
            {t("creatorApplication.pendingApplications")}
          </p>
        </div>
        <div className="px-5 py-4">
          {applications.length === 0 ? (
            <EmptyState
              icon={CheckCircle}
              title={t("creatorApplication.noPendingApplications")}
              description={t("creatorApplication.noPendingApplicationsDesc")}
            />
          ) : (
            <div className="space-y-4">
              {applications.map((application) => (
                <div
                  key={application.id}
                  className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <div className="bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-full text-[13px] font-medium">
                        {(application.username || "").charAt(0).toUpperCase()}
                      </div>
                      <div>
                        <h3 className="font-medium">
                          {application.username || "-"}
                        </h3>
                        <p className="text-sm text-muted-foreground">
                          {application.email || "-"}
                        </p>
                      </div>
                    </div>
                    <div className="space-y-2">
                      <div className="flex items-center gap-4 text-sm text-muted-foreground">
                        <span>
                          {t("creatorApplication.currentRole")}:{" "}
                          {t(`roles.${application.currentRole || "guest"}`)}
                        </span>
                        <span>
                          {t("creatorApplication.appliedAt")}:{" "}
                          {formatDate(
                            application.createdAt || new Date().toISOString(),
                          )}
                        </span>
                      </div>
                      {application.reason && (
                        <div className="text-sm">
                          <span className="font-medium">
                            {t("creatorApplication.reasonLabel")}:
                          </span>
                          <p className="text-muted-foreground mt-1">
                            {application.reason}
                          </p>
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    {getStatusBadge(application.status || "")}
                    <div className="flex items-center gap-2">
                      <Button
                        variant="default"
                        size="sm"
                        onClick={() => openApproveDialog(application)}
                      >
                        <CheckCircle className="w-4 h-4 mr-1" />
                        {t("creatorApplication.approve")}
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => openRejectDialog(application)}
                      >
                        <XCircle className="w-4 h-4 mr-1" />
                        {t("creatorApplication.reject")}
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </Card>

      {/* Approve Confirmation Dialog */}
      <AlertDialog
        open={isApproveDialogOpen}
        onOpenChange={setIsApproveDialogOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("creatorApplication.approveConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("creatorApplication.approveConfirmDescription", {
                username: selectedApplication?.username || "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleReview("approve")}
              disabled={isProcessing}
            >
              {isProcessing
                ? t("common.processing")
                : t("creatorApplication.confirmApprove")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Reject Confirmation Dialog */}
      <AlertDialog
        open={isRejectDialogOpen}
        onOpenChange={setIsRejectDialogOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("creatorApplication.rejectConfirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("creatorApplication.rejectConfirmDescription", {
                username: selectedApplication?.username || "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-2">
            <label htmlFor="rejectReason" className="text-sm font-medium">
              {t("creatorApplication.rejectReasonLabel")} (
              {t("common.optional")})
            </label>
            <Textarea
              id="rejectReason"
              placeholder={t("creatorApplication.rejectReasonPlaceholder")}
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              rows={3}
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleReview("reject")}
              disabled={isProcessing}
            >
              {isProcessing
                ? t("common.processing")
                : t("creatorApplication.confirmReject")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
