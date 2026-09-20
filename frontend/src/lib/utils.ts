import { UserUpdateUserRoleBodyRole } from "@/api/generated/model";
import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/**
 * The console defines its own steps of the type scale in `index.css`
 * (`text-subtitle`, `text-title`, `text-figure`, `text-micro`, `text-2xs`).
 * tailwind-merge only knows Tailwind's built-in sizes, so without this it
 * classifies an unfamiliar `text-subtitle` as a *colour* utility and drops one
 * of the two when a component asks for a size and a colour at once — which
 * silently left every card title at the inherited 14px body size.
 */
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      "font-size": [
        { text: ["micro", "2xs", "subtitle", "title", "display", "figure"] },
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// Normalize various tag shapes into a string array
export function normalizeTagsArray(raw: unknown): string[] {
  if (!raw) return [];
  if (Array.isArray(raw)) {
    return raw
      .map((t) => {
        if (!t && t !== 0) return "";
        if (typeof t === "string") return t;
        if (typeof t === "number") return String(t);
        if (typeof t === "object") {
          const tag = t as Record<string, unknown>;
          const name = tag.name ?? tag.Name ?? tag.title ?? tag.label;
          if (name) return String(name);
          return tag.id ? String(tag.id) : "";
        }
        return String(t);
      })
      .map((s) => (s ? s.trim() : ""))
      .filter(Boolean);
  }
  if (typeof raw === "string") {
    return raw
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  }
  return [];
}

export function isUserRole(value: string): value is UserUpdateUserRoleBodyRole {
  return Object.values(UserUpdateUserRoleBodyRole).includes(
    value as UserUpdateUserRoleBodyRole,
  );
}
