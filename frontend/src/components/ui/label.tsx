import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * A field label is set like a masthead eyebrow: small, uppercase, and
 * letterspaced. Rendering a native `<label>` (rather than a composed
 * primitive) means `htmlFor` associates for free, which is what actually
 * makes a form operable from the keyboard.
 */
function Label({ className, ...props }: React.ComponentProps<"label">) {
  return (
    <label
      data-slot="label"
      className={cn(
        "eyebrow flex items-center gap-2 select-none",
        "group-data-[disabled=true]:pointer-events-none group-data-[disabled=true]:opacity-50",
        "peer-disabled:cursor-not-allowed peer-disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}

export { Label };
