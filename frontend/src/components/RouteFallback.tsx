import { Spinner } from "@/components/ui/spinner";

/**
 * Route-level loading state. Kept small and quiet — a full-height spinner
 * makes a route switch feel slower than it is.
 */
export function RouteFallback() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center">
      <Spinner className="size-5 text-muted-foreground" />
    </div>
  );
}
