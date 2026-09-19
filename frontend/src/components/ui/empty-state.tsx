import type * as React from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@/lib/utils";

interface EmptyStateProps {
  /** Small, muted glyph. A large pastel tile is decoration, not information. */
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

/**
 * One empty state for the whole console, so "nothing here yet" always looks
 * the same and always offers the next step.
 */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-1.5 px-6 py-14 text-center",
        className,
      )}
    >
      {Icon && (
        <Icon
          className="text-muted-foreground/60 mb-1 size-5"
          aria-hidden="true"
        />
      )}
      <p className="text-sm font-medium">{title}</p>
      {description && (
        <p className="text-muted-foreground max-w-sm text-[13px] leading-relaxed">
          {description}
        </p>
      )}
      {action && <div className="mt-3">{action}</div>}
    </div>
  );
}
