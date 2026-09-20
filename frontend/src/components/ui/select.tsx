import * as React from "react";
import { Select as BaseSelect } from "@base-ui/react/select";
import { CheckIcon, ChevronDownIcon } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Base UI's `<Select.Value>` renders the raw value unless the root receives an
 * `items` map. This console's selects are almost always value ≠ label
 * ("openai" → "OpenAI", "draft" → the localized label), so the root instead
 * collects the options its own `<SelectContent>` declares. That keeps every
 * call site reading like the Radix one it replaces while the trigger still
 * shows the human label.
 */
interface SelectOption {
  value: string;
  label: React.ReactNode;
}

function collectOptions(
  node: React.ReactNode,
  out: SelectOption[] = [],
): SelectOption[] {
  React.Children.forEach(node, (child) => {
    if (!React.isValidElement(child)) return;
    const props = child.props as {
      value?: string;
      children?: React.ReactNode;
    };

    if (child.type === SelectItem) {
      if (props.value !== undefined) {
        out.push({ value: props.value, label: props.children });
      }
      return;
    }
    if (props.children !== undefined) {
      collectOptions(props.children, out);
    }
  });
  return out;
}

type SelectProps = Omit<
  BaseSelect.Root.Props<string, false>,
  "items" | "onValueChange"
> & {
  items?: SelectOption[];
  /**
   * Base UI reports `null` when a selection is cleared and passes an event
   * details argument; every select in this console is always populated, so the
   * callback keeps the plain `(value: string)` shape its call sites expect.
   */
  onValueChange?: (value: string) => void;
};

function Select({
  children,
  items,
  value,
  onValueChange,
  ...props
}: SelectProps) {
  const options = React.useMemo(
    () => items ?? collectOptions(children),
    [items, children],
  );

  const handleValueChange = React.useCallback(
    (next: string | null) => {
      if (next !== null) onValueChange?.(next);
    },
    [onValueChange],
  );

  return (
    <BaseSelect.Root
      items={options}
      {...(value !== undefined ? { value } : {})}
      {...(onValueChange ? { onValueChange: handleValueChange } : {})}
      {...props}
    >
      {children}
    </BaseSelect.Root>
  );
}

function SelectValue({
  className,
  ...props
}: React.ComponentProps<typeof BaseSelect.Value>) {
  return (
    <BaseSelect.Value
      data-slot="select-value"
      className={cn("truncate text-left", className)}
      {...props}
    />
  );
}

function SelectTrigger({
  className,
  children,
  size = "default",
  ...props
}: React.ComponentProps<typeof BaseSelect.Trigger> & {
  size?: "sm" | "default";
}) {
  return (
    <BaseSelect.Trigger
      data-slot="select-trigger"
      data-size={size}
      className={cn(
        "flex w-fit min-w-0 items-center justify-between gap-2 rounded-md border border-input bg-card px-2.5 py-1 text-sm text-foreground",
        "transition-colors",
        "data-[size=default]:h-9 data-[size=sm]:h-8",
        "disabled:cursor-not-allowed disabled:opacity-50",
        "aria-invalid:border-destructive",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    >
      {children}
      <BaseSelect.Icon>
        <ChevronDownIcon className="size-4 text-muted-foreground" />
      </BaseSelect.Icon>
    </BaseSelect.Trigger>
  );
}

function SelectContent({
  className,
  children,
  sideOffset = 4,
  alignItemWithTrigger = false,
  ...props
}: React.ComponentProps<typeof BaseSelect.Popup> & {
  sideOffset?: number;
  alignItemWithTrigger?: boolean;
}) {
  return (
    <BaseSelect.Portal>
      <BaseSelect.Positioner
        sideOffset={sideOffset}
        align="start"
        alignItemWithTrigger={alignItemWithTrigger}
        className="z-50 outline-none"
      >
        <BaseSelect.Popup
          data-slot="select-content"
          className={cn(
            "ui-popup max-h-[min(20rem,var(--available-height))] min-w-[9rem] overflow-hidden",
            "rounded-md border border-border bg-popover text-popover-foreground",
            className,
          )}
          {...props}
        >
          <BaseSelect.ScrollUpArrow className="flex h-6 items-center justify-center text-muted-foreground" />
          <BaseSelect.List className="max-h-[inherit] overflow-y-auto p-1">
            {children}
          </BaseSelect.List>
          <BaseSelect.ScrollDownArrow className="flex h-6 items-center justify-center text-muted-foreground" />
        </BaseSelect.Popup>
      </BaseSelect.Positioner>
    </BaseSelect.Portal>
  );
}

function SelectItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof BaseSelect.Item>) {
  return (
    <BaseSelect.Item
      data-slot="select-item"
      className={cn(
        "relative flex cursor-default items-center gap-2 rounded-sm py-1.5 pr-2 pl-6 text-sm outline-none select-none",
        "data-highlighted:bg-accent data-highlighted:text-accent-foreground",
        "data-disabled:pointer-events-none data-disabled:opacity-50",
        "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className,
      )}
      {...props}
    >
      <BaseSelect.ItemIndicator
        data-slot="select-item-indicator"
        className="absolute left-1.5 flex size-3.5 items-center justify-center text-accent-ink"
      >
        <CheckIcon className="size-3.5" />
      </BaseSelect.ItemIndicator>
      <BaseSelect.ItemText>{children}</BaseSelect.ItemText>
    </BaseSelect.Item>
  );
}

function SelectGroup({
  className,
  ...props
}: React.ComponentProps<typeof BaseSelect.Group>) {
  return (
    <BaseSelect.Group
      data-slot="select-group"
      className={cn("py-0.5", className)}
      {...props}
    />
  );
}

function SelectLabel({
  className,
  ...props
}: React.ComponentProps<typeof BaseSelect.GroupLabel>) {
  return (
    <BaseSelect.GroupLabel
      data-slot="select-label"
      className={cn("eyebrow px-2 py-1", className)}
      {...props}
    />
  );
}

function SelectSeparator({
  className,
  ...props
}: React.ComponentProps<typeof BaseSelect.Separator>) {
  return (
    <BaseSelect.Separator
      data-slot="select-separator"
      className={cn("my-1 h-px bg-border", className)}
      {...props}
    />
  );
}

export {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
};
