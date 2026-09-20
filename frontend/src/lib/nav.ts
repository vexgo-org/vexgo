import {
  Bell,
  Cpu,
  FileText,
  Files,
  LayoutDashboard,
  Mail,
  MessageSquare,
  Palette,
  PenLine,
  Settings2,
  ShieldCheck,
  SlidersHorizontal,
  User,
  UserCheck,
  Users,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  to: string;
  labelKey: string;
  icon: LucideIcon;
  /** Match the path exactly — required for index routes like `/admin`. */
  end?: boolean;
  /** Visible to `admin` and `super_admin` only. */
  adminOnly?: boolean;
  /** Hidden from guests, who may read but not author. */
  authorOnly?: boolean;
  /** Renders the unread notification count beside the label. */
  unreadBadge?: boolean;
}

export interface NavGroup {
  /** Omitted for the primary group, which sits at the top of the rail. */
  titleKey?: string;
  adminOnly?: boolean;
  items: NavItem[];
}

/**
 * The console's navigation, grouped by what an editor is doing rather than by
 * which package owns the screen. Groups, not a flat list: an admin console
 * with twenty flat entries is a wall the eye cannot scan.
 */
export const NAV_GROUPS: NavGroup[] = [
  {
    items: [
      {
        to: "/admin",
        labelKey: "layout.navOverview",
        icon: LayoutDashboard,
        end: true,
        adminOnly: true,
      },
      {
        to: "/admin/write",
        labelKey: "layout.writePost",
        icon: PenLine,
        authorOnly: true,
      },
      {
        to: "/admin/my-posts",
        labelKey: "layout.myPosts",
        icon: FileText,
        authorOnly: true,
      },
      {
        to: "/admin/notifications",
        labelKey: "layout.notifications",
        icon: Bell,
        authorOnly: true,
        unreadBadge: true,
      },
    ],
  },
  {
    titleKey: "layout.navContent",
    adminOnly: true,
    items: [
      {
        to: "/admin/pages",
        labelKey: "layout.pages",
        icon: Files,
        end: true,
      },
    ],
  },
  {
    titleKey: "layout.navModeration",
    adminOnly: true,
    items: [
      {
        to: "/admin/moderation",
        labelKey: "layout.navPosts",
        icon: ShieldCheck,
      },
      {
        to: "/admin/comment-moderation",
        labelKey: "layout.navComments",
        icon: MessageSquare,
      },
      {
        to: "/admin/creator-applications",
        labelKey: "layout.navCreatorApplications",
        icon: UserCheck,
      },
    ],
  },
  {
    titleKey: "layout.navSystem",
    adminOnly: true,
    items: [
      { to: "/admin/users", labelKey: "layout.navUsers", icon: Users },
      { to: "/admin/theme", labelKey: "layout.navThemes", icon: Palette },
    ],
  },
  {
    titleKey: "layout.navSettings",
    adminOnly: true,
    items: [
      {
        to: "/admin/general-settings",
        labelKey: "layout.navGeneral",
        icon: Settings2,
      },
      {
        to: "/admin/comment-config",
        labelKey: "layout.navCommentSettings",
        icon: MessageSquare,
      },
      { to: "/admin/ai-settings", labelKey: "layout.navAI", icon: Cpu },
      { to: "/admin/smtp", labelKey: "layout.navEmail", icon: Mail },
    ],
  },
  {
    titleKey: "layout.navAccount",
    items: [
      { to: "/admin/profile", labelKey: "layout.profile", icon: User },
      {
        to: "/admin/settings",
        labelKey: "layout.navPreferences",
        icon: SlidersHorizontal,
      },
    ],
  },
];

export function isAdminRole(role?: string): boolean {
  return role === "admin" || role === "super_admin";
}

/**
 * Filters the rail down to the entries the visitor may actually open, so a
 * contributor never sees a door they will be bounced out of.
 */
export function visibleNavGroups(
  isAuthenticated: boolean,
  role?: string,
): NavGroup[] {
  const admin = isAdminRole(role);
  const canAuthor = isAuthenticated && role !== "guest";

  return NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      if (item.adminOnly && !admin) return false;
      if (item.authorOnly && !canAuthor) return false;
      return true;
    }),
  })).filter((group) => group.items.length > 0 && (!group.adminOnly || admin));
}
