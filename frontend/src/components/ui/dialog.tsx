import * as React from "react";
import { Dialog as BaseDialog } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";

import { cn } from "@/lib/utils";

function Dialog(props: BaseDialog.Root.Props) {
  return <BaseDialog.Root data-slot="dialog" {...props} />;
}

function DialogTrigger({
  className,
  ...props
}: React.ComponentProps<typeof BaseDialog.Trigger>) {
  return (
    <BaseDialog.Trigger
      data-slot="dialog-trigger"
      className={cn("select-none", className)}
      {...props}
    />
  );
}

function DialogPortal(props: BaseDialog.Portal.Props) {
  return <BaseDialog.Portal data-slot="dialog-portal" {...props} />;
}

function DialogClose({
  className,
  ...props
}: React.ComponentProps<typeof BaseDialog.Close>) {
  return (
    <BaseDialog.Close
      data-slot="dialog-close"
      className={cn("select-none", className)}
      {...props}
    />
  );
}

function DialogBackdrop({
  className,
  ...props
}: React.ComponentProps<typeof BaseDialog.Backdrop>) {
  return (
    <BaseDialog.Backdrop
      data-slot="dialog-backdrop"
      className={cn(
        "ui-backdrop fixed inset-0 z-50 bg-black/40 dark:bg-black/60",
        className,
      )}
      {...props}
    />
  );
}

/**
 * A dialog is a sheet of paper laid on top of the page: squared, hairline
 * bordered, no shadow. The backdrop does the work of separating the two
 * planes, and it dims rather than blurs — a blurred page behind a modal is a
 * template tell and it costs a compositing pass on every open.
 */
function DialogContent({
  className,
  children,
  showCloseButton = true,
  variant = "center",
  ...props
}: React.ComponentProps<typeof BaseDialog.Popup> & {
  showCloseButton?: boolean;
  /** `drawer` anchors the sheet to the left edge instead of centring it. */
  variant?: "center" | "drawer";
}) {
  return (
    <DialogPortal>
      <DialogBackdrop />
      <BaseDialog.Popup
        data-slot="dialog-content"
        data-variant={variant}
        className={cn(
          "fixed z-50 grid bg-card text-card-foreground outline-none",
          variant === "center"
            ? "ui-popup-dialog top-1/2 left-1/2 w-[calc(100%-2rem)] max-w-lg gap-4 rounded-md border border-border p-5 max-h-[calc(100dvh-2rem)] overflow-y-auto"
            : "ui-popup-drawer inset-y-0 left-0 w-64 max-w-[80vw] gap-0 border-r border-border p-0",
          className,
        )}
        {...props}
      >
        {children}
        {showCloseButton && (
          <BaseDialog.Close
            data-slot="dialog-close"
            className="absolute top-3 right-3 grid size-7 place-content-center rounded-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none"
          >
            <XIcon className="size-4" />
            <span className="sr-only">Close</span>
          </BaseDialog.Close>
        )}
      </BaseDialog.Popup>
    </DialogPortal>
  );
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex flex-col gap-2 pr-8", className)}
      {...props}
    />
  );
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        className,
      )}
      {...props}
    />
  );
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof BaseDialog.Title>) {
  return (
    <BaseDialog.Title
      data-slot="dialog-title"
      className={cn("display text-lg text-foreground", className)}
      {...props}
    />
  );
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof BaseDialog.Description>) {
  return (
    <BaseDialog.Description
      data-slot="dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogBackdrop,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
};
