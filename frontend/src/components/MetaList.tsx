import type * as React from "react";

import { cn } from "@/lib/utils";

/**
 * A label/value stack set like a printed data table: the label is a micro-caps
 * eyebrow in a fixed column, so a grid of records can be scanned vertically
 * without every value having to announce what it is. This replaces the
 * `Label: value` sentences with bold labels that the pages used to hand-roll,
 * which read as prose rather than as data.
 */
function MetaList({ className, ...props }: React.ComponentProps<"dl">) {
  return <dl className={cn("space-y-1.5", className)} {...props} />;
}

function MetaRow({
  label,
  children,
  className,
  labelClassName,
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
  labelClassName?: string;
}) {
  return (
    <div className={cn("flex gap-3", className)}>
      <dt className={cn("eyebrow w-16 shrink-0 pt-[3px]", labelClassName)}>
        {label}
      </dt>
      <dd className="min-w-0 flex-1 text-xs text-foreground">{children}</dd>
    </div>
  );
}

export { MetaList, MetaRow };
