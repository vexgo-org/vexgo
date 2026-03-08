import { useState } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { authApi, uploadApi } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Loader2, User, Mail, Key, Check, Calendar, UserPlus, Eye, EyeOff, Camera } from 'lucide-react';
import { Badge } from '@/components/ui/badge';

export function ProfilePage() {
  const { user, updateUser } = useAuth();
  const [username, setUsername] = useState(user?.username || '');
  const [birthday, setBirthday] = useState(user?.birthday || '');
  const [bio, setBio] = useState(user?.bio || '');
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [emailLoading, setEmailLoading] = useState(false);
  const [success, setSuccess] = useState('');
  const [error, setError] = useState('');
  const [showEmailDialog, setShowEmailDialog] = useState(false);
  const [showOldPassword, setShowOldPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [avatarLoading, setAvatarLoading] = useState(false);

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setLoading(true);

    try {
      const response = await authApi.updateProfile({ 
        username, 
        birthday, 
        bio
      });
      updateUser(response.data.user);
      setSuccess('个人信息更新成功');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '更新失败';
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');

    if (newPassword !== confirmPassword) {
      setError('两次输入的密码不一致');
      return;
    }

    if (newPassword.length < 6) {
      setError('新密码至少需要6个字符');
      return;
    }

    setPasswordLoading(true);

    try {
      await authApi.changePassword({ oldPassword, newPassword });
      setSuccess('密码修改成功');
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '密码修改失败';
      setError(errorMessage);
    } finally {
      setPasswordLoading(false);
    }
  };

  const handleUpdateEmail = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');
    setShowEmailDialog(false);

    if (!newEmail) {
      setError('请输入新邮箱地址');
      return;
    }

    if (newEmail === user?.email) {
      setError('新邮箱不能与当前邮箱相同');
      return;
    }

    setEmailLoading(true);

    try {
      const response = await authApi.updateEmail({ email: newEmail });
      setSuccess(response.data.message);
      setNewEmail('');
      if (response.data.pending) {
        // 如果返回 pending: true，表示需要验证邮件，等待用户点击链接
        // 不需要更新本地用户信息，等验证后再更新
      } else if (response.data.user) {
        // 如果直接更新成功（SMTP未启用），更新本地用户信息
        updateUser(response.data.user);
      } else {
        // 如果pending状态，更新本地邮箱显示（等待验证）
        if (user) {
          updateUser({ ...user, email: newEmail });
        }
      }
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '邮箱修改失败';
      setError(errorMessage);
    } finally {
      setEmailLoading(false);
    }
  };

  const openEmailChangeDialog = () => {
    setError('');
    setSuccess('');
    setShowEmailDialog(true);
  };

  const handleAvatarClick = () => {
    document.getElementById('avatar-upload')?.click();
  };

  const handleAvatarChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setAvatarLoading(true);

    try {
      // 使用现有的上传API
      const uploadResponse = await uploadApi.uploadFile(file);
      if (uploadResponse.data.file && uploadResponse.data.file.url) {
        // 更新用户头像
        const updateResponse = await authApi.updateProfile({ avatar: uploadResponse.data.file.url });
        updateUser(updateResponse.data.user);
        setSuccess('头像更新成功');
      }
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : '头像更新失败';
      setError(errorMessage);
    } finally {
      setAvatarLoading(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-8 max-w-2xl">
      <Card>
        <CardHeader className="text-center">
          <div className="flex justify-center mb-4">
            <div className="relative cursor-pointer" onClick={handleAvatarClick}>
              <Avatar className="w-24 h-24">
                {user?.avatar ? (
                  <img src={user.avatar} alt="Avatar" className="w-full h-full object-cover" />
                ) : (
                  <AvatarFallback className="bg-primary/10 text-primary text-3xl">
                    {user?.username?.charAt(0).toUpperCase()}
                  </AvatarFallback>
                )}
              </Avatar>
              <div className="absolute inset-0 bg-black/30 rounded-full flex items-center justify-center opacity-0 hover:opacity-100 transition-opacity">
                {avatarLoading ? (
                  <Loader2 className="w-8 h-8 text-white animate-spin" />
                ) : (
                  <Camera className="w-8 h-8 text-white" />
                )}
              </div>
              <input
                type="file"
                id="avatar-upload"
                accept="image/*"
                className="hidden"
                onChange={handleAvatarChange}
              />
            </div>
          </div>
          <CardTitle className="text-2xl">{user?.username}</CardTitle>
          <CardDescription>{user?.email}</CardDescription>
          <Badge variant={user?.role === 'admin' || user?.role === 'super_admin' ? 'default' : 'secondary'} className="mt-2">
            {user?.role === 'super_admin'
              ? '超级管理员'
              : user?.role === 'admin'
              ? '管理员'
              : user?.role === 'author'
              ? '作者'
              : user?.role === 'contributor'
              ? '贡献者'
              : '访客'}
          </Badge>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="profile" className="w-full">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="profile">个人信息</TabsTrigger>
              <TabsTrigger value="password">修改密码</TabsTrigger>
            </TabsList>

            <TabsContent value="profile">
              <form onSubmit={handleUpdateProfile} className="space-y-4 mt-4">
                {success && (
                  <Alert className="bg-green-50 border-green-200">
                    <Check className="w-4 h-4 text-green-600" />
                    <AlertDescription className="text-green-600">{success}</AlertDescription>
                  </Alert>
                )}
                {error && (
                  <Alert variant="destructive">
                    <AlertDescription>{error}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-2">
                  <Label htmlFor="email">邮箱</Label>
                  <div className="relative">
                    <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="email"
                      type="email"
                      value={user?.email}
                      disabled
                      className="pl-10 bg-muted"
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    如需修改邮箱，请点击下方"修改邮箱"按钮
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full"
                    onClick={openEmailChangeDialog}
                  >
                    修改邮箱
                  </Button>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="username">用户名</Label>
                  <div className="relative">
                    <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="username"
                      type="text"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      className="pl-10"
                      minLength={3}
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="birthday">生日</Label>
                  <div className="relative">
                    <Calendar className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="birthday"
                      type="date"
                      value={birthday}
                      onChange={(e) => setBirthday(e.target.value)}
                      className="pl-10"
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="bio">个性签名</Label>
                  <div className="relative">
                    <UserPlus className="absolute left-3 top-3 w-4 h-4 text-muted-foreground" />
                    <Textarea
                      id="bio"
                      value={bio}
                      onChange={(e) => setBio(e.target.value)}
                      placeholder="请输入个性签名"
                      className="pl-10"
                      rows={3}
                    />
                  </div>
                </div>



                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      保存中...
                    </>
                  ) : (
                    '保存修改'
                  )}
                </Button>
              </form>
            </TabsContent>

            <TabsContent value="password">
              <form onSubmit={handleChangePassword} className="space-y-4 mt-4">
                {success && (
                  <Alert className="bg-green-50 border-green-200">
                    <Check className="w-4 h-4 text-green-600" />
                    <AlertDescription className="text-green-600">{success}</AlertDescription>
                  </Alert>
                )}
                {error && (
                  <Alert variant="destructive">
                    <AlertDescription>{error}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-2">
                  <Label htmlFor="oldPassword">当前密码</Label>
                  <div className="relative">
                    <Key className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="oldPassword"
                      type={showOldPassword ? 'text' : 'password'}
                      value={oldPassword}
                      onChange={(e) => setOldPassword(e.target.value)}
                      className="pl-10 pr-10"
                      required
                    />
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                      onClick={() => setShowOldPassword(!showOldPassword)}
                    >
                      {showOldPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="newPassword">新密码</Label>
                  <div className="relative">
                    <Key className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="newPassword"
                      type={showNewPassword ? 'text' : 'password'}
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className="pl-10 pr-10"
                      required
                      minLength={6}
                    />
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                      onClick={() => setShowNewPassword(!showNewPassword)}
                    >
                      {showNewPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="confirmPassword">确认新密码</Label>
                  <div className="relative">
                    <Key className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="confirmPassword"
                      type={showConfirmPassword ? 'text' : 'password'}
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="pl-10 pr-10"
                      required
                    />
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                      onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                    >
                      {showConfirmPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                <Button type="submit" className="w-full" disabled={passwordLoading}>
                  {passwordLoading ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      修改中...
                    </>
                  ) : (
                    '修改密码'
                  )}
                </Button>
              </form>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {/* 邮箱修改确认对话框 */}
      <AlertDialog open={showEmailDialog} onOpenChange={setShowEmailDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>修改邮箱地址</AlertDialogTitle>
            <AlertDialogDescription>
              请输入新的邮箱地址。{user?.emailVerified ? '由于您已启用邮件验证，系统将发送一封确认邮件到您的新邮箱，请点击邮件中的链接完成验证。' : '由于未启用SMTP，邮箱将直接更新。'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="py-4">
            <div className="space-y-2">
              <Label htmlFor="newEmailInput">新邮箱</Label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="newEmailInput"
                  type="email"
                  placeholder="请输入新邮箱地址"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={emailLoading}>取消</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                handleUpdateEmail(e);
              }}
              disabled={emailLoading || !newEmail}
            >
              {emailLoading ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  处理中...
                </>
              ) : (
                '确认修改'
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}