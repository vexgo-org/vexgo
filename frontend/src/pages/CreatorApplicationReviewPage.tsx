import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getLocale } from "@/lib/i18n";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/PageHeader";
import { MetaList, MetaRow } from "@/components/MetaList";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
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
        <PageHeader title={t("creatorApplication.reviewTitle")} />
        <div className="border-t border-border">
          {[1, 2, 3].map((i) => (
            <div key={i} className="space-y-3 border-b border-border py-5">
              <Skeleton className="h-4 w-40" />
              <Skeleton className="h-3 w-24" />
            </div>
          ))}
        </div>
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
        <div className="rule-ink pb-3">
          <h1 className="display text-title">{t("common.accessDenied")}</h1>
        </div>
        <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
          {t("common.insufficientPermissions")}
        </p>
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
    return new Date(dateString).toLocaleString(getLocale());
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
        <PageHeader title={t("creatorApplication.reviewTitle")} />
        <div className="border-t border-border">
          {[1, 2, 3].map((i) => (
            <div key={i} className="space-y-3 border-b border-border py-5">
              <Skeleton className="h-5 w-48" />
              <Skeleton className="h-3 w-64" />
              <Skeleton className="h-3 w-2/5" />
            </div>
          ))}
        </div>
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

      {/* The queue is a list of decisions to make, so it is set as one: a rule
       * above it, a hairline between entries, and the applicant's own reason
       * quoted on a rule. Each entry used to be a bordered box holding a
       * circular monogram that repeated the first letter of the name beside
       * it. */}
      <section>
        <h2 className="eyebrow">
          {t("creatorApplication.pendingApplications")}
        </h2>
        {applications.length === 0 ? (
          <EmptyState
            icon={CheckCircle}
            title={t("creatorApplication.noPendingApplications")}
            description={t("creatorApplication.noPendingApplicationsDesc")}
          />
        ) : (
          <ul className="divide-y divide-border">
            {applications.map((application) => (
              <li
                key={application.id}
                className="flex flex-col gap-4 py-5 lg:flex-row lg:items-start lg:justify-between lg:gap-6"
              >
                <div className="min-w-0 flex-1 space-y-3">
                  <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
                    <h3 className="display text-subtitle">
                      {application.username || "-"}
                    </h3>
                    {getStatusBadge(application.status || "")}
                  </div>
                  <MetaList>
                    <MetaRow label={t("creatorApplication.emailLabel")}>
                      <span className="block truncate">
                        {application.email || "-"}
                      </span>
                    </MetaRow>
                    <MetaRow label={t("creatorApplication.currentRole")}>
                      {t(`roles.${application.currentRole || "guest"}`)}
                    </MetaRow>
                    <MetaRow label={t("creatorApplication.appliedAt")}>
                      <span className="tabular-nums">
                        {formatDate(
                          application.createdAt || new Date().toISOString(),
                        )}
                      </span>
                    </MetaRow>
                  </MetaList>
                  {application.reason && (
                    <div>
                      <p className="eyebrow mb-1.5">
                        {t("creatorApplication.reasonLabel")}
                      </p>
                      <p className="border-border border-l-2 pl-3 text-sm leading-relaxed text-muted-foreground">
                        {application.reason}
                      </p>
                    </div>
                  )}
                </div>

                <div className="flex flex-wrap items-center gap-2 lg:shrink-0">
                  <Button
                    variant="default"
                    size="sm"
                    onClick={() => openApproveDialog(application)}
                  >
                    <CheckCircle />
                    {t("creatorApplication.approve")}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => openRejectDialog(application)}
                  >
                    <XCircle />
                    {t("creatorApplication.reject")}
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

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
            <Label htmlFor="rejectReason">
              {t("creatorApplication.rejectReasonLabel")} (
              {t("common.optional")})
            </Label>
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
