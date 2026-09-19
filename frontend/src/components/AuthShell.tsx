import { Suspense } from "react";
import { Link, Outlet } from "react-router-dom";
import { ArrowLeft } from "lucide-react";

import { useSiteBrand } from "@/hooks/useSiteSettings";
import { useTranslation } from "@/lib/I18nContext";
import { RouteFallback } from "@/components/RouteFallback";

/**
 * Unauthenticated screens get a stripped frame: the brand, a way back to the
 * live site, and the form. No navigation (there is nothing to navigate yet)
 * and no footer — a copyright line under a login box is filler.
 */
export function AuthShell() {
  const { t } = useTranslation();
  const { siteName, renderBrand } = useSiteBrand();

  return (
    <div className="bg-background flex min-h-screen flex-col">
      <header className="flex h-14 shrink-0 items-center justify-between px-4 lg:px-6">
        <Link
          to="/admin"
          className="focus-visible:ring-ring/25 flex items-center gap-2 rounded-md outline-none focus-visible:ring-[3px]"
        >
          {renderBrand("size-5")}
          <span className="text-[13px] font-semibold">{siteName}</span>
        </Link>
        <Link
          to="/"
          className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-[13px] transition-colors"
        >
          <ArrowLeft className="size-3.5" aria-hidden="true" />
          {t("layout.viewSite")}
        </Link>
      </header>

      <main className="flex flex-1 items-start justify-center px-4 pt-6 pb-20 sm:items-center sm:pt-0">
        <div className="w-full max-w-md">
          <Suspense fallback={<RouteFallback />}>
            <Outlet />
          </Suspense>
        </div>
      </main>
    </div>
  );
}
