import { useState } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { authApi, uploadApi } from '@/lib/api';
import { useTranslation } from '@/lib/I18nContext';
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
import ImageCropper from '@/components/image/ImageCropper';

export function ProfilePage() {
  const { user, updateUser } = useAuth();
  const { t } = useTranslation();
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
  const [showCropper, setShowCropper] = useState(false);
  const [selectedAvatarFile, setSelectedAvatarFile] = useState<File | null>(null);

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
      setSuccess(t('profilePage.updateSuccess'));
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : t('profilePage.updateFailed');
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
      setError(t('profilePage.passwordMismatch'));
      return;
    }

    if (newPassword.length < 6) {
      setError(t('profilePage.passwordTooShort'));
      return;
    }

    setPasswordLoading(true);

    try {
      await authApi.changePassword({ oldPassword, newPassword });
      setSuccess(t('profilePage.passwordChangeSuccess'));
      setOldPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : t('profilePage.passwordChangeFailed');
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
      setError(t('profilePage.enterNewEmail'));
      return;
    }

    if (newEmail === user?.email) {
      setError(t('profilePage.emailSameAsCurrent'));
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
      const errorMessage = err instanceof Error ? err.message : t('profilePage.emailChangeFailed');
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

  const handleAvatarChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setSelectedAvatarFile(file);
    setShowCropper(true);
  };

  const handleAvatarCrop = async (croppedFile: File) => {
    setAvatarLoading(true);
    setShowCropper(false);

    try {
      // 使用现有的上传API
      const uploadResponse = await uploadApi.uploadFile(croppedFile);
      if (uploadResponse.data.file && uploadResponse.data.file.url) {
        // 更新用户头像
        const updateResponse = await authApi.updateProfile({ avatar: uploadResponse.data.file.url });
        updateUser(updateResponse.data.user);
        setSuccess(t('profilePage.updateAvatar'));
      }
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : t('profilePage.avatarUpdateFailed');
      setError(errorMessage);
    } finally {
      setAvatarLoading(false);
      setSelectedAvatarFile(null);
    }
  };

  const getRoleLabel = (role?: string) => {
    switch (role) {
      case 'super_admin':
        return t('profilePage.roleSuperAdmin');
      case 'admin':
        return t('profilePage.roleAdmin');
      case 'author':
        return t('profilePage.roleAuthor');
      case 'contributor':
        return t('profilePage.roleContributor');
      default:
        return t('profilePage.roleGuest');
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
            {getRoleLabel(user?.role)}
          </Badge>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="profile" className="w-full">
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="profile">{t('profilePage.profileInfo')}</TabsTrigger>
              <TabsTrigger value="password">{t('profilePage.changePassword')}</TabsTrigger>
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
                  <Label htmlFor="email">{t('profilePage.emailLabel')}</Label>
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
                    {t('profilePage.changeEmailTip')}
                  </p>
                  <Button
                    type="button"
                    variant="outline"
                    className="w-full"
                    onClick={openEmailChangeDialog}
                  >
                    {t('profilePage.changeEmailButton')}
                  </Button>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="username">{t('profilePage.usernameLabel')}</Label>
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
                  <Label htmlFor="birthday">{t('profilePage.birthdayLabel')}</Label>
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
                  <Label htmlFor="bio">{t('profilePage.bioLabel')}</Label>
                  <div className="relative">
                    <UserPlus className="absolute left-3 top-3 w-4 h-4 text-muted-foreground" />
                    <Textarea
                      id="bio"
                      value={bio}
                      onChange={(e) => setBio(e.target.value)}
                      placeholder={t('profilePage.bioPlaceholder')}
                      className="pl-10"
                      rows={3}
                    />
                  </div>
                </div>

                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? (
                    <>
                      <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                      {t('profilePage.saving')}
                    </>
                  ) : (
                    t('profilePage.saveChanges')
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
                  <Label htmlFor="oldPassword">{t('profilePage.currentPassword')}</Label>
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
                  <Label htmlFor="newPassword">{t('profilePage.newPassword')}</Label>
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
                  <Label htmlFor="confirmPassword">{t('profilePage.confirmPassword')}</Label>
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
                      {t('profilePage.saving')}
                    </>
                  ) : (
                    t('profilePage.changePassword')
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
            <AlertDialogTitle>{t('profilePage.changeEmailDialog')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('profilePage.changeEmailDescription', { smtpEnabled: user?.emailVerified ? t('profilePage.smtpEnabled') : t('profilePage.smtpDisabled') })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="py-4">
            <div className="space-y-2">
              <Label htmlFor="newEmailInput">{t('profilePage.newEmail')}</Label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="newEmailInput"
                  type="email"
                  placeholder={t('profilePage.enterNewEmail')}
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="pl-10"
                />
              </div>
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={emailLoading}>{t('common.cancel')}</AlertDialogCancel>
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
                  {t('profilePage.saving')}
                </>
              ) : (
                t('profilePage.confirmChange')
              )}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 头像裁剪 */}
      {showCropper && selectedAvatarFile && (
        <ImageCropper
          file={selectedAvatarFile}
          circle={true}
          aspect={1}
          outputWidth={400}
          outputHeight={400}
          onCancel={() => {
            setShowCropper(false);
            setSelectedAvatarFile(null);
          }}
          onCrop={handleAvatarCrop}
        />
      )}
    </div>
  );
}