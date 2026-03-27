import { Link, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { configApi, messagesApi } from "@/lib/api";
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
  const navigate = useNavigate();
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState("");
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [siteName, setSiteName] = useState("VexGo");
  const [allowGuestView, setAllowGuestView] = useState(true);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadSettings = async () => {
      try {
        const response = await configApi.getGeneralSettings();
        if (response.data.siteName) {
          setSiteName(response.data.siteName);
          // 更新网页标题
          document.title = response.data.siteName;
        }
        // 加载允许访客浏览的设置
        setAllowGuestView(response.data.allowGuestViewPosts !== false);
      } catch (error) {
        console.error(t("common.error"), error);
      } finally {
        setLoading(false);
      }
    };
    loadSettings();
  }, []);

  // 检查是否需要重定向到登录页面
  useEffect(() => {
    if (!loading && !isAuthenticated && !allowGuestView) {
      // 只在非登录页面且需要登录时重定向
      if (location.pathname !== "/login" && location.pathname !== "/register") {
        navigate("/login");
      }
    }
  }, [loading, isAuthenticated, allowGuestView, navigate, location.pathname]);

  const handleSearch = (e: React.FormEvent) => {
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

  // 消息数量
  const [unreadCount, setUnreadCount] = useState(0);

  // 获取未读消息数量的函数
  const fetchUnreadCount = async () => {
    try {
      const response = await messagesApi.getUnreadCount();
      setUnreadCount(response.data.unreadCount);
    } catch (error) {
      console.error("获取未读消息数量失败:", error);
    }
  };

  useEffect(() => {
    // 只有登录后才获取未读消息数量
    if (isAuthenticated) {
      fetchUnreadCount();
    }
  }, [isAuthenticated]);

  // 定期检查未读消息数量（每30秒）
  useEffect(() => {
    let interval: NodeJS.Timeout;
    if (isAuthenticated) {
      interval = setInterval(() => {
        fetchUnreadCount();
      }, 30000); // 30秒检查一次
    }
    return () => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, [isAuthenticated]);

  // 监听路由变化，当进入或离开消息页面时更新未读数量
  useEffect(() => {
    if (isAuthenticated) {
      // 当路由变化时检查未读消息数量
      fetchUnreadCount();
    }
  }, [isAuthenticated, location.pathname]);

  const navItems = [
    { path: "/", label: t("layout.home"), icon: Home },
    ...(isAuthenticated && user?.role !== "guest"
      ? [{ path: "/write", label: t("layout.writePost"), icon: PenLine }]
      : []),
    ...(isAuthenticated && user?.role !== "guest"
      ? [{ path: "/my-posts", label: t("layout.myPosts"), icon: FileText }]
      : []),
    ...(isAuthenticated
      ? [{ path: "/messages", label: t("layout.messages"), icon: Bell }]
      : []),
    ...(user?.role === "admin" || user?.role === "super_admin"
      ? [{ path: "/admin", label: t("layout.adminPanel"), icon: BarChart3 }]
      : []),
  ];

  const isActive = (path: string) => {
    if (path === "/") {
      return location.pathname === "/";
    }
    // 对于管理后台的特定路由，需要精确匹配
    if (path === "/admin") {
      return location.pathname === "/admin";
    }
    return location.pathname.startsWith(path);
  };

  if (loading) {
    return null; // 或者返回一个加载状态
  }

  return (
    <div className="min-h-screen flex flex-col bg-background">
      {/* 导航栏 */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container mx-auto px-4">
          <div className="flex h-16 items-center justify-between gap-4">
            {/* Logo */}
            <Link to="/" className="flex items-center gap-2 shrink-0">
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
              <span className="text-xl font-bold hidden sm:inline">
                {siteName}
              </span>
            </Link>

            {/* 搜索框 - 桌面端 */}
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

            {/* 导航链接 - 桌面端 */}
            <nav className="hidden md:flex items-center gap-1">
              {navItems.map((item) => (
                <Button
                  key={item.path}
                  variant={isActive(item.path) ? "default" : "ghost"}
                  size="sm"
                  asChild
                  className={item.path === "/messages" ? "relative" : ""}
                >
                  <Link to={item.path} className="flex items-center gap-2">
                    <item.icon className="w-4 h-4" />
                    {item.label}
                    {item.path === "/messages" && unreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 bg-destructive text-destructive-foreground text-xs rounded-full w-4 h-4 flex items-center justify-center">
                        {unreadCount}
                      </span>
                    )}
                  </Link>
                </Button>
              ))}
            </nav>

            {/* 用户菜单 */}
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

              {/* 移动端菜单按钮 */}
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

          {/* 移动端菜单 */}
          {mobileMenuOpen && (
            <div className="md:hidden border-t py-4 space-y-4">
              {/* 搜索框 */}
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

              {/* 导航链接 */}
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

      {/* 主内容 */}
      <main className="flex-1">{children}</main>

      {/* 页脚 */}
      <footer className="border-t bg-muted/50">
        <div className="container mx-auto px-4 py-8">
          <div className="flex flex-col md:flex-row justify-between items-center gap-4">
            <div className="flex items-center gap-2">
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
