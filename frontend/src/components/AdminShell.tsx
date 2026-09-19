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

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { RouteFallback } from "@/components/RouteFallback";

const COLLAPSE_KEY = "admin.sidebar.collapsed";

/**
 * The console frame: a navigation rail on the left, a utility bar on top, and
 * the page in a full-width content area.
 *
 * A blog console is somewhere people return to daily and stay in for a while,
 * so navigation gets a permanent rail instead of a menu that must be opened
 * first. The content area is deliberately not a centered marketing column —
 * editors work in tables and side-by-side fields.
 */
function NavRailItem({
  item,
  collapsed,
  unreadCount,
  touch,
  onNavigate,
}: {
  item: NavItem;
  collapsed: boolean;
  unreadCount: number;
  /** Taller rows for the mobile drawer, where fingers replace the cursor. */
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
          "flex items-center gap-2.5 rounded-md px-2 text-[13px] font-medium transition-colors",
          touch ? "h-11" : "h-8",
          collapsed && "justify-center px-0",
          isActive
            ? "bg-accent text-foreground"
            : "text-sidebar-foreground hover:bg-accent/60 hover:text-foreground",
        )
      }
    >
      <item.icon className="size-4 shrink-0" aria-hidden="true" />
      {!collapsed && <span className="truncate">{label}</span>}
      {!collapsed && item.unreadBadge && unreadCount > 0 && (
        <span className="bg-foreground text-background ml-auto rounded-sm px-1 text-[10px] leading-4 font-medium tabular-nums">
          {unreadCount > 99 ? "99+" : unreadCount}
        </span>
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
    <nav className="flex flex-1 flex-col gap-5 overflow-y-auto px-2.5 py-4">
      {groups.map((group, index) => (
        <div key={group.titleKey ?? `group-${index}`} className="space-y-0.5">
          {group.titleKey &&
            (collapsed ? (
              <div
                className="border-border mx-2 mb-1.5 border-t"
                role="separator"
              />
            ) : (
              <p className="text-muted-foreground/80 px-2 pb-1.5 text-[11px] font-medium tracking-wide uppercase">
                {t(group.titleKey)}
              </p>
            ))}
          {group.items.map((item) => (
            <NavRailItem
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
        "focus-visible:ring-ring/25 flex min-w-0 items-center gap-2 rounded-md px-2 py-1 outline-none focus-visible:ring-[3px]",
        collapsed && "justify-center px-0",
      )}
      title={siteName}
    >
      {renderBrand(size)}
      {!collapsed && (
        <span className="truncate text-[13px] font-semibold">{siteName}</span>
      )}
    </Link>
  );
}

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
      <div className="relative">
        <Search
          className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
          aria-hidden="true"
        />
        <Input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("layout.searchPlaceholder")}
          aria-label={t("layout.searchPlaceholder")}
          className="h-8 pl-8"
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
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          className="size-9 lg:size-7"
          aria-label={t("layout.language")}
          title={t("layout.language")}
        >
          <Languages className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-40">
        <DropdownMenuLabel>{t("layout.language")}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {options.map((option) => (
          <DropdownMenuItem
            key={option.value}
            onSelect={() => setLocale(option.value)}
          >
            <span className="flex-1">{option.label}</span>
            {locale === option.value && (
              <Check className="text-accent-blue size-4" />
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
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          className="size-9 rounded-full lg:size-7"
          aria-label={user?.username}
        >
          <Avatar className="size-6">
            {user?.avatar ? (
              <img
                src={user.avatar}
                alt=""
                className="size-full object-cover"
              />
            ) : (
              <AvatarFallback className="bg-muted text-[11px] font-medium">
                {user?.username?.charAt(0).toUpperCase()}
              </AvatarFallback>
            )}
          </Avatar>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <div className="px-2 py-1.5">
          <p className="truncate text-[13px] font-medium">{user?.username}</p>
          <p className="text-muted-foreground truncate text-[11px]">
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
        <DropdownMenuItem asChild>
          <a href="/">
            <ExternalLink />
            {t("layout.viewSite")}
          </a>
        </DropdownMenuItem>
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
    <div className="bg-background flex min-h-screen">
      {/* Navigation rail — desktop */}
      <aside
        className={cn(
          "bg-sidebar border-border sticky top-0 hidden h-screen shrink-0 flex-col border-r lg:flex",
          collapsed ? "w-14" : "w-56",
        )}
      >
        <div
          className={cn(
            "border-border flex h-14 shrink-0 items-center border-b",
            collapsed ? "justify-center" : "px-2",
          )}
        >
          <Brand collapsed={collapsed} size="size-5" />
        </div>

        <NavRail collapsed={collapsed} unreadCount={unreadCount} />

        <div
          className={cn(
            "border-border mt-auto shrink-0 border-t p-2",
            collapsed && "flex justify-center",
          )}
        >
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={toggleCollapsed}
            aria-label={t("layout.toggleNavigation")}
            title={t("layout.toggleNavigation")}
          >
            {collapsed ? (
              <ChevronsRight className="size-4" />
            ) : (
              <ChevronsLeft className="size-4" />
            )}
          </Button>
        </div>
      </aside>

      {/* Navigation drawer — mobile */}
      <Dialog open={mobileOpen} onOpenChange={setMobileOpen}>
        <DialogContent
          showCloseButton={false}
          className="bg-sidebar top-0 left-0 flex h-full w-64 max-w-none translate-x-0 translate-y-0 flex-col gap-0 rounded-none border-y-0 border-l-0 p-0 sm:max-w-none"
        >
          <DialogTitle className="sr-only">
            {t("layout.openNavigation")}
          </DialogTitle>
          <div className="border-border flex h-14 shrink-0 items-center border-b px-2">
            <Brand collapsed={false} size="size-5" />
          </div>
          <div className="px-2.5 pt-3">
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
        <header className="border-border bg-background sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b px-3 lg:px-5">
          <Button
            variant="ghost"
            size="icon-sm"
            className="size-9 lg:hidden"
            onClick={() => setMobileOpen(true)}
            aria-label={t("layout.openNavigation")}
          >
            <Menu className="size-4" />
          </Button>
          <span className="flex min-w-0 items-center gap-2 lg:hidden">
            {renderBrand("size-5")}
            <span className="truncate text-[13px] font-semibold">
              {siteName}
            </span>
          </span>

          <SearchForm className="hidden min-w-0 max-w-xs flex-1 md:block" />

          <div className="ml-auto flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon-sm"
              className="size-9 lg:size-7"
              onClick={toggleTheme}
              aria-label={t("layout.toggleTheme")}
              title={t("layout.toggleTheme")}
            >
              {isDark ? (
                <Sun className="size-4" />
              ) : (
                <Moon className="size-4" />
              )}
            </Button>

            <LanguageMenu />

            <Button
              variant="ghost"
              size="icon-sm"
              asChild
              className="relative size-9 lg:size-7"
            >
              <Link
                to="/admin/notifications"
                aria-label={t("layout.notifications")}
              >
                <Bell className="size-4" />
                {unreadCount > 0 && (
                  <span className="bg-destructive text-destructive-foreground absolute -top-0.5 -right-0.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full px-1 text-[10px] font-medium tabular-nums">
                    {unreadCount > 9 ? "9+" : unreadCount}
                  </span>
                )}
              </Link>
            </Button>

            <UserMenu />
          </div>
        </header>

        <main className="min-w-0 flex-1 px-4 py-6 lg:px-6">
          <Suspense fallback={<RouteFallback />}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  );
}
