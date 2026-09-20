import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

/**
 * A badge is a micro-label, not a pill: squared corners, uppercase, and
 * letterspaced so it reads as an annotation printed beside the value rather
 * than as decoration. `success`/`warning`/`info` map to the print-ink tokens
 * so state is never expressed through a hardcoded Tailwind shade.
 */
const badgeVariants = cva(
  [
    "inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden",
    "rounded-sm border px-1.5 py-[3px]",
    "text-micro font-medium uppercase whitespace-nowrap",
    "transition-colors",
    "[&>svg]:pointer-events-none [&>svg]:size-3",
  ].join(" "),
  {
    variants: {
      variant: {
        default:
          "border border-transparent bg-primary text-primary-foreground [a&]:hover:bg-primary/85",
        secondary:
          "border border-transparent bg-secondary text-secondary-foreground [a&]:hover:bg-muted",
        destructive:
          "border border-transparent bg-destructive text-destructive-foreground [a&]:hover:bg-destructive/88",
        success:
          "border border-transparent bg-success text-success-foreground [a&]:hover:bg-success/88",
        warning:
          "border border-transparent bg-warning text-warning-foreground [a&]:hover:bg-warning/88",
        info: "border border-transparent bg-info text-info-foreground [a&]:hover:bg-info/88",
        outline:
          "border border-input text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
        /* Tinted outline: state at a glance without a solid block of color. */
        "outline-success":
          "border-success/40 bg-success/10 text-success [a&]:hover:bg-success/18",
        "outline-warning":
          "border-warning/40 bg-warning/10 text-warning [a&]:hover:bg-warning/18",
        "outline-info":
          "border-info/40 bg-info/10 text-info [a&]:hover:bg-info/18",
        "outline-destructive":
          "border-destructive/40 bg-destructive/10 text-destructive [a&]:hover:bg-destructive/18",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

function Badge({
  className,
  variant,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants>) {
  return (
    <span
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
