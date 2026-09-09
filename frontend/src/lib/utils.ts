import { UserUpdateUserRoleBodyRole } from "@vexgo/sdk";
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

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
