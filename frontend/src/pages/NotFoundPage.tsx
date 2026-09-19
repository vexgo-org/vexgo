import { Link } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";

export function NotFoundPage() {
  const { t } = useTranslation();

  return (
    <div className="bg-background flex min-h-screen flex-col items-center justify-center gap-2 px-4 text-center">
      <p className="text-muted-foreground font-mono text-[13px]">404</p>
      <h1 className="text-lg font-semibold tracking-[-0.01em]">
        {t("errors.notFound")}
      </h1>
      <Button variant="outline" asChild className="mt-3">
        <Link to="/admin">{t("postDetailPage.backToHome")}</Link>
      </Button>
    </div>
  );
}
