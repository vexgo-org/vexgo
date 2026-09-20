import * as React from "react";
import { Menu as BaseMenu } from "@base-ui/react/menu";
import { CheckIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Every popup in the console is a flat paper sheet: a hairline border, squared
 * corners, and no drop shadow. Depth comes from the backdrop dimming the page
 * behind it, not from a blur under the panel.
 */
function DropdownMenu(props: BaseMenu.Root.Props) {
  return <BaseMenu.Root data-slot="dropdown-menu" {...props} />;
}

function DropdownMenuTrigger({
  className,
  ...props
}: React.ComponentProps<typeof BaseMenu.Trigger>) {
  return (
    <BaseMenu.Trigger
      data-slot="dropdown-menu-trigger"
      className={cn("select-none", className)}
      {...props}
    />
  );
}

function DropdownMenuContent({
  className,
  sideOffset = 4,
  align = "end",
  children,
  ...props
}: React.ComponentProps<typeof BaseMenu.Popup> & {
  sideOffset?: number;
  align?: "start" | "center" | "end";
}) {
  return (
    <BaseMenu.Portal>
      <BaseMenu.Positioner
        sideOffset={sideOffset}
        align={align}
        className="z-50 outline-none"
      >
        <BaseMenu.Popup
          data-slot="dropdown-menu-content"
          className={cn(
            "ui-popup min-w-[11rem] overflow-hidden rounded-md border border-border bg-popover p-1 text-popover-foreground",
            className,
          )}
          {...props}
        >
          <BaseMenu.Viewport>{children}</BaseMenu.Viewport>
        </BaseMenu.Popup>
      </BaseMenu.Positioner>
    </BaseMenu.Portal>
  );
}

function DropdownMenuItem({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<typeof BaseMenu.Item> & {
  variant?: "default" | "destructive";
}) {
  return (
    <BaseMenu.Item
      data-slot="dropdown-menu-item"
      data-variant={variant}
      className={cn(
        "relative flex cursor-default items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm outline-none select-none",
        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
        "data-disabled:pointer-events-none data-disabled:opacity-50",
        "data-[variant=destructive]:text-destructive data-[variant=destructive]:data-highlighted:bg-destructive/10",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        "[&_svg]:text-muted-foreground data-[variant=destructive]:[&_svg]:text-destructive",
        className,
      )}
      {...props}
    />
  );
}

/**
 * Base UI only renders a group label inside a real group, so this stays a plain
 * element: it is used as a standalone heading for a run of items.
 */
function DropdownMenuLabel({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dropdown-menu-label"
      className={cn("eyebrow px-2 py-1.5", className)}
      {...props}
    />
  );
}

function DropdownMenuSeparator({
  className,
  ...props
}: React.ComponentProps<typeof BaseMenu.Separator>) {
  return (
    <BaseMenu.Separator
      data-slot="dropdown-menu-separator"
      className={cn("my-1 h-px bg-border", className)}
      {...props}
    />
  );
}

function DropdownMenuGroup({
  className,
  ...props
}: React.ComponentProps<typeof BaseMenu.Group>) {
  return (
    <BaseMenu.Group
      data-slot="dropdown-menu-group"
      className={cn("py-0.5", className)}
      {...props}
    />
  );
}

function DropdownMenuGroupLabel({
  className,
  ...props
}: React.ComponentProps<typeof BaseMenu.GroupLabel>) {
  return (
    <BaseMenu.GroupLabel
      data-slot="dropdown-menu-group-label"
      className={cn("eyebrow px-2 py-1.5", className)}
      {...props}
    />
  );
}

function DropdownMenuRadioGroup({
  ...props
}: React.ComponentProps<typeof BaseMenu.RadioGroup>) {
  return (
    <BaseMenu.RadioGroup data-slot="dropdown-menu-radio-group" {...props} />
  );
}

function DropdownMenuRadioItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof BaseMenu.RadioItem>) {
  return (
    <BaseMenu.RadioItem
      data-slot="dropdown-menu-radio-item"
      className={cn(
        "relative flex cursor-default items-center gap-2.5 rounded-sm py-1.5 pr-2 pl-6 text-sm outline-none select-none",
        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
        "data-disabled:pointer-events-none data-disabled:opacity-50",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    >
      <BaseMenu.RadioItemIndicator
        data-slot="dropdown-menu-radio-indicator"
        className="absolute left-1.5 flex size-3.5 items-center justify-center text-accent-ink"
      >
        <CheckIcon className="size-3.5" />
      </BaseMenu.RadioItemIndicator>
      {children}
    </BaseMenu.RadioItem>
  );
}

function DropdownMenuCheckboxItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof BaseMenu.CheckboxItem>) {
  return (
    <BaseMenu.CheckboxItem
      data-slot="dropdown-menu-checkbox-item"
      className={cn(
        "relative flex cursor-default items-center gap-2.5 rounded-sm py-1.5 pr-2 pl-6 text-sm outline-none select-none",
        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
        "data-disabled:pointer-events-none data-disabled:opacity-50",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    >
      <BaseMenu.CheckboxItemIndicator
        data-slot="dropdown-menu-checkbox-indicator"
        className="absolute left-1.5 flex size-3.5 items-center justify-center text-accent-ink"
      >
        <CheckIcon className="size-3.5" />
      </BaseMenu.CheckboxItemIndicator>
      {children}
    </BaseMenu.CheckboxItem>
  );
}

function DropdownMenuLinkItem({
  className,
  ...props
}: React.ComponentProps<typeof BaseMenu.LinkItem>) {
  return (
    <BaseMenu.LinkItem
      data-slot="dropdown-menu-link-item"
      className={cn(
        "relative flex cursor-default items-center gap-2.5 rounded-sm px-2 py-1.5 text-sm no-underline outline-none select-none",
        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 [&_svg]:text-muted-foreground",
        className,
      )}
      {...props}
    />
  );
}

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuGroupLabel,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuCheckboxItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuLinkItem,
};
