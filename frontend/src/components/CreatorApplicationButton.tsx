import { useState } from "react";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { applyForCreator } from "@/lib/userApi";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
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
import { UserPlus, Send } from "lucide-react";

interface CreatorApplicationButtonProps {
  className?: string;
}

export function CreatorApplicationButton({
  className,
}: CreatorApplicationButtonProps) {
  const { user } = useAuth();
  const { t } = useTranslation();
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  // Only show for guest and contributor users
  // Hide for author, admin, and super_admin users
  if (
    user?.role === "author" ||
    user?.role === "admin" ||
    user?.role === "super_admin"
  ) {
    return null;
  }

  const handleApply = async () => {
    setIsLoading(true);
    try {
      const response = await applyForCreator(reason || undefined);
      toast.success(response.data.message);
      setIsDialogOpen(false);
      setIsConfirmOpen(false);
      setReason("");
    } catch (error: unknown) {
      console.error("Failed to apply for role upgrade:", error);
      const apiError = error as { response?: { data?: { error?: string } } };
      toast.error(apiError.response?.data?.error || t("errors.networkError"));
    } finally {
      setIsLoading(false);
    }
  };

  const openConfirmation = () => {
    if (reason.trim()) {
      setIsConfirmOpen(true);
    } else {
      handleApply();
    }
  };

  return (
    <>
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogTrigger asChild>
          <Button className={className} variant="outline" size="sm">
            <UserPlus className="w-4 h-4 mr-2" />
            {user?.role === "guest"
              ? t("creatorApplication.applyButton")
              : t("creatorApplication.applyButton") +
                " (" +
                t("roles.contributor") +
                ")"}
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("creatorApplication.title")}</DialogTitle>
            <DialogDescription>
              {t("creatorApplication.description")}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="text-sm text-muted-foreground">
              {t("creatorApplication.currentRole")}:{" "}
              <span className="font-medium">
                {t(`roles.${user?.role || "guest"}`)}
              </span>
            </div>

            <div className="space-y-2">
              <label htmlFor="reason" className="text-sm font-medium">
                {t("creatorApplication.reasonLabel")} ({t("common.optional")})
              </label>
              <Textarea
                id="reason"
                placeholder={t("creatorApplication.reasonPlaceholder")}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={4}
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setIsDialogOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button onClick={openConfirmation} disabled={isLoading}>
              <Send className="w-4 h-4 mr-2" />
              {isLoading
                ? t("creatorApplication.submitting")
                : t("creatorApplication.submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={isConfirmOpen} onOpenChange={setIsConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("creatorApplication.confirmTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("creatorApplication.confirmDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleApply} disabled={isLoading}>
              {isLoading
                ? t("creatorApplication.submitting")
                : t("creatorApplication.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
