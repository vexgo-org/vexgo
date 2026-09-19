import { cn } from "@/lib/utils";

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn(
        "bg-muted-foreground/15 animate-pulse rounded-sm",
        className,
      )}
      {...props}
    />
  );
}

export { Skeleton };
