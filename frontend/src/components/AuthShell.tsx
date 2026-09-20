import { Suspense } from "react";
import { Link, Outlet } from "react-router-dom";
import { ArrowLeft } from "lucide-react";

import { useSiteBrand } from "@/hooks/useSiteSettings";
import { useTranslation } from "@/lib/I18nContext";
import { RouteFallback } from "@/components/RouteFallback";

/**
 * Unauthenticated screens get a stripped frame: a masthead column carrying the
 * publication's name, and the form beside it. No navigation (there is nothing
 * to navigate yet), no card around the form, and no footer — a copyright line
 * under a login box is filler.
 *
 * The two-column split exists so the sign-in screen still says *which* site is
 * being signed into. It collapses to a single column below `lg`, where the
 * masthead becomes a slim bar instead of eating the fold.
 */
export function AuthShell() {
  const { t } = useTranslation();
  const { siteName, renderBrand } = useSiteBrand();

  return (
    <div className="flex min-h-screen bg-background">
      {/* Masthead column */}
      <aside className="hidden w-[38%] max-w-md shrink-0 flex-col justify-between border-r border-border bg-sidebar px-8 py-8 lg:flex">
        <Link to="/admin" className="flex items-center gap-2.5">
          {renderBrand("size-6")}
          <span className="display text-base">{siteName}</span>
        </Link>

        <div className="space-y-4">
          <div className="rule-ink pb-4">
            <p className="display text-display break-words">{siteName}</p>
          </div>
          <p className="max-w-xs text-sm leading-relaxed text-muted-foreground">
            {t("layout.authTagline")}
          </p>
        </div>

        <Link
          to="/"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="size-3.5" aria-hidden="true" />
          {t("layout.viewSite")}
        </Link>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Slim masthead — mobile only */}
        <header className="flex h-14 shrink-0 items-center justify-between border-b border-border px-4 lg:hidden">
          <Link to="/admin" className="flex items-center gap-2">
            {renderBrand("size-5")}
            <span className="display text-sm">{siteName}</span>
          </Link>
          <Link
            to="/"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground"
          >
            <ArrowLeft className="size-3.5" aria-hidden="true" />
            {t("layout.viewSite")}
          </Link>
        </header>

        <main className="flex flex-1 items-center justify-center px-4 py-12">
          <div className="w-full max-w-sm">
            <Suspense fallback={<RouteFallback />}>
              <Outlet />
            </Suspense>
          </div>
        </main>
      </div>
    </div>
  );
}
