import { Link } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";

export function NotFoundPage() {
  const { t } = useTranslation();

  return (
    <div className="container mx-auto px-4 py-16 max-w-4xl text-center">
      <h1 className="text-6xl font-bold text-muted-foreground/30 mb-4">404</h1>
      <p className="text-muted-foreground mb-8">{t("errors.notFound")}</p>
      <Button asChild>
        <Link to="/">{t("postDetailPage.backToHome")}</Link>
      </Button>
    </div>
  );
}
