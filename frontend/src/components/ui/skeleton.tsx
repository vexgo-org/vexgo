import { cn } from "@/lib/utils";

/**
 * A slow diagonal sweep rather than the pulsing opacity block every template
 * ships: it reads as "being typeset" instead of as a broken element, and the
 * motion stays under the threshold that makes a loading screen feel busy.
 */
function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      aria-hidden="true"
      className={cn("skeleton rounded-sm", className)}
      {...props}
    />
  );
}

/**
 * Loading shape for a settings panel. Label/field pairs rather than one tall
 * grey slab: a slab tells the reader nothing about what is coming and the page
 * jumps when the real form replaces it.
 */
function SkeletonForm({
  rows = 4,
  className,
}: {
  rows?: number;
  className?: string;
}) {
  return (
    <div className={cn("space-y-5", className)}>
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="space-y-2">
          <Skeleton className="h-3.5 w-28" />
          <Skeleton className="h-9 w-full" />
        </div>
      ))}
    </div>
  );
}

/**
 * Loading shape for a repeating row list, echoing the hairlines the real rows
 * will be separated by.
 */
function SkeletonRows({
  rows = 3,
  className,
}: {
  rows?: number;
  className?: string;
}) {
  return (
    <div
      className={cn("divide-y divide-border border-t border-border", className)}
    >
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="space-y-3 py-5">
          <Skeleton className="h-5 w-2/5" />
          <Skeleton className="h-3 w-1/4" />
        </div>
      ))}
    </div>
  );
}

export { Skeleton, SkeletonForm, SkeletonRows };
