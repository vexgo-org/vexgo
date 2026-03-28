import { useEffect, useState } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { useTranslation } from '@/lib/I18nContext';
import { getUsers, updateUserRole, deleteUser } from '@/lib/userApi';
import type { User } from '@/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { toast } from 'sonner';
import {
  Users, UserCheck, Trash2, Search
} from 'lucide-react';
import { getLocale } from '@/lib/i18n';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from '@/components/ui/alert-dialog';

export function UserManagementPage() {
  const { user: currentUser } = useAuth();
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [searchTerm, setSearchTerm] = useState('');
  const [searchQuery, setSearchQuery] = useState('');

  // 角色显示映射
  const roleDisplayMap: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
    'super_admin': { label: t('roles.super_admin'), variant: 'destructive' },
    'admin': { label: t('roles.admin'), variant: 'default' },
    'author': { label: t('roles.author'), variant: 'secondary' },
    'contributor': { label: t('roles.contributor'), variant: 'outline' },
    'guest': { label: t('roles.guest'), variant: 'outline' }
  };

  // 可分配的角色选项（根据当前用户角色确定）
  const getAssignableRoles = () => {
    if (currentUser?.role === 'super_admin') {
      return [
        { value: 'admin', label: t('roles.admin') },
        { value: 'author', label: t('roles.author') },
        { value: 'contributor', label: t('roles.contributor') },
        { value: 'guest', label: t('roles.guest') }
      ];
    } else if (currentUser?.role === 'admin') {
      return [
        { value: 'author', label: t('roles.author') },
        { value: 'contributor', label: t('roles.contributor') },
        { value: 'guest', label: t('roles.guest') }
      ];
    }
    return [];
  };

  useEffect(() => {
    loadData();
  }, [currentPage, searchQuery]);

  const loadData = async () => {
    setLoading(true);
    try {
      const response = await getUsers({ page: currentPage, limit: 10, search: searchQuery });
      setUsers(response.data.users);
      setTotalPages(response.data.pagination.totalPages);
    } catch (error) {
      console.error('加载用户列表失败:', error);
      toast.error(t('userManagement.loadingUsers'));
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = () => {
    setSearchQuery(searchTerm);
    setCurrentPage(1); // 搜索时重置到第一页
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  const handleRoleChange = async (userId: string, newRole: string) => {
    try {
      const response = await updateUserRole(userId, newRole);
      toast.success(response.data.message);

      // 更新本地用户列表
      setUsers(prevUsers =>
        prevUsers.map(user =>
          user.id === userId ? { ...user, role: newRole as 'super_admin' | 'admin' | 'author' | 'contributor' | 'guest' } : user
        )
      );
    } catch (error) {
      console.error('更新用户角色失败:', error);
      toast.error(t('userManagement.updateRoleFailed'));
    }
  };

  const handleDeleteUser = async (userId: string) => {
    try {
      const response = await deleteUser(userId);
      toast.success(response.data.message);

      // 从本地用户列表中移除删除的用户
      setUsers(prevUsers => prevUsers.filter(user => user.id !== userId));
    } catch (error) {
      console.error('删除用户失败:', error);
      toast.error(t('userManagement.deleteUserFailed'));
    }
  };

  // 检查是否可以删除用户
  const canDeleteUser = (user: User) => {
    if (currentUser?.id === user.id) return false; // 不能删除自己
    if (currentUser?.role === 'super_admin') return true; // 超级管理员可以删除任何用户
    if (currentUser?.role === 'admin') {
      // 管理员只能删除作者、贡献者和访客
      return ['author', 'contributor', 'guest'].includes(user.role);
    }
    return false;
  };

  const formatDate = (dateString: string) => {
    const locale = getLocale();
    return new Date(dateString).toLocaleDateString(locale, {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    });
  };

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Users className="w-6 h-6" />
            {t('userManagement.title')}
          </h1>
        </div>
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardContent className="p-6">
                <div className="animate-pulse space-y-4">
                  <div className="h-6 bg-muted rounded w-3/4"></div>
                  <div className="h-4 bg-muted rounded w-1/2"></div>
                  <div className="h-4 bg-muted rounded w-1/4"></div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-2xl font-bold flex items-center gap-2">
          <Users className="w-6 h-6" />
          {t('userManagement.title')}
        </h1>
        <div className="relative w-64">
          <Input
            type="text"
            placeholder={t('userManagement.searchPlaceholder')}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            onKeyPress={handleKeyPress}
            className="pl-10"
          />
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground w-4 h-4" />
          <Button
            variant="ghost"
            size="sm"
            className="absolute right-2 top-1/2 transform -translate-y-1/2 h-8 w-8 p-0"
            onClick={handleSearch}
          >
            <Search className="w-4 h-4" />
          </Button>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <UserCheck className="w-5 h-5" />
            {t('userManagement.userList')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {users.map((user) => (
              <div 
                key={user.id} 
                className="flex items-center justify-between p-4 border rounded-lg hover:bg-muted/50 transition-colors"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-2">
                    <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                      <span className="text-primary font-medium">
                        {user.username.charAt(0).toUpperCase()}
                      </span>
                    </div>
                    <div>
                      <h3 className="font-medium">{user.username}</h3>
                      <p className="text-sm text-muted-foreground">{user.email}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-4 text-sm text-muted-foreground">
                    <span>{t('userManagement.registered')}: {user.createdAt ? formatDate(user.createdAt) : t('userManagement.unknown')}</span>
                  </div>
                </div>
                
                <div className="flex items-center gap-4">
                  <Badge variant={roleDisplayMap[user.role]?.variant || 'outline'}>
                    {roleDisplayMap[user.role]?.label || user.role}
                  </Badge>
                  
                  {currentUser?.id !== user.id && !(currentUser?.role === 'admin' && user.role === 'admin') && (
                    <div className="flex items-center gap-2">
                      <Select
                        value={user.role}
                        onValueChange={(value) => handleRoleChange(user.id, value)}
                      >
                        <SelectTrigger className="w-32">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {getAssignableRoles().map((role) => (
                            <SelectItem key={role.value} value={role.value}>
                              {role.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                  
                  {canDeleteUser(user) && (
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" size="sm">
                          <Trash2 className="w-4 h-4 mr-1" />
                          {t('userManagement.delete')}
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>{t('userManagement.deleteConfirmation')}</AlertDialogTitle>
                          <AlertDialogDescription>
                            {t('userManagement.deleteDescription', { username: user.username })}
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
                          <AlertDialogAction onClick={() => handleDeleteUser(user.id)}>
                            {t('userManagement.delete')}
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                  
                  {currentUser?.id !== user.id && currentUser?.role === 'admin' && user.role === 'admin' && (
                    <Badge variant="secondary">{t('userManagement.sameLevel')}</Badge>
                  )}

                  {currentUser?.id === user.id && (
                    <Badge variant="secondary">{t('userManagement.currentUser')}</Badge>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* 分页 */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-6">
              <div className="text-sm text-muted-foreground">
                {t('userManagement.page', { page: currentPage, totalPages })}
              </div>
             <div className="flex gap-2">
               <Button
                 variant="outline"
                 size="sm"
                 onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                 disabled={currentPage === 1}
               >
                 {t('userManagement.previousPage')}
               </Button>
               <Button
                 variant="outline"
                 size="sm"
                 onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
                 disabled={currentPage === totalPages}
               >
                 {t('userManagement.nextPage')}
               </Button>
             </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}