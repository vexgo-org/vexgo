import { Link, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useNotifications } from "@/hooks/useNotifications";
import { configApi } from "@/lib/api";
import { useTranslation } from "@/lib/I18nContext";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Input } from "@/components/ui/input";
import {
  Search,
  PenLine,
  Menu,
  X,
  Home,
  User,
  Settings,
  LogOut,
  FileText,
  BarChart3,
  Bell,
} from "lucide-react";
import { useEffect, useState } from "react";

interface LayoutProps {
  children: React.ReactNode;
}

export function Layout({ children }: LayoutProps) {
  const { t } = useTranslation();
  const { user, isAuthenticated, logout } = useAuth();
  const { unreadCount } = useNotifications();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState("");
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [siteName, setSiteName] = useState("VexGo");
  const [siteIcon, setSiteIcon] = useState("");
  const [allowGuestView, setAllowGuestView] = useState(true);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadSettings = async () => {
      try {
        const response = await configApi.getGeneralSettings();
        if (response.data.siteName) {
          setSiteName(response.data.siteName);
          document.title = response.data.siteName;
        }
        if (response.data.siteIcon) {
          setSiteIcon(response.data.siteIcon);
          // Update favicon link
          let link =
            document.querySelector<HTMLLinkElement>("link[rel~='icon']");
          if (!link) {
            link = document.createElement("link");
            link.rel = "icon";
            document.head.appendChild(link);
          }
          link.href = response.data.siteIcon;
        }
        setAllowGuestView(response.data.allowGuestViewPosts !== false);
      } catch (error) {
        console.error(t("common.error"), error);
      } finally {
        setLoading(false);
      }
    };
    loadSettings();
  }, [t]);

  // Check whether we need to redirect to the login page
  useEffect(() => {
    if (!loading && !isAuthenticated && !allowGuestView) {
      // Only redirect on non-login pages when login is required
      if (location.pathname !== "/login" && location.pathname !== "/register") {
        navigate("/login");
      }
    }
  }, [loading, isAuthenticated, allowGuestView, navigate, location.pathname]);

  const handleSearch = (e: React.SubmitEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      if (isAuthenticated) {
        navigate(`/?search=${encodeURIComponent(searchQuery.trim())}`);
      } else {
        navigate("/login");
      }
    }
  };

  const handleLogout = () => {
    logout();
    navigate("/");
  };

  const navItems = [
    { path: "/", label: t("layout.home"), icon: Home },
    ...(isAuthenticated && user?.role !== "guest"
      ? [{ path: "/write", label: t("layout.writePost"), icon: PenLine }]
      : []),
    ...(isAuthenticated && user?.role !== "guest"
      ? [{ path: "/my-posts", label: t("layout.myPosts"), icon: FileText }]
      : []),
    ...(isAuthenticated
      ? [
          {
            path: "/notifications",
            label: t("layout.notifications"),
            icon: Bell,
          },
        ]
      : []),
    ...(user?.role === "admin" || user?.role === "super_admin"
      ? [{ path: "/admin", label: t("layout.adminPanel"), icon: BarChart3 }]
      : []),
  ];

  const isActive = (path: string) => {
    if (path === "/") {
      return location.pathname === "/";
    }
    // Admin routes need exact matching
    if (path === "/admin") {
      return location.pathname === "/admin";
    }
    return location.pathname.startsWith(path);
  };

  if (loading) {
    return null; // or render a loading state
  }

  return (
    <div className="min-h-screen flex flex-col bg-background">
      {/* Navbar */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container mx-auto px-4">
          <div className="flex h-16 items-center justify-between gap-4">
            {/* Logo */}
            <Link to="/" className="flex items-center gap-2 shrink-0">
              {siteIcon ? (
                <img src={siteIcon} alt="Logo" className="w-8 h-8" />
              ) : (
                <>
                  <img
                    src="/assets/vexgo-light.ico"
                    alt="Logo"
                    className="w-8 h-8 dark:hidden"
                  />
                  <img
                    src="/assets/vexgo-dark.ico"
                    alt="Logo"
                    className="w-8 h-8 hidden dark:block"
                  />
                </>
              )}
              <span className="text-xl font-bold hidden sm:inline">
                {siteName}
              </span>
            </Link>

            {/* Search box - desktop */}
            <form
              onSubmit={handleSearch}
              className="hidden md:flex flex-1 max-w-md"
            >
              <div className="relative w-full">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  type="search"
                  placeholder={t("layout.searchPlaceholder")}
                  className="pl-10 w-full"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </form>

            {/* Nav links - desktop */}
            <nav className="hidden md:flex items-center gap-1">
              {navItems.map((item) => (
                <Button
                  key={item.path}
                  variant={isActive(item.path) ? "default" : "ghost"}
                  size="sm"
                  asChild
                  className={item.path === "/notifications" ? "relative" : ""}
                >
                  <Link to={item.path} className="flex items-center gap-2">
                    <item.icon className="w-4 h-4" />
                    {item.label}
                    {item.path === "/notifications" && unreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 bg-destructive text-destructive-foreground text-xs rounded-full w-4 h-4 flex items-center justify-center">
                        {unreadCount}
                      </span>
                    )}
                  </Link>
                </Button>
              ))}
            </nav>

            {/* User menu */}
            <div className="flex items-center gap-2">
              {isAuthenticated ? (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      variant="ghost"
                      className="relative h-9 w-9 rounded-full"
                    >
                      <Avatar className="h-9 w-9">
                        {user?.avatar ? (
                          <img
                            src={user.avatar}
                            alt="Avatar"
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <AvatarFallback className="bg-primary/10 text-primary">
                            {user?.username?.charAt(0).toUpperCase()}
                          </AvatarFallback>
                        )}
                      </Avatar>
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent className="w-56" align="end" forceMount>
                    <div className="flex items-center gap-2 p-2">
                      <Avatar className="h-8 w-8">
                        {user?.avatar ? (
                          <img
                            src={user.avatar}
                            alt="Avatar"
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <AvatarFallback className="bg-primary/10 text-primary text-sm">
                            {user?.username?.charAt(0).toUpperCase()}
                          </AvatarFallback>
                        )}
                      </Avatar>
                      <div className="flex flex-col">
                        <p className="text-sm font-medium">{user?.username}</p>
                        <p className="text-xs text-muted-foreground">
                          {user?.email}
                        </p>
                      </div>
                    </div>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => navigate("/profile")}>
                      <User className="mr-2 h-4 w-4" />
                      {t("layout.profile")}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => navigate("/settings")}>
                      <Settings className="mr-2 h-4 w-4" />
                      {t("layout.settings")}
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      onClick={handleLogout}
                      className="text-destructive"
                    >
                      <LogOut className="mr-2 h-4 w-4" />
                      {t("layout.logout")}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : (
                <div className="flex items-center gap-2">
                  <Button variant="ghost" size="sm" asChild>
                    <Link to="/login">{t("layout.login")}</Link>
                  </Button>
                  <Button size="sm" asChild>
                    <Link to="/register">{t("layout.registerText")}</Link>
                  </Button>
                </div>
              )}

              {/* Mobile menu button */}
              <Button
                variant="ghost"
                size="icon"
                className="md:hidden"
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              >
                {mobileMenuOpen ? (
                  <X className="w-5 h-5" />
                ) : (
                  <Menu className="w-5 h-5" />
                )}
              </Button>
            </div>
          </div>

          {/* Mobile menu */}
          {mobileMenuOpen && (
            <div className="md:hidden border-t py-4 space-y-4">
              {/* Search box */}
              <form onSubmit={handleSearch}>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                  <Input
                    type="search"
                    placeholder={t("layout.searchPlaceholder")}
                    className="pl-10 w-full"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                </div>
              </form>

              {/* Nav links */}
              <nav className="flex flex-col gap-2">
                {navItems.map((item) => (
                  <Button
                    key={item.path}
                    variant={isActive(item.path) ? "default" : "ghost"}
                    className="justify-start"
                    asChild
                    onClick={() => setMobileMenuOpen(false)}
                  >
                    <Link to={item.path} className="flex items-center gap-2">
                      <item.icon className="w-4 h-4" />
                      {item.label}
                    </Link>
                  </Button>
                ))}
              </nav>
            </div>
          )}
        </div>
      </header>

      {/* Main content */}
      <main className="flex-1">{children}</main>

      {/* Footer */}
      <footer className="border-t bg-muted/50">
        <div className="container mx-auto px-4 py-8">
          <div className="flex flex-col md:flex-row justify-between items-center gap-4">
            <div className="flex items-center gap-2">
              {siteIcon ? (
                <img src={siteIcon} alt="Logo" className="w-6 h-6" />
              ) : (
                <>
                  <img
                    src="/assets/vexgo-light.ico"
                    alt="Logo"
                    className="w-6 h-6 dark:hidden"
                  />
                  <img
                    src="/assets/vexgo-dark.ico"
                    alt="Logo"
                    className="w-6 h-6 hidden dark:block"
                  />
                </>
              )}
              <span className="font-semibold">{siteName}</span>
            </div>
            <p className="text-sm text-muted-foreground">
              {t("layout.allRightsReserved", { siteName })}
            </p>
            <div className="flex gap-4">
              <Link
                to="/"
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                {t("layout.home")}
              </Link>
              <Link
                to="/about"
                className="text-sm text-muted-foreground hover:text-foreground"
              >
                {t("layout.about")}
              </Link>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
