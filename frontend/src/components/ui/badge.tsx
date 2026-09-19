import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

/**
 * Badges are square-ish labels, not pills — a pill reads as decoration, a
 * rectangle reads as data. `success`/`warning`/`info` exist so state is
 * expressed through tokens instead of hardcoded Tailwind shades.
 */
const badgeVariants = cva(
  "inline-flex items-center justify-center gap-1 rounded-sm border px-1.5 py-0.5 text-[11px] font-medium leading-4 w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 [&>svg]:pointer-events-none transition-colors overflow-hidden focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/25",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-primary text-primary-foreground [a&]:hover:bg-primary/85",
        secondary:
          "border-transparent bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/70",
        destructive:
          "border-transparent bg-destructive text-destructive-foreground [a&]:hover:bg-destructive/90",
        success:
          "border-transparent bg-success text-success-foreground [a&]:hover:bg-success/90",
        warning:
          "border-transparent bg-warning text-warning-foreground [a&]:hover:bg-warning/90",
        info: "border-transparent bg-info text-info-foreground [a&]:hover:bg-info/90",
        outline:
          "text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
        /* Tinted outline: state at a glance without a solid block of color. */
        "outline-success":
          "border-success/40 text-success bg-success/10 [a&]:hover:bg-success/20",
        "outline-warning":
          "border-warning/40 text-warning bg-warning/10 [a&]:hover:bg-warning/20",
        "outline-info":
          "border-info/40 text-info bg-info/10 [a&]:hover:bg-info/20",
        "outline-destructive":
          "border-destructive/40 text-destructive bg-destructive/10 [a&]:hover:bg-destructive/20",
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
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "span";

  return (
    <Comp
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  );
}

export { Badge, badgeVariants };
