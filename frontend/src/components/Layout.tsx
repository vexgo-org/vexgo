import { Link, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Input } from '@/components/ui/input';
import {
  Search, PenLine, Menu, X, Home, User, Settings,
  LogOut, FileText, BarChart3, Users
} from 'lucide-react';
import { useState } from 'react';

interface LayoutProps {
  children: React.ReactNode;
}

export function Layout({ children }: LayoutProps) {
  const { user, isAuthenticated, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState('');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      navigate(`/?search=${encodeURIComponent(searchQuery.trim())}`);
    }
  };

  const handleLogout = () => {
    logout();
    navigate('/');
  };

  const navItems = [
    { path: '/', label: '首页', icon: Home },
    ...(isAuthenticated ? [{ path: '/write', label: '写文章', icon: PenLine }] : []),
    ...(user?.role === 'admin' || user?.role === 'super_admin' ? [{ path: '/admin', label: '管理后台', icon: BarChart3 }] : []),
    ...(user?.role === 'admin' || user?.role === 'super_admin' ? [{ path: '/moderation', label: '内容审核', icon: FileText }] : []),
    ...(user?.role === 'admin' || user?.role === 'super_admin' ? [{ path: '/admin/users', label: '用户管理', icon: Users }] : []),
  ];

  const isActive = (path: string) => {
    if (path === '/') {
      return location.pathname === '/';
    }
    // 对于管理后台的特定路由，需要精确匹配
    if (path === '/admin') {
      return location.pathname === '/admin';
    }
    return location.pathname.startsWith(path);
  };

  return (
    <div className="min-h-screen flex flex-col bg-background">
      {/* 导航栏 */}
      <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
        <div className="container mx-auto px-4">
          <div className="flex h-16 items-center justify-between gap-4">
            {/* Logo */}
            <Link to="/" className="flex items-center gap-2 shrink-0">
              <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
                <PenLine className="w-5 h-5 text-primary-foreground" />
              </div>
              <span className="text-xl font-bold hidden sm:inline">BlogHub</span>
            </Link>

            {/* 搜索框 - 桌面端 */}
            <form onSubmit={handleSearch} className="hidden md:flex flex-1 max-w-md">
              <div className="relative w-full">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  type="search"
                  placeholder="搜索文章..."
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
                  variant={isActive(item.path) ? 'default' : 'ghost'}
                  size="sm"
                  asChild
                >
                  <Link to={item.path} className="flex items-center gap-2">
                    <item.icon className="w-4 h-4" />
                    {item.label}
                  </Link>
                </Button>
              ))}
            </nav>

            {/* 用户菜单 */}
            <div className="flex items-center gap-2">
              {isAuthenticated ? (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button variant="ghost" className="relative h-9 w-9 rounded-full">
                      <Avatar className="h-9 w-9">
                        <AvatarFallback className="bg-primary/10 text-primary">
                          {user?.username?.charAt(0).toUpperCase()}
                        </AvatarFallback>
                      </Avatar>
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent className="w-56" align="end" forceMount>
                    <div className="flex items-center gap-2 p-2">
                      <Avatar className="h-8 w-8">
                        <AvatarFallback className="bg-primary/10 text-primary text-sm">
                          {user?.username?.charAt(0).toUpperCase()}
                        </AvatarFallback>
                      </Avatar>
                      <div className="flex flex-col">
                        <p className="text-sm font-medium">{user?.username}</p>
                        <p className="text-xs text-muted-foreground">{user?.email}</p>
                      </div>
                    </div>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => navigate('/profile')}>
                      <User className="mr-2 h-4 w-4" />
                      个人中心
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => navigate('/my-posts')}>
                      <FileText className="mr-2 h-4 w-4" />
                      我的文章
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => navigate('/settings')}>
                      <Settings className="mr-2 h-4 w-4" />
                      设置
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={handleLogout} className="text-destructive">
                      <LogOut className="mr-2 h-4 w-4" />
                      退出登录
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : (
                <div className="flex items-center gap-2">
                  <Button variant="ghost" size="sm" asChild>
                    <Link to="/login">登录</Link>
                  </Button>
                  <Button size="sm" asChild>
                    <Link to="/register">注册</Link>
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
                {mobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
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
                    placeholder="搜索文章..."
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
                    variant={isActive(item.path) ? 'default' : 'ghost'}
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
      <main className="flex-1">
        {children}
      </main>

      {/* 页脚 */}
      <footer className="border-t bg-muted/50">
        <div className="container mx-auto px-4 py-8">
          <div className="flex flex-col md:flex-row justify-between items-center gap-4">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 bg-primary rounded flex items-center justify-center">
                <PenLine className="w-4 h-4 text-primary-foreground" />
              </div>
              <span className="font-semibold">BlogHub</span>
            </div>
            <p className="text-sm text-muted-foreground">
              © 2024 BlogHub. All rights reserved.
            </p>
            <div className="flex gap-4">
              <Link to="/" className="text-sm text-muted-foreground hover:text-foreground">
                首页
              </Link>
              <Link to="/about" className="text-sm text-muted-foreground hover:text-foreground">
                关于
              </Link>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
