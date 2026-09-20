import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

/**
 * Inline feedback. Each tone is a faintly tinted paper with a matching hairline
 * and icon — never a solid block of saturated colour, and never a coloured bar
 * down the left edge (that pattern means nothing to a reader).
 */
const alertVariants = cva(
  [
    "relative grid w-full items-start gap-x-2.5 gap-y-0.5 rounded-md border px-3.5 py-2.5 text-sm",
    "has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] grid-cols-[0_1fr]",
    "[&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:text-current",
  ].join(" "),
  {
    variants: {
      variant: {
        default: "border-border bg-card text-card-foreground",
        muted: "border-border bg-muted text-foreground",
        info: "border-info/35 bg-info/8 text-info",
        success: "border-success/35 bg-success/8 text-success",
        warning: "border-warning/35 bg-warning/8 text-warning",
        destructive: "border-destructive/35 bg-destructive/8 text-destructive",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

function Alert({
  className,
  variant,
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof alertVariants>) {
  return (
    <div
      data-slot="alert"
      role="alert"
      className={cn(alertVariants({ variant }), className)}
      {...props}
    />
  );
}

function AlertTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-title"
      className={cn(
        "col-start-2 min-h-4 font-medium tracking-[-0.005em]",
        className,
      )}
      {...props}
    />
  );
}

function AlertDescription({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-description"
      className={cn(
        "col-start-2 grid justify-items-start gap-1 text-current/80 [&_p]:leading-relaxed",
        className,
      )}
      {...props}
    />
  );
}

export { Alert, AlertTitle, AlertDescription };
