import type * as React from "react";

import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  /** Optional section label, set as an uppercase eyebrow above the title. */
  eyebrow?: string;
  description?: string;
  actions?: React.ReactNode;
  className?: string;
}

/**
 * Every console screen opens the same way: a section eyebrow, the title in the
 * display face, an optional standfirst, and a full-width ink rule closing the
 * block. That rule is what makes a page read as a page — it separates the
 * masthead from the work the way a rule does in print, without a card or a
 * coloured band to do it.
 */
export function PageHeader({
  title,
  eyebrow,
  description,
  actions,
  className,
}: PageHeaderProps) {
  return (
    <div
      className={cn(
        "rule-ink flex flex-wrap items-end justify-between gap-x-6 gap-y-3 pb-3",
        className,
      )}
    >
      <div className="min-w-0 space-y-1.5">
        {eyebrow && <p className="eyebrow">{eyebrow}</p>}
        <h1 className="display truncate text-title">{title}</h1>
        {description && (
          <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
            {description}
          </p>
        )}
      </div>
      {actions && (
        <div className="flex shrink-0 items-center gap-2">{actions}</div>
      )}
    </div>
  );
}
