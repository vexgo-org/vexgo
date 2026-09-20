import * as React from "react";
import { Tabs as BaseTabs } from "@base-ui/react/tabs";

import { cn } from "@/lib/utils";

/**
 * Tabs sit on a single hairline that runs the width of the strip, and the
 * active tab breaks it with an ink rule. That is the printed index pattern:
 * one continuous baseline, one marked entry — rather than a segmented control,
 * which would claim the tabs are a set of buttons.
 */
function Tabs({
  className,
  ...props
}: React.ComponentProps<typeof BaseTabs.Root>) {
  return (
    <BaseTabs.Root
      data-slot="tabs"
      className={cn("flex flex-col gap-5", className)}
      {...props}
    />
  );
}

function TabsList({
  className,
  ...props
}: React.ComponentProps<typeof BaseTabs.List>) {
  return (
    <BaseTabs.List
      data-slot="tabs-list"
      className={cn(
        "flex items-center gap-5 overflow-x-auto border-b border-border",
        className,
      )}
      {...props}
    />
  );
}

function TabsTrigger({
  className,
  ...props
}: React.ComponentProps<typeof BaseTabs.Tab>) {
  return (
    <BaseTabs.Tab
      data-slot="tabs-trigger"
      className={cn(
        "-mb-px inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap",
        "border-b-2 border-transparent px-0.5 pt-2 pb-2.5 text-sm font-medium text-muted-foreground",
        "transition-colors hover:text-foreground",
        "disabled:pointer-events-none disabled:opacity-50",
        "data-active:border-rule data-active:text-foreground",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    />
  );
}

function TabsContent({
  className,
  ...props
}: React.ComponentProps<typeof BaseTabs.Panel>) {
  return (
    <BaseTabs.Panel
      data-slot="tabs-content"
      className={cn("flex-1 outline-none", className)}
      {...props}
    />
  );
}

export { Tabs, TabsList, TabsTrigger, TabsContent };
