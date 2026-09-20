import * as React from "react";
import { useRender } from "@base-ui/react/use-render";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

/**
 * Buttons are ink on paper. The solid variant is near-black rather than a
 * brand hue because the action that matters in a CMS is "publish", which
 * should read as decisive; every other variant is a hairline or a bare
 * surface. No shadows — a border and a hover fill carry the affordance, which
 * keeps a dense toolbar from looking busy.
 */
const buttonVariants = cva(
  [
    "inline-flex shrink-0 select-none items-center justify-center gap-1.5 whitespace-nowrap",
    "rounded-md font-medium transition-colors",
    "disabled:pointer-events-none disabled:opacity-45",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
    "aria-invalid:border-destructive",
  ].join(" "),
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/85",
        destructive:
          "bg-destructive text-destructive-foreground hover:bg-destructive/88",
        outline:
          "border border-input bg-card text-foreground hover:bg-accent hover:text-accent-foreground",
        secondary: "bg-secondary text-secondary-foreground hover:bg-muted",
        ghost: "text-foreground hover:bg-accent hover:text-accent-foreground",
        link: "text-accent-ink underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-3.5 text-sm has-[>svg]:px-3",
        sm: "h-8 px-2.5 text-sm has-[>svg]:px-2",
        lg: "h-10 px-5 text-base has-[>svg]:px-4",
        icon: "size-9",
        "icon-sm": "size-7",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

type ButtonProps = React.ComponentPropsWithRef<"button"> &
  VariantProps<typeof buttonVariants> & {
    /**
     * Replaces the rendered element, Base UI style — `<Button render={<Link
     * to="/posts" />}>` gives a button-shaped link without a Slot indirection.
     */
    render?: useRender.RenderProp;
  };

function Button({
  className,
  variant = "default",
  size = "default",
  render,
  ...props
}: ButtonProps) {
  return useRender({
    render,
    defaultTagName: "button",
    props: {
      "data-slot": "button",
      "data-variant": variant,
      "data-size": size,
      className: cn(buttonVariants({ variant, size }), className),
      ...props,
    },
  });
}

export { Button, buttonVariants };
