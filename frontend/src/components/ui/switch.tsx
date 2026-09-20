import * as React from "react";
import { Switch as BaseSwitch } from "@base-ui/react/switch";

import { cn } from "@/lib/utils";

/**
 * A squared switch. The pill track is the single most recognisable piece of
 * template UI there is; a rectangle with a square thumb says the same thing —
 * off or on — and belongs to the same geometry as the rest of the console.
 */
function Switch({
  className,
  ...props
}: React.ComponentProps<typeof BaseSwitch.Root>) {
  return (
    <BaseSwitch.Root
      data-slot="switch"
      className={cn(
        "inline-flex h-5 w-9 shrink-0 items-center rounded-sm border border-input bg-input p-0.5 transition-colors",
        "data-checked:border-primary data-checked:bg-primary",
        "data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className,
      )}
      {...props}
    >
      <BaseSwitch.Thumb
        data-slot="switch-thumb"
        className={cn(
          "pointer-events-none block size-3.5 rounded-xs bg-card shadow-none transition-transform",
          "data-checked:translate-x-4 data-unchecked:translate-x-0",
          "dark:data-unchecked:bg-foreground dark:data-checked:bg-primary-foreground",
        )}
      />
    </BaseSwitch.Root>
  );
}

export { Switch };
