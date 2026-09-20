import { Link } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { buttonVariants } from "@/components/ui/button";

/**
 * A 404 is set, not illustrated. The numeral is the display face at the largest
 * size in the console, closed by the same ink rule every page header uses, so
 * a wrong URL still looks like it belongs to the publication.
 */
export function NotFoundPage() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen flex-col items-center justify-center px-4 py-16">
      <div className="w-full max-w-sm">
        <div className="rule-ink pb-4">
          <p className="display text-[3.5rem] leading-none tracking-[-0.03em]">
            404
          </p>
          <p className="mt-3 text-sm text-muted-foreground">
            {t("errors.notFound")}
          </p>
        </div>
        <Link
          to="/admin"
          className={`${buttonVariants({ variant: "outline", size: "sm" })} mt-5`}
        >
          {t("postDetailPage.backToHome")}
        </Link>
      </div>
    </div>
  );
}
