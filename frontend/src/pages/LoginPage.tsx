import { useEffect, useState } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/hooks/useAuth';
import { useTranslation } from '@/lib/I18nContext';
import { authApi, configApi } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { SliderCaptcha } from '@/components/ui/slider-captcha';
import { Loader2, Mail, Lock, Eye, EyeOff, ArrowLeft, CheckCircle } from 'lucide-react';

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { login } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [emailVerified, setEmailVerified] = useState<boolean | null>(null);
  const [captchaData, setCaptchaData] = useState<{ id: string; token: string; x: number } | null>(null);
  const [isCaptchaModalOpen, setIsCaptchaModalOpen] = useState(false);
  const [captchaEnabled, setCaptchaEnabled] = useState(false);
  const [isCaptchaVerified, setIsCaptchaVerified] = useState(false);

  // 获取重定向地址
  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/';

  useEffect(() => {
    const loadCaptchaSettings = async () => {
      try {
        const response = await configApi.getGeneralSettings();
        setCaptchaEnabled(response.data.captchaEnabled);
      } catch (error) {
        console.error(t('common.error'), error);
        // 如果加载失败，默认不启用滑块验证
        setCaptchaEnabled(false);
      }
    };
    loadCaptchaSettings();
  }, [t]);

  // 验证码验证成功回调
  const handleCaptchaSuccess = async (data: { id: string; token: string; x: number }) => {
    setCaptchaData(data);
    
    // 调用预验证接口，标记验证码为已使用
    try {
      await fetch('/api/captcha/verify', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          id: data.id,
          token: data.token,
          x: data.x,
        }),
      });
    } catch (error) {
      console.error('预验证失败:', error);
      // 即使预验证失败，也保存数据，让用户尝试登录
    }
    
    setIsCaptchaVerified(true);
    // 不自动执行登录，等待用户点击登录按钮
  };

  // 重置验证码状态
  const resetCaptcha = () => {
    setCaptchaData(null);
    setIsCaptchaVerified(false);
  };

  const performLogin = async () => {
    setLoading(true);

    try {
      // 如果启用了滑块验证，传递验证码数据
      if (captchaEnabled) {
        if (!captchaData) {
          setError(t('loginPage.completeCaptcha'));
          setIsCaptchaModalOpen(true);
          return;
        }
        await login(email, password, captchaData);
      } else {
        // 传递空的验证码数据
        await login(email, password);
      }
      // 登录成功后关闭验证码弹窗并重置状态
      setIsCaptchaModalOpen(false);
      resetCaptcha();
      navigate(from, { replace: true });
    } catch (err) {
      const error = err as { response?: { data?: { message?: string; email_verified?: boolean } }; message?: string };
      const errorMessage = error.response?.data?.message || error.message || '登录失败，请检查邮箱和密码';
      setError(errorMessage);
      
      // 如果是因为邮箱未验证，保存邮箱状态以便显示重新发送链接
      if (error.response?.data?.email_verified === false) {
        setEmailVerified(false);
      }
      
      // 登录失败后不重置验证码状态，允许用户重试登录
    } finally {
      setLoading(false);
    }
  };





  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    
    // 表单验证
    if (!email || !password) {
      setError(t('loginPage.fillRequired'));
      return;
    }
    
    // 如果启用了滑块验证，检查是否已完成验证
    if (captchaEnabled) {
      if (!isCaptchaVerified || !captchaData) {
        setError(t('loginPage.completeCaptcha'));
        setIsCaptchaModalOpen(true);
        return;
      }
    }
    
    // 执行登录
    performLogin();
  };

  const handleResendVerification = async () => {
    if (!email) {
      setError(t('auth.email') + ' ' + t('common.required'));
      return;
    }
    
    setLoading(true);
    try {
      await authApi.resendVerificationEmail();
      setError('');
      alert(t('loginPage.verificationSent'));
    } catch (err) {
      const error = err as { response?: { data?: { message?: string } } };
      setError(error.response?.data?.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-16 flex justify-center">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">{t('loginPage.welcomeBack')}</CardTitle>
          <CardDescription>
            {t('loginPage.loginVexgo')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert
                variant={emailVerified === false ? "default" : "destructive"}
                className="mb-4"
              >
                <AlertDescription className="space-y-2">
                  <p className="font-medium">{error}</p>
                  {emailVerified === false && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={handleResendVerification}
                      disabled={loading}
                      className="mt-2"
                    >
                      <ArrowLeft className="w-3 h-3 mr-2" />
                      {t('loginPage.resendVerification')}
                    </Button>
                  )}
                </AlertDescription>
              </Alert>
            )}

            <div className="space-y-2">
              <Label htmlFor="email">{t('auth.email')}</Label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="email"
                  type="email"
                  placeholder="your@email.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="pl-10"
                  required
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">{t('auth.password')}</Label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="password"
                  type={showPassword ? 'text' : 'password'}
                  placeholder={t('auth.password')}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="pl-10 pr-10"
                  required
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            {/* 滑块验证区域 */}
            {captchaEnabled && (
              <div className="mt-4">
                <div className="flex items-center justify-between mb-2">
                  <Label className="text-sm font-medium">{t('loginPage.securityVerification')}</Label>
                  {isCaptchaVerified && (
                    <span className="text-xs text-green-600 flex items-center">
                      <CheckCircle className="h-3 w-3 mr-1" />
                      {t('loginPage.verifiedBadge')}
                    </span>
                  )}
                </div>
                <div className="border rounded-lg p-3 bg-gray-50">
                  {isCaptchaVerified ? (
                    <div className="flex items-center justify-between">
                      <div className="flex items-center text-sm text-green-600">
                        <CheckCircle className="h-4 w-4 mr-2" />
                        {t('loginPage.captchaCompleted')}
                      </div>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={resetCaptcha}
                        className="text-xs h-7"
                      >
                        {t('auth.resetPassword')}
                      </Button>
                    </div>
                  ) : (
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-600">{t('loginPage.completeSlider')}</span>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setIsCaptchaModalOpen(true)}
                        className="text-xs h-7"
                      >
                        {t('loginPage.verifyButton')}
                      </Button>
                    </div>
                  )}
                </div>
              </div>
            )}

            <Button
              type="submit"
              className="w-full mt-4"
              disabled={loading || (captchaEnabled && !isCaptchaVerified)}
            >
              {loading ? (
                <>
                  <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  {t('loginPage.loggingIn')}
                </>
              ) : (
                t('loginPage.loginButton')
              )}
            </Button>
          </form>

          <div className="mt-4 text-center text-sm">
            <button
              type="button"
              onClick={() => navigate('/reset-password')}
              className="text-primary hover:underline focus:outline-none"
            >
              {t('loginPage.forgotPassword')}
            </button>
          </div>

          <div className="mt-6 text-center text-sm">
            <span className="text-muted-foreground">{t('loginPage.noAccount')}</span>{' '}
            <Link to="/register" className="text-primary hover:underline">
              {t('loginPage.registerNow')}
            </Link>
          </div>
        </CardContent>
      </Card>

      {/* 验证码弹窗 */}
      <SliderCaptcha
        isOpen={isCaptchaModalOpen}
        onClose={() => {
          // 允许用户随时关闭验证窗口
          setIsCaptchaModalOpen(false);
        }}
        onSuccess={handleCaptchaSuccess}
      />
    </div>
  );
}