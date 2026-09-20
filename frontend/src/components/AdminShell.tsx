import { Suspense, useState } from "react";
import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import {
  Bell,
  Check,
  ChevronsLeft,
  ChevronsRight,
  ExternalLink,
  Languages,
  LogOut,
  Menu,
  Moon,
  Search,
  Sun,
  User as UserIcon,
  UserCog,
} from "lucide-react";

import { useAuth } from "@/hooks/useAuth";
import { useIsDark } from "@/hooks/useIsDark";
import { useNotifications } from "@/hooks/useNotifications";
import { useSiteBrand } from "@/hooks/useSiteSettings";
import { useTheme } from "@/hooks/useTheme";
import { useTranslation } from "@/lib/I18nContext";
import { visibleNavGroups, type NavItem } from "@/lib/nav";
import { cn } from "@/lib/utils";

import { RouteFallback } from "@/components/RouteFallback";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { buttonVariants } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuLinkItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const COLLAPSE_KEY = "admin.sidebar.collapsed";

/** Utility-bar controls: 36px on touch, 28px once a cursor is available. */
const iconButton = cn(
  buttonVariants({ variant: "ghost", size: "icon-sm" }),
  "size-9 lg:size-7",
);

/**
 * The console frame: a contents column on the left, a utility bar on top.
 *
 * The rail is set like a magazine's table of contents rather than like an app
 * sidebar. Groups are separated by hairlines and titled with an uppercase
 * micro-label, entries are plain text rows, and the current page is marked by
 * an ink bar against the rail edge plus ink text — no rounded chips, no filled
 * pills, nothing that a scan has to decode.
 *
 * Spacing is one rhythm read top to bottom. Entries tile at exactly 32px
 * (44px in the drawer, where fingers replace the cursor) so the list counts in
 * whole modules instead of drifting by a gap every row. A section rule sits
 * 20px below the previous section and 12px above the label it introduces, so
 * the rule belongs to the section under it — the way a rule does in print —
 * rather than floating in the middle of an even gap. The label then sits 6px
 * above its own entries, which is what makes a section read as one block.
 */
function NavRow({
  item,
  collapsed,
  unreadCount,
  touch,
  onNavigate,
}: {
  item: NavItem;
  collapsed: boolean;
  unreadCount: number;
  /** Taller rows for the drawer, where fingers replace the cursor. */
  touch?: boolean;
  onNavigate?: () => void;
}) {
  const { t } = useTranslation();
  const label = t(item.labelKey);

  return (
    <NavLink
      to={item.to}
      end={item.end}
      onClick={onNavigate}
      title={collapsed ? label : undefined}
      className={({ isActive }) =>
        cn(
          "group relative flex items-center gap-2.5 px-3 text-sm transition-colors",
          // Full-bleed rows sit flush against the rail edge, so the focus ring
          // is pulled inside them; at the default +1px offset it would paint
          // over the rail's own border.
          "hover:bg-accent/70 focus-visible:outline-offset-[-2px]",
          touch ? "h-11" : "h-8",
          collapsed && "justify-center px-0",
          isActive
            ? "font-medium text-foreground"
            : "text-sidebar-foreground hover:text-foreground",
        )
      }
    >
      {({ isActive }) => (
        <>
          {isActive && (
            <span
              className="absolute inset-y-0 left-0 w-[2px] bg-rule"
              aria-hidden="true"
            />
          )}
          <item.icon
            className={cn(
              "size-4 shrink-0",
              // The icon has to answer the pointer too, or a hover leaves half
              // the row changed.
              isActive
                ? "text-foreground"
                : "text-muted-foreground group-hover:text-foreground",
            )}
            aria-hidden="true"
          />
          {!collapsed && <span className="truncate">{label}</span>}
          {!collapsed && item.unreadBadge && unreadCount > 0 && (
            // A count is fine data, not a label: 2xs keeps it out of the
            // letterspaced uppercase treatment `.eyebrow` would apply.
            <span className="ml-auto text-2xs tabular-nums text-accent-ink">
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </>
      )}
    </NavLink>
  );
}

function NavRail({
  collapsed,
  unreadCount,
  touch,
  onNavigate,
}: {
  collapsed: boolean;
  unreadCount: number;
  touch?: boolean;
  onNavigate?: () => void;
}) {
  const { t } = useTranslation();
  const { user, isAuthenticated } = useAuth();
  const groups = visibleNavGroups(isAuthenticated, user?.role);

  return (
    // Full-bleed rows so the active ink bar sits flush against the rail edge.
    // `overscroll-contain` keeps a flick at either end of the rail from
    // scrolling the page behind it.
    <nav className="flex flex-1 flex-col gap-5 overflow-y-auto overscroll-contain py-4">
      {groups.map((group, index) => (
        <div
          key={group.titleKey ?? `group-${index}`}
          className={cn("flex flex-col", index > 0 && "border-t pt-3")}
        >
          {/* Collapsed, the rail has no room for a label; the section rule
           * alone carries the grouping, so the labels are simply dropped
           * rather than replaced by a second, interior rule. */}
          {group.titleKey && !collapsed && (
            <p className="eyebrow px-3 pb-1.5">{t(group.titleKey)}</p>
          )}
          {group.items.map((item) => (
            <NavRow
              key={item.to}
              item={item}
              collapsed={collapsed}
              unreadCount={unreadCount}
              touch={touch}
              onNavigate={onNavigate}
            />
          ))}
        </div>
      ))}
    </nav>
  );
}

function Brand({ collapsed, size }: { collapsed: boolean; size: string }) {
  const { siteName, renderBrand } = useSiteBrand();

  return (
    <Link
      to="/admin"
      className={cn(
        "flex min-h-7 min-w-0 items-center gap-2 px-3",
        collapsed && "justify-center px-0",
      )}
      title={siteName}
    >
      {renderBrand(size)}
      {!collapsed && (
        <span className="display truncate text-subtitle">{siteName}</span>
      )}
    </Link>
  );
}

/**
 * Search is an underline field rather than a boxed input. The utility bar has
 * exactly one thing worth typing into, and an underline says "text goes here"
 * without adding a second rectangle next to the brand and the controls.
 */
function SearchForm({
  className,
  onSubmitted,
}: {
  className?: string;
  onSubmitted?: () => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");

  const handleSubmit = (event: React.SubmitEvent) => {
    event.preventDefault();
    const trimmed = query.trim();
    if (!trimmed) return;
    // Search is served by the public theme, outside this SPA.
    window.location.href = `/?search=${encodeURIComponent(trimmed)}`;
    onSubmitted?.();
  };

  return (
    <form onSubmit={handleSubmit} className={className} role="search">
      <div className="flex items-center gap-2 border-b border-border pb-1 transition-colors focus-within:border-rule">
        <Search
          className="size-3.5 shrink-0 text-muted-foreground"
          aria-hidden="true"
        />
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("layout.searchPlaceholder")}
          aria-label={t("layout.searchPlaceholder")}
          /* `min-h-6` keeps the underline field at the 24px minimum target
           * size without turning it back into a box. */
          className="min-h-6 min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-foreground outline-none placeholder:text-muted-foreground/70 focus-visible:outline-none"
        />
      </div>
    </form>
  );
}

/**
 * Language switch, kept in the utility bar next to the theme toggle: both are
 * "how the console reads" preferences, and neither should need a trip into a
 * settings page mid-task. Two locales only, so the menu is a flat list.
 */
function LanguageMenu() {
  const { locale, setLocale, t } = useTranslation();

  // Language names stay in their own language, the way pickers are expected to
  // behave — an English-only reader must still recognise "简体中文".
  const options = [
    { value: "zh-CN", label: "简体中文" },
    { value: "en-US", label: "English" },
  ];

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={iconButton}
        aria-label={t("layout.language")}
        title={t("layout.language")}
      >
        <Languages className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-44">
        <DropdownMenuLabel>{t("layout.language")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {options.map((option) => (
          <DropdownMenuItem
            key={option.value}
            onClick={() => setLocale(option.value)}
          >
            <span className="flex-1">{option.label}</span>
            {locale === option.value && (
              <Check className="size-4 text-accent-ink" />
            )}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function UserMenu() {
  const { t } = useTranslation();
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate("/admin/login");
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={cn(iconButton, "rounded-full")}
        aria-label={user?.username}
      >
        <Avatar className="size-6">
          {user?.avatar ? (
            <img src={user.avatar} alt="" className="size-full object-cover" />
          ) : (
            <AvatarFallback>
              {user?.username?.charAt(0).toUpperCase()}
            </AvatarFallback>
          )}
        </Avatar>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-60">
        <div className="px-2 py-1.5">
          <p className="truncate text-sm font-medium">{user?.username}</p>
          <p className="truncate text-xs text-muted-foreground">
            {user?.email}
          </p>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => navigate("/admin/profile")}>
          <UserIcon />
          {t("layout.profile")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate("/admin/settings")}>
          <UserCog />
          {t("layout.navPreferences")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuLinkItem href="/">
          <ExternalLink />
          {t("layout.viewSite")}
        </DropdownMenuLinkItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onClick={handleLogout}>
          <LogOut />
          {t("layout.logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function AdminShell() {
  const { t } = useTranslation();
  const { unreadCount } = useNotifications();
  const { toggleTheme } = useTheme();
  const isDark = useIsDark();
  const { siteName, renderBrand } = useSiteBrand();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem(COLLAPSE_KEY) === "1",
  );

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      localStorage.setItem(COLLAPSE_KEY, current ? "0" : "1");
      return !current;
    });
  };

  return (
    <div className="flex min-h-screen bg-background">
      {/* Contents column — desktop */}
      <aside
        className={cn(
          "sticky top-0 hidden h-screen shrink-0 flex-col border-r border-border bg-sidebar lg:flex",
          collapsed ? "w-14" : "w-56",
        )}
      >
        <div
          className={cn(
            "flex h-14 shrink-0 items-center border-b border-border",
            collapsed && "justify-center",
          )}
        >
          <Brand collapsed={collapsed} size="size-5" />
        </div>

        <NavRail collapsed={collapsed} unreadCount={unreadCount} />

        {/* A control strip, not a content band: 44px keeps it clear of the
         * 56px masthead above while still landing the button on the rail's
         * 12px gutter. */}
        <div
          className={cn(
            "mt-auto flex h-11 shrink-0 items-center border-t border-border px-3",
            collapsed ? "justify-center" : "justify-end",
          )}
        >
          <button
            type="button"
            onClick={toggleCollapsed}
            aria-label={t("layout.toggleNavigation")}
            title={t("layout.toggleNavigation")}
            className={iconButton}
          >
            {collapsed ? (
              <ChevronsRight className="size-4" />
            ) : (
              <ChevronsLeft className="size-4" />
            )}
          </button>
        </div>
      </aside>

      {/* Contents drawer — mobile */}
      <Dialog open={mobileOpen} onOpenChange={setMobileOpen}>
        <DialogContent
          variant="drawer"
          showCloseButton={false}
          className="bg-sidebar"
        >
          <DialogTitle className="sr-only">
            {t("layout.openNavigation")}
          </DialogTitle>
          <div className="flex h-14 shrink-0 items-center border-b border-border">
            <Brand collapsed={false} size="size-5" />
          </div>
          <div className="p-3">
            <SearchForm onSubmitted={() => setMobileOpen(false)} />
          </div>
          <NavRail
            collapsed={false}
            unreadCount={unreadCount}
            touch
            onNavigate={() => setMobileOpen(false)}
          />
        </DialogContent>
      </Dialog>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background px-3 lg:px-6">
          <button
            type="button"
            className={cn(iconButton, "lg:hidden")}
            onClick={() => setMobileOpen(true)}
            aria-label={t("layout.openNavigation")}
          >
            <Menu className="size-4" />
          </button>
          <span className="flex min-w-0 items-center gap-2 lg:hidden">
            {renderBrand("size-5")}
            <span className="display truncate text-sm">{siteName}</span>
          </span>

          <SearchForm className="hidden min-w-0 max-w-xs flex-1 md:block" />

          <div className="ml-auto flex items-center gap-1.5">
            <button
              type="button"
              className={iconButton}
              onClick={toggleTheme}
              aria-label={t("layout.toggleTheme")}
              title={t("layout.toggleTheme")}
            >
              {isDark ? (
                <Sun className="size-4" />
              ) : (
                <Moon className="size-4" />
              )}
            </button>

            <LanguageMenu />

            <Link
              to="/admin/notifications"
              className={cn(iconButton, "relative")}
              aria-label={t("layout.notifications")}
            >
              <Bell className="size-4" />
              {unreadCount > 0 && (
                <span className="absolute top-1 right-1 size-1.5 rounded-full bg-destructive" />
              )}
            </Link>

            <UserMenu />
          </div>
        </header>

        {/* The content area stays full-width: editors work in tables and
         * side-by-side fields, which a centred reading column would break. */}
        <main className="min-w-0 flex-1 px-4 py-6 lg:px-8 lg:py-8">
          <Suspense fallback={<RouteFallback />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  );
}
