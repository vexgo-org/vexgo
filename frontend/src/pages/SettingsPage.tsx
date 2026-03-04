import { useState } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Check, Bell, Palette, Globe, Shield } from 'lucide-react';

export function SettingsPage() {
  const { user } = useAuth();
  
  // Retrieve saved settings from localStorage or use default values
  const getSavedSetting = <T,>(key: string, defaultValue: T): T => {
    const savedSettings = localStorage.getItem('userSettings');
    if (savedSettings) {
      try {
        const settings = JSON.parse(savedSettings);
        return settings[key] !== undefined ? settings[key] : defaultValue;
      } catch {
        return defaultValue;
      }
    }
    return defaultValue;
  };

  // Notification settings
  const [emailNotifications, setEmailNotifications] = useState(() =>
    getSavedSetting('emailNotifications', true)
  );
  const [pushNotifications, setPushNotifications] = useState(() =>
    getSavedSetting('pushNotifications', false)
  );
  
  // Display settings
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>(() =>
    getSavedSetting('theme', 'light')
  );
  const [language, setLanguage] = useState(() =>
    getSavedSetting('language', 'zh-CN')
  );
  
  // Privacy settings
  const [profileVisibility, setProfileVisibility] = useState<'public' | 'private'>(() =>
    getSavedSetting('profileVisibility', 'public')
  );
  
  const [success, setSuccess] = useState('');

  const handleSave = () => {
    // Save settings to localStorage
    const settings = {
      emailNotifications,
      pushNotifications,
      theme,
      language,
      profileVisibility
    };
    
    localStorage.setItem('userSettings', JSON.stringify(settings));
    
    // Apply theme
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
    
    setSuccess('设置已保存');
    setTimeout(() => setSuccess(''), 3000);
  };

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      <div className="mb-8">
        <h1 className="text-3xl font-bold">设置</h1>
        <p className="text-muted-foreground">管理您的账户设置和偏好</p>
      </div>

      {success && (
        <Alert className="bg-green-50 border-green-200 mb-6">
          <Check className="w-4 h-4 text-green-600" />
          <AlertDescription className="text-green-600">{success}</AlertDescription>
        </Alert>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        {/* Notification settings cards */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bell className="w-5 h-5" />
              通知设置
            </CardTitle>
            <CardDescription>选择您希望接收的通知类型</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <Label>邮件通知</Label>
                <p className="text-sm text-muted-foreground">通过电子邮件接收重要更新</p>
              </div>
              <Switch
                checked={emailNotifications}
                onCheckedChange={setEmailNotifications}
              />
            </div>
            
            <div className="flex items-center justify-between">
              <div>
                <Label>推送通知</Label>
                <p className="text-sm text-muted-foreground">在设备上接收推送通知</p>
              </div>
              <Switch
                checked={pushNotifications}
                onCheckedChange={setPushNotifications}
              />
            </div>
          </CardContent>
        </Card>

        {/* Display settings card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Palette className="w-5 h-5" />
              显示设置
            </CardTitle>
            <CardDescription>自定义界面外观和语言</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>主题</Label>
              <Select value={theme} onValueChange={(value: 'light' | 'dark' | 'system') => setTheme(value)}>
                <SelectTrigger>
                  <SelectValue placeholder="选择主题" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="light">浅色</SelectItem>
                  <SelectItem value="dark">深色</SelectItem>
                  <SelectItem value="system">跟随系统</SelectItem>
                </SelectContent>
              </Select>
            </div>
            
            <div className="space-y-2">
              <Label>语言</Label>
              <Select value={language} onValueChange={setLanguage}>
                <SelectTrigger>
                  <SelectValue placeholder="选择语言" />
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
              隐私设置
            </CardTitle>
            <CardDescription>控制谁可以看到您的信息</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>个人资料可见性</Label>
              <Select value={profileVisibility} onValueChange={(value: 'public' | 'private') => setProfileVisibility(value)}>
                <SelectTrigger>
                  <SelectValue placeholder="选择可见性" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="public">公开</SelectItem>
                  <SelectItem value="private">仅自己可见</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-sm text-muted-foreground">
                {profileVisibility === 'public' 
                  ? '您的个人资料对所有人可见' 
                  : '只有您自己可以看到您的个人资料'}
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Account Information Card */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Globe className="w-5 h-5" />
              账户信息
            </CardTitle>
            <CardDescription>您的账户基本信息</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>用户名</Label>
              <div className="p-2 bg-muted rounded-md">{user?.username}</div>
            </div>
            
            <div className="space-y-2">
              <Label>邮箱地址</Label>
              <div className="p-2 bg-muted rounded-md">{user?.email}</div>
            </div>
            
            <div className="space-y-2">
              <Label>账户角色</Label>
              <div className="p-2 bg-muted rounded-md">
                {user?.role === 'admin' ? '管理员' : '普通用户'}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="mt-8 flex justify-end">
        <Button onClick={handleSave}>保存设置</Button>
      </div>
    </div>
  );
}