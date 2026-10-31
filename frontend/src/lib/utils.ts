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

export function isUserRole(value: string): value is UserUpdateUserRoleBodyRole {
  return Object.values(UserUpdateUserRoleBodyRole).includes(
    value as UserUpdateUserRoleBodyRole,
  );
}
