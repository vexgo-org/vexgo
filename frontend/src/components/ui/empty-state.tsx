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
 * the same and always offers the next step. The title is set in the display
 * face with an ink rule above it, which reads as the end of a section rather
 * than as a broken screen.
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
        "flex flex-col items-center justify-center gap-2 px-6 py-16 text-center",
        className,
      )}
    >
      {Icon && (
        <Icon
          className="mb-1 size-4 text-muted-foreground/70"
          aria-hidden="true"
        />
      )}
      <p className="display text-subtitle text-foreground">{title}</p>
      {description && (
        <p className="max-w-sm text-sm leading-relaxed text-muted-foreground">
          {description}
        </p>
      )}
      {action && <div className="mt-3">{action}</div>}
    </div>
  );
}
