import { cn } from "@/lib/utils";

/**
 * A hairline arc instead of the usual chunky glyph: at 16px the thin stroke
 * reads as a machine working rather than as a spinner pasted on top of the UI.
 *
 * Inside a button the arc is decoration — the button's own label already says
 * what is happening, so it stays out of the accessibility tree. Pass `label`
 * only where the spinner is the sole signal that work is in progress.
 */
function Spinner({
  className,
  label,
  ...props
}: React.ComponentProps<"svg"> & { label?: string }) {
  return (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      aria-hidden={label ? undefined : true}
      role={label ? "status" : undefined}
      aria-label={label}
      className={cn("size-4 animate-spin", className)}
      {...props}
    >
      <circle
        cx="8"
        cy="8"
        r="6.25"
        stroke="currentColor"
        strokeOpacity="0.22"
        strokeWidth="1.5"
      />
      <path
        d="M14.25 8A6.25 6.25 0 0 0 8 1.75"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

export { Spinner };
