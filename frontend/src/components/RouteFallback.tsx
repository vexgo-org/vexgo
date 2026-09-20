import { useTranslation } from "@/lib/I18nContext";
import { Spinner } from "@/components/ui/spinner";

/**
 * Route-level loading state. A hairline arc in the middle of the page, nothing
 * else — a full-height spinner or a grid of skeleton cards would make a route
 * switch feel slower than it is. This is the one place the arc carries the
 * whole message, so it announces itself rather than staying decorative.
 */
export function RouteFallback() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <Spinner
        label={t("common.loading")}
        className="size-5 text-muted-foreground/70"
      />
    </div>
  );
}
