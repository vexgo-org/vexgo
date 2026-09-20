import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * Fields are quiet by default: a hairline box on paper that only raises its
 * voice when it is focused or invalid. The ink focus outline comes from the
 * global `:focus-visible` rule, so every control in the console marks focus
 * the same way.
 */
function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "h-9 w-full min-w-0 rounded-md border border-input bg-card px-2.5 py-1 text-sm text-foreground",
        "transition-colors",
        "placeholder:text-muted-foreground/70",
        "file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground",
        "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        "aria-invalid:border-destructive",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
