import { useTranslation } from "@/lib/I18nContext";
import { cn } from "@/lib/utils";

/**
 * Published / draft / pending / rejected. The state is carried by a short
 * word plus a flat dot — the dot is a second, non-textual channel so the
 * status survives a glance and colour-blind reading. No icon: a glyph inside
 * a badge this small is ink, not information.
 */
const TONES: Record<string, string> = {
  published: "border-success/40 bg-success/10 text-success",
  approved: "border-success/40 bg-success/10 text-success",
  draft: "border-border bg-muted text-muted-foreground",
  pending: "border-warning/40 bg-warning/10 text-warning",
  rejected: "border-destructive/40 bg-destructive/10 text-destructive",
};

const LABEL_KEYS: Record<string, string> = {
  published: "posts.published",
  approved: "moderation.approved",
  draft: "posts.draft",
  pending: "posts.pending",
  rejected: "posts.rejected",
};

interface StatusBadgeProps {
  status?: string;
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const { t } = useTranslation();
  const key = (status ?? "").toLowerCase();
  const labelKey = LABEL_KEYS[key];

  return (
    <span
      data-slot="status-badge"
      className={cn(
        "inline-flex w-fit shrink-0 items-center gap-1.5 rounded-sm border px-1.5 py-0.5 text-[11px] leading-4 font-medium whitespace-nowrap",
        TONES[key] ?? "border-border bg-muted text-muted-foreground",
        className,
      )}
    >
      <span
        className="size-1.5 shrink-0 rounded-full bg-current"
        aria-hidden="true"
      />
      {labelKey ? t(labelKey) : status}
    </span>
  );
}
