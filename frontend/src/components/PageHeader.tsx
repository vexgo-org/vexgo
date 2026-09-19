import type * as React from "react";

import { cn } from "@/lib/utils";

interface PageHeaderProps {
  title: string;
  description?: string;
  actions?: React.ReactNode;
  className?: string;
}

/**
 * Every console screen opens with the same two lines: what this page is, and
 * what it is for. One component keeps that rhythm consistent — and keeps the
 * page title out of the top bar, where it would duplicate it.
 */
export function PageHeader({
  title,
  description,
  actions,
  className,
}: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-start justify-between gap-x-4 gap-y-3",
        className,
      )}
    >
      <div className="min-w-0 space-y-1">
        <h1 className="truncate text-lg leading-tight font-semibold tracking-[-0.01em]">
          {title}
        </h1>
        {description && (
          <p className="text-muted-foreground max-w-2xl text-[13px] leading-relaxed">
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
