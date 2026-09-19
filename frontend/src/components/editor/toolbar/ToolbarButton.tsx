import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface ToolbarButtonProps {
  label: string;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
}

export function ToolbarButton({
  label,
  active = false,
  disabled = false,
  onClick,
  children,
}: ToolbarButtonProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      title={label}
      aria-label={label}
      aria-pressed={active}
      disabled={disabled}
      // Keep the caret and selection in the editor when a button is pressed.
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      className={cn(
        "text-muted-foreground hover:text-foreground",
        active && "bg-accent text-accent-foreground hover:bg-accent",
      )}
    >
      {children}
    </Button>
  );
}

export function ToolbarDivider() {
  return <span className="mx-1 h-6 w-px bg-border" />;
}
