import { useState, useEffect } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { useTranslation } from '@/lib/I18nContext';
import { authApi } from '@/lib/api';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Check, Palette, Shield, Loader2 } from 'lucide-react';

export function SettingsPage() {
  const { user, updateUser } = useAuth();
  const { locale, setLocale: setI18nLocale, t } = useTranslation();
  const [loading, setLoading] = useState(false);
  
  // Display settings
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>(() => {
    const savedTheme = localStorage.getItem('theme');
    return (savedTheme as 'light' | 'dark' | 'system') || 'light';
  });
  const [language, setLanguage] = useState(() => {
    const savedLocale = localStorage.getItem('locale');
    return savedLocale === 'zh-CN' || savedLocale === 'en-US' ? savedLocale : locale;
  });
  
  // Privacy settings - 从 user 对象或 localStorage 初始化
  const [profileVisibility, setProfileVisibility] = useState<'public' | 'private'>(() => {
    const savedSettings = localStorage.getItem('userSettings');
    if (user?.profile_visibility) {
      return user.profile_visibility as 'public' | 'private';
    }
    if (savedSettings) {
      try {
        const settings = JSON.parse(savedSettings);
        return settings.profileVisibility !== undefined ? settings.profileVisibility : 'public';
      } catch {
        return 'public';
      }
    }
    return 'public';
  });
  const [hideEmail, setHideEmail] = useState(() => {
    const savedSettings = localStorage.getItem('userSettings');
    if (user?.hide_email !== undefined) {
      return user.hide_email;
    }
    if (savedSettings) {
      try {
        const settings = JSON.parse(savedSettings);
        return settings.hideEmail !== undefined ? settings.hideEmail : false;
      } catch {
        return false;
      }
    }
    return false;
  });
  const [hideBirthday, setHideBirthday] = useState(() => {
    const savedSettings = localStorage.getItem('userSettings');
    if (user?.hide_birthday !== undefined) {
      return user.hide_birthday;
    }
    if (savedSettings) {
      try {
        const settings = JSON.parse(savedSettings);
        return settings.hideBirthday !== undefined ? settings.hideBirthday : false;
      } catch {
        return false;
      }
    }
    return false;
  });
  const [hideBio, setHideBio] = useState(() => {
    const savedSettings = localStorage.getItem('userSettings');
    if (user?.hide_bio !== undefined) {
      return user.hide_bio;
    }
    if (savedSettings) {
      try {
        const settings = JSON.parse(savedSettings);
        return settings.hideBio !== undefined ? settings.hideBio : false;
      } catch {
        return false;
      }
    }
    return false;
  });
  
  const [success, setSuccess] = useState('');

  // Sync language with i18n when language changes
  useEffect(() => {
    setI18nLocale(language);
  }, [language, setI18nLocale]);

  // Apply theme when theme changes or on component mount
  useEffect(() => {
    document.documentElement.classList.remove('light', 'dark');
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else if (theme === 'light') {
      document.documentElement.classList.add('light');
    } else {
      // Follow the system to remove specific theme classes
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      if (prefersDark) {
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.classList.add('light');
      }
    }
  }, [theme]);

  const handleSave = async () => {
    setLoading(true);
    setError('');
    
    try {
      // 保存隐私设置到服务器
      const response = await authApi.updateSettings({
        profile_visibility: profileVisibility,
        hide_email: hideEmail,
        hide_birthday: hideBirthday,
        hide_bio: hideBio
      });
      
      // 更新本地用户信息
      if (response.data.user) {
        updateUser(response.data.user);
      }
      
      // 单独保存主题设置
      localStorage.setItem('theme', theme);
      
      // 保存隐私设置到 localStorage
      const settings = {
        profileVisibility,
        hideEmail,
        hideBirthday,
        hideBio
      };
      
      localStorage.setItem('userSettings', JSON.stringify(settings));
      
      // 单独保存语言设置
      localStorage.setItem('locale', language);
      
      setSuccess(t('settings.saveSuccess'));
      setTimeout(() => setSuccess(''), 3000);
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message : t('settings.saveFailed');
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const [error, setError] = useState('');

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="mb-8">
        <h1 className="text-3xl font-bold">{t('settings.title')}</h1>
        <p className="text-muted-foreground">{t('settings.displaySettings')} / {t('settings.privacySettings')}</p>
      </div>

      {success && (
        <Alert className="bg-green-50 border-green-200 mb-6">
          <Check className="w-4 h-4 text-green-600" />
          <AlertDescription className="text-green-600">{success}</AlertDescription>
        </Alert>
      )}
      {error && (
        <Alert variant="destructive" className="mb-6">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        {/* Display settings card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Palette className="w-5 h-5" />
              {t('settings.displaySettings')}
            </CardTitle>
            <CardDescription>{t('settings.displaySettings')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>{t('settings.theme')}</Label>
              <Select value={theme} onValueChange={(value: 'light' | 'dark' | 'system') => setTheme(value)}>
                <SelectTrigger>
                  <SelectValue placeholder={t('settings.theme')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="light">{t('settings.light')}</SelectItem>
                  <SelectItem value="dark">{t('settings.dark')}</SelectItem>
                  <SelectItem value="system">{t('settings.system')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            
            <div className="space-y-2">
              <Label>{t('settings.language')}</Label>
              <Select value={language} onValueChange={setLanguage}>
                <SelectTrigger>
                  <SelectValue placeholder={t('settings.language')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="zh-CN">简体中文</SelectItem>
                  <SelectItem value="en-US">English</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardContent>
        </Card>

        {/* Privacy settings card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="w-5 h-5" />
              {t('settings.privacySettings')}
            </CardTitle>
	          <CardDescription>{t('settings.privacySettingsDesc')}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>{t('settings.profileVisibility')}</Label>
              <Select value={profileVisibility} onValueChange={(value: 'public' | 'private') => setProfileVisibility(value)}>
                <SelectTrigger>
                  <SelectValue placeholder={t('settings.profileVisibility')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="public">{t('settings.public')}</SelectItem>
                  <SelectItem value="private">{t('settings.private')}</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-muted-foreground">
                {profileVisibility === 'public'
                  ? t('settings.publicDesc')
                  : t('settings.privateDesc')}
              </p>
            </div>
            
            <div className="space-y-4 pt-2">
              <h3 className="text-sm font-medium">{t('settings.personalInfoVisibility')}</h3>
              
              <div className="flex items-center justify-between">
                <div>
	                <Label>{t('settings.hideEmail')}</Label>
                  <p className="text-sm text-muted-foreground">{t('settings.hideEmailDesc')}</p>
                </div>
                <Switch
                  checked={hideEmail}
                  onCheckedChange={setHideEmail}
                />
              </div>
              
              <div className="flex items-center justify-between">
                <div>
	                <Label>{t('settings.hideBirthday')}</Label>
                  <p className="text-sm text-muted-foreground">{t('settings.hideBirthdayDesc')}</p>
                </div>
                <Switch
                  checked={hideBirthday}
                  onCheckedChange={setHideBirthday}
                />
              </div>
              
              <div className="flex items-center justify-between">
                <div>
                  <Label>{t('settings.hideBio')}</Label>
                  <p className="text-sm text-muted-foreground">{t('settings.hideBioDesc')}</p>
                </div>
                <Switch
                  checked={hideBio}
                  onCheckedChange={setHideBio}
                />
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="mt-8 flex justify-end">
        <Button onClick={handleSave} disabled={loading}>
          {loading ? (
            <>
              <Loader2 className="w-4 h-4 mr-2 animate-spin" />
              {t('common.saving')}
            </>
          ) : (
            t('settings.save')
          )}
        </Button>
      </div>
    </div>
  );
}