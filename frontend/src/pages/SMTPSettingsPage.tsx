import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { configApi } from '@/lib/api';
import type { SMTPConfig } from '@/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { ArrowLeft, Mail, Save, TestTube } from 'lucide-react';
import { toast } from 'sonner';

export function SMTPSettingsPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [config, setConfig] = useState<SMTPConfig>({
    id: '',
    enabled: false,
    host: '',
    port: 587,
    username: '',
    password: '',
    fromEmail: '',
    fromName: 'Blog System',
    testEmail: '',
    createdAt: '',
    updatedAt: ''
  });

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response = await configApi.getSMTPConfig();
      setConfig(response.data);
    } catch (error: any) {
      console.error('加载 SMTP 配置失败:', error);
      toast.error('加载配置失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!config.host.trim()) {
      toast.error('请输入 SMTP 服务器地址');
      return;
    }
    if (config.port <= 0 || config.port > 65535) {
      toast.error('请输入有效的端口号');
      return;
    }
    if (!config.username.trim()) {
      toast.error('请输入邮箱账号');
      return;
    }
    if (config.enabled && !config.password.trim()) {
      toast.error('启用 SMTP 时需要填写密码');
      return;
    }
    if (!config.fromEmail.trim()) {
      toast.error('请输入发件人邮箱');
      return;
    }

    setSaving(true);
    try {
      await configApi.updateSMTPConfig(config);
      toast.success('SMTP 配置已保存');
    } catch (error: any) {
      console.error('保存 SMTP 配置失败:', error);
      toast.error('保存失败: ' + (error.response?.data?.error || '未知错误'));
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!config.enabled) {
      toast.error('请先启用 SMTP');
      return;
    }
    if (!config.password.trim()) {
      toast.error('请先保存密码');
      return;
    }

    setTesting(true);
    try {
      const response = await configApi.testSMTP();
      toast.success(response.data.message);
    } catch (error: any) {
      console.error('测试邮件失败:', error);
      toast.error('测试失败: ' + (error.response?.data?.error || '未知错误'));
    } finally {
      setTesting(false);
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <div className="mb-6">
          <div className="h-8 w-48 bg-muted rounded animate-pulse mb-2" />
          <div className="h-4 w-64 bg-muted rounded animate-pulse" />
        </div>
        <Card>
          <CardContent className="p-6 space-y-4">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="h-10 bg-muted rounded animate-pulse" />
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* 头部 */}
      <div className="mb-6">
        <Button 
          variant="ghost" 
          size="sm" 
          onClick={() => navigate('/admin')}
          className="mb-4"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          返回管理后台
        </Button>
        <h1 className="text-3xl font-bold flex items-center gap-2">
          <Mail className="w-8 h-8" />
          SMTP 邮件设置
        </h1>
        <p className="text-muted-foreground mt-2">
          配置邮件服务器，用于注册验证等邮件发送功能
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>邮件服务器配置</CardTitle>
          <CardDescription>
            填写您的 SMTP 服务器信息以启用邮件发送功能
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* 启用开关 */}
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="enabled">启用 SMTP</Label>
              <p className="text-sm text-muted-foreground">
                开启后系统将使用 SMTP 发送邮件
              </p>
            </div>
            <Switch
              id="enabled"
              checked={config.enabled}
              onCheckedChange={(checked) => setConfig({ ...config, enabled: checked })}
            />
          </div>

          {/* SMTP 服务器地址 */}
          <div className="space-y-2">
            <Label htmlFor="host">SMTP 服务器地址 *</Label>
            <Input
              id="host"
              value={config.host}
              onChange={(e) => setConfig({ ...config, host: e.target.value })}
              placeholder="例如: smtp.gmail.com 或 smtp.qq.com"
              disabled={saving}
            />
          </div>

          {/* 端口 */}
          <div className="space-y-2">
            <Label htmlFor="port">端口 *</Label>
            <Input
              id="port"
              type="number"
              value={config.port}
              onChange={(e) => setConfig({ ...config, port: parseInt(e.target.value) || 587 })}
              placeholder="587"
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              常用端口: 587 (TLS), 465 (SSL), 25 (非加密)
            </p>
          </div>

          {/* 邮箱账号 */}
          <div className="space-y-2">
            <Label htmlFor="username">邮箱账号 *</Label>
            <Input
              id="username"
              value={config.username}
              onChange={(e) => setConfig({ ...config, username: e.target.value })}
              placeholder="your-email@example.com"
              disabled={saving}
            />
          </div>

          {/* 邮箱密码/授权码 */}
          <div className="space-y-2">
            <Label htmlFor="password">
              邮箱密码或授权码 {config.enabled && <span className="text-red-500">*</span>}
            </Label>
            <Input
              id="password"
              type="password"
              value={config.password}
              onChange={(e) => setConfig({ ...config, password: e.target.value })}
              placeholder={config.enabled ? "请输入密码或授权码" : "如需修改密码请填写"}
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              注意: 部分邮箱服务需要使用授权码而非登录密码
            </p>
          </div>

          {/* 发件人邮箱 */}
          <div className="space-y-2">
            <Label htmlFor="fromEmail">发件人邮箱 *</Label>
            <Input
              id="fromEmail"
              type="email"
              value={config.fromEmail}
              onChange={(e) => setConfig({ ...config, fromEmail: e.target.value })}
              placeholder="noreply@yourblog.com"
              disabled={saving}
            />
          </div>

          {/* 发件人名称 */}
          <div className="space-y-2">
            <Label htmlFor="fromName">发件人名称</Label>
            <Input
              id="fromName"
              value={config.fromName}
              onChange={(e) => setConfig({ ...config, fromName: e.target.value })}
              placeholder="Blog System"
              disabled={saving}
            />
          </div>

          {/* 测试邮箱 */}
          <div className="space-y-2">
            <Label htmlFor="testEmail">测试邮件收件人</Label>
            <Input
              id="testEmail"
              type="email"
              value={config.testEmail}
              onChange={(e) => setConfig({ ...config, testEmail: e.target.value })}
              placeholder="输入测试邮箱地址"
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              用于发送测试邮件，验证 SMTP 配置是否正确
            </p>
          </div>

          {/* 操作按钮 */}
          <div className="flex gap-3 pt-4 border-t">
            <Button 
              onClick={handleSave} 
              disabled={saving}
              className="flex-1"
            >
              <Save className="w-4 h-4 mr-2" />
              {saving ? '保存中...' : '保存配置'}
            </Button>
            <Button 
              variant="outline" 
              onClick={handleTest}
              disabled={testing || !config.enabled}
              className="flex-1"
            >
              <TestTube className="w-4 h-4 mr-2" />
              {testing ? '测试中...' : '发送测试邮件'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 帮助信息 */}
      <Card className="mt-6">
        <CardHeader>
          <CardTitle className="text-base">常见邮箱服务配置示例</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <div>
            <strong>Gmail:</strong> smtp.gmail.com:587 (TLS) 或 465 (SSL)
          </div>
          <div>
            <strong>QQ 邮箱:</strong> smtp.qq.com:587 (TLS) 或 465 (SSL)，需使用授权码
          </div>
          <div>
            <strong>163 邮箱:</strong> smtp.163.com:465 (SSL)，需使用授权码
          </div>
          <div>
            <strong>Outlook/Hotmail:</strong> smtp.office365.com:587 (TLS)
          </div>
        </CardContent>
      </Card>
    </div>
  );
}