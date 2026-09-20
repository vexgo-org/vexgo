import {
  CircleCheckIcon,
  InfoIcon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { Toaster as Sonner, type ToasterProps } from "sonner";

import { Spinner } from "@/components/ui/spinner";
import { useIsDark } from "@/hooks/useIsDark";

/**
 * Toasts are styled through Sonner's `classNames` rather than its CSS custom
 * properties: the previous `--normal-bg: var(--popover)` handed Sonner an HSL
 * triplet where a colour was expected, so every toast fell back to its own
 * palette. Classes also let the toast inherit the token layer like any other
 * surface in the console.
 */
const Toaster = ({ ...props }: ToasterProps) => {
  // Read the resolved theme off <html> rather than from a provider: the
  // console owns its own light/dark switch, so toasts would otherwise follow
  // the OS while the rest of the app follows the stored preference.
  const isDark = useIsDark();

  return (
    <Sonner
      theme={isDark ? "dark" : "light"}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Spinner className="size-4" />,
      }}
      toastOptions={{
        classNames: {
          toast:
            "!rounded-md !border !border-border !bg-popover !text-popover-foreground !text-sm !shadow-none",
          title: "!font-medium",
          description: "!text-muted-foreground",
          actionButton:
            "!rounded-sm !bg-primary !text-primary-foreground !text-xs !font-medium",
          cancelButton:
            "!rounded-sm !bg-secondary !text-secondary-foreground !text-xs !font-medium",
          icon: "!text-muted-foreground",
        },
      }}
      {...props}
    />
  );
};

export { Toaster };
