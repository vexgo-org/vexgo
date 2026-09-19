import { useEffect, useState } from "react";
import { Link, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { SliderCaptcha } from "@/components/ui/slider-captcha";
import { Mail, Lock, Eye, EyeOff, CheckCircle } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { useSSOProviders, type SSOProvider } from "@/hooks/useSSOProviders";

// ── SSO helpers ──────────────────────────────────────────────────────────────

/**
 * Opens a small popup window for the given SSO provider and returns a Promise
 * that resolves with the JWT token (on success) or rejects with an error message.
 */
// SSO callback result is written to localStorage by the popup, then picked up
// here via the 'storage' event. This avoids postMessage cross-origin issues
// caused by the IdP redirect clearing window.opener.
const SSO_STORAGE_KEY = "sso_callback_result";

function ssoLogin(provider: SSOProvider): Promise<string> {
  return new Promise((resolve, reject) => {
    // Clear any stale result from a previous attempt
    localStorage.removeItem(SSO_STORAGE_KEY);

    const url = `/api/sso/${provider}/login?method=sso_get_token`;
    const popup = window.open(
      url,
      "sso_login",
      "width=600,height=700,resizable=yes",
    );

    if (!popup) {
      reject(new Error("Popup blocked. Please allow popups for this site."));
      return;
    }

    // Listen for the callback result written by the popup page
    const onStorage = (event: StorageEvent) => {
      if (event.key !== SSO_STORAGE_KEY || !event.newValue) return;
      cleanup();
      const data = JSON.parse(event.newValue) as {
        token?: string;
        error?: string;
      };
      localStorage.removeItem(SSO_STORAGE_KEY);
      if (data.token) {
        resolve(data.token);
      } else {
        reject(new Error(data.error || "SSO login failed"));
      }
    };

    // Fallback: popup closed without writing result
    const pollClosed = setInterval(() => {
      if (popup.closed) {
        // Give a short grace period for the storage event to fire
        setTimeout(() => {
          const stored = localStorage.getItem(SSO_STORAGE_KEY);
          cleanup();
          if (stored) {
            const data = JSON.parse(stored) as {
              token?: string;
              error?: string;
            };
            localStorage.removeItem(SSO_STORAGE_KEY);
            if (data.token) resolve(data.token);
            else reject(new Error(data.error || "SSO login failed"));
          } else {
            reject(new Error("Login window was closed"));
          }
        }, 300);
      }
    }, 500);

    function cleanup() {
      window.removeEventListener("storage", onStorage);
      clearInterval(pollClosed);
    }

    window.addEventListener("storage", onStorage);
  });
}

// ── Provider button config ───────────────────────────────────────────────────

interface ProviderConfig {
  id: SSOProvider;
  label: string;
  icon: React.ReactNode;
}

const GitHubIcon = () => (
  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
    <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
  </svg>
);

const GoogleIcon = () => (
  <svg className="w-4 h-4" viewBox="0 0 24 24">
    <path
      fill="#4285F4"
      d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
    />
    <path
      fill="#34A853"
      d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
    />
    <path
      fill="#FBBC05"
      d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
    />
    <path
      fill="#EA4335"
      d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
    />
  </svg>
);

const OIDCIcon = () => (
  <svg
    className="w-4 h-4"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
  >
    <circle cx="12" cy="12" r="10" />
    <path d="M12 8v4l3 3" />
  </svg>
);

// ── Main component ───────────────────────────────────────────────────────────

const ALL_PROVIDERS: ProviderConfig[] = [
  {
    id: "github",
    label: "GitHub",
    icon: <GitHubIcon />,
  },
  {
    id: "google",
    label: "Google",
    icon: <GoogleIcon />,
  },
  {
    id: "oidc",
    label: "SSO",
    icon: <OIDCIcon />,
  },
];

export function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const { login, loginWithToken } = useAuth(); // loginWithToken: accepts a raw JWT string
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [ssoLoading, setSSOLoading] = useState<SSOProvider | null>(null);
  const {
    providers: enabledProviders,
    allowLocalLogin,
    loading: ssoConfigLoading,
  } = useSSOProviders();
  const [error, setError] = useState("");
  const [registrationMessage, setRegistrationMessage] = useState("");
  const [resendMessage, setResendMessage] = useState("");
  const [resendLoading, setResendLoading] = useState(false);
  const [emailVerified, setEmailVerified] = useState<boolean | null>(null);
  const [captchaData, setCaptchaData] = useState<{
    id: string;
    token: string;
    x: number;
    y: number;
  } | null>(null);
  const [isCaptchaModalOpen, setIsCaptchaModalOpen] = useState(false);
  const [captchaEnabled, setCaptchaEnabled] = useState(false);
  const [isCaptchaVerified, setIsCaptchaVerified] = useState(false);

  const locationState = location.state as
    { from?: { pathname: string }; registrationMessage?: string } | undefined;
  const from = locationState?.from?.pathname || "/admin";

  useEffect(() => {
    if (locationState?.registrationMessage) {
      setRegistrationMessage(locationState.registrationMessage);
      navigate(location.pathname, {
        replace: true,
        state: locationState.from ? { from: locationState.from } : undefined,
      });
    }
  }, [location.pathname, locationState, navigate]);

  useEffect(() => {
    const loadCaptchaSettings = async () => {
      try {
        const response = await unwrap(getVexGoAPI().getConfigGeneral());
        setCaptchaEnabled(response.captchaEnabled ?? false);
      } catch (error) {
        console.error(t("common.error"), error);
        setCaptchaEnabled(false);
      }
    };
    loadCaptchaSettings();
  }, [t]);

  const handleCaptchaSuccess = async (data: {
    id: string;
    token: string;
    x: number;
    y: number;
  }) => {
    // The captcha dialog already pre-verified this challenge on drop; keep
    // the data so the login submit re-checks it server-side.
    setCaptchaData(data);
    setIsCaptchaVerified(true);
  };

  const resetCaptcha = () => {
    setCaptchaData(null);
    setIsCaptchaVerified(false);
  };

  const performLogin = async () => {
    setLoading(true);
    try {
      if (captchaEnabled) {
        if (!captchaData) {
          setError(t("loginPage.completeCaptcha"));
          setIsCaptchaModalOpen(true);
          return;
        }
        await login(email, password, captchaData);
      } else {
        await login(email, password);
      }
      setIsCaptchaModalOpen(false);
      resetCaptcha();
      navigate(from, { replace: true });
    } catch (err) {
      const error = err as {
        response?: { data?: { message?: string; email_verified?: boolean } };
        message?: string;
      };
      setError(
        error.response?.data?.message ||
          error.message ||
          t("loginPage.loginFailed"),
      );
      if (error.response?.data?.email_verified === false)
        setEmailVerified(false);
      if (captchaEnabled) {
        resetCaptcha();
      }
    } finally {
      setLoading(false);
    }
  };

  const handleResendVerification = async () => {
    if (!email) {
      setError(t("loginPage.enterEmailForVerification"));
      return;
    }
    setResendLoading(true);
    setResendMessage("");
    setError("");
    try {
      const response = await unwrap(
        getVexGoAPI().postAuthEmailVerifyResend({ email }),
      );
      setResendMessage(response.message ?? "");
    } catch (err) {
      const error = err as { response?: { data?: { error?: string } } };
      setError(
        error.response?.data?.error || t("loginPage.resendVerificationFailed"),
      );
    } finally {
      setResendLoading(false);
    }
  };

  const handleSubmit = async (e: React.SubmitEvent) => {
    e.preventDefault();
    setError("");
    if (!email || !password) {
      setError(t("loginPage.fillRequired"));
      return;
    }
    if (captchaEnabled && (!isCaptchaVerified || !captchaData)) {
      setError(t("loginPage.completeCaptcha"));
      setIsCaptchaModalOpen(true);
      return;
    }
    performLogin();
  };

  // ── SSO login handler ──────────────────────────────────────────────────────

  const handleSSOLogin = async (provider: SSOProvider) => {
    setError("");
    setSSOLoading(provider);
    try {
      const token = await ssoLogin(provider);
      // Hand the JWT straight to the auth context so the app treats it like a
      // normal login (stores it, updates user state, etc.)
      await loginWithToken(token);
      navigate(from, { replace: true });
    } catch (err) {
      const raw = err instanceof Error ? err.message : "";
      const message =
        raw === "Popup blocked. Please allow popups for this site."
          ? t("loginPage.popupBlocked")
          : raw || t("loginPage.ssoLoginFailed");
      setError(message);
    } finally {
      setSSOLoading(null);
    }
  };

  return (
    <>
      <Card>
        <CardHeader className="text-center">
          <CardTitle className="text-lg">
            {t("loginPage.welcomeBack")}
          </CardTitle>
          <CardDescription>{t("loginPage.loginVexgo")}</CardDescription>
        </CardHeader>
        <CardContent>
          {allowLocalLogin && (
            <form onSubmit={handleSubmit} className="space-y-4">
              {registrationMessage && (
                <Alert variant="success">
                  <CheckCircle />
                  <AlertDescription className="text-current">
                    {registrationMessage}
                  </AlertDescription>
                </Alert>
              )}

              {resendMessage && (
                <Alert variant="success">
                  <CheckCircle />
                  <AlertDescription className="text-current">
                    {resendMessage}
                  </AlertDescription>
                </Alert>
              )}

              {error && (
                <Alert
                  variant={emailVerified === false ? "warning" : "destructive"}
                >
                  <AlertDescription className="space-y-2">
                    <p className="font-medium">{error}</p>
                    {emailVerified === false && (
                      <Button
                        type="button"
                        variant="link"
                        className="h-auto p-0 font-medium"
                        onClick={handleResendVerification}
                        disabled={resendLoading}
                      >
                        {resendLoading
                          ? t("loginPage.resendingVerification")
                          : t("loginPage.resendVerification")}
                      </Button>
                    )}
                  </AlertDescription>
                </Alert>
              )}

              <div className="space-y-2">
                <Label htmlFor="email">{t("auth.email")}</Label>
                <div className="relative">
                  <Mail className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                  <Input
                    id="email"
                    type="email"
                    placeholder="your@email.com"
                    autoComplete="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    className="pl-9"
                    required
                  />
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="password">{t("auth.password")}</Label>
                <div className="relative">
                  <Lock className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                  <Input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    placeholder={t("auth.password")}
                    autoComplete="current-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    className="pr-9 pl-9"
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    aria-label={
                      showPassword
                        ? t("loginPage.hidePassword")
                        : t("loginPage.showPassword")
                    }
                    className="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2 transition-colors"
                  >
                    {showPassword ? (
                      <EyeOff className="size-4" />
                    ) : (
                      <Eye className="size-4" />
                    )}
                  </button>
                </div>
              </div>

              {captchaEnabled && (
                <div className="mt-4">
                  <div className="flex items-center justify-between mb-2">
                    <Label className="text-sm font-medium">
                      {t("loginPage.securityVerification")}
                    </Label>
                    {isCaptchaVerified && (
                      <span className="text-success flex items-center gap-1 text-[11px]">
                        <CheckCircle className="size-3" />
                        {t("loginPage.verifiedBadge")}
                      </span>
                    )}
                  </div>
                  <div className="bg-muted/60 rounded-md border p-3">
                    {isCaptchaVerified ? (
                      <div className="flex items-center justify-between gap-3">
                        <div className="text-success flex items-center gap-2 text-[13px]">
                          <CheckCircle className="size-4" />
                          {t("loginPage.captchaCompleted")}
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={resetCaptcha}
                        >
                          {t("auth.reverify")}
                        </Button>
                      </div>
                    ) : (
                      <div className="flex items-center justify-between gap-3">
                        <span className="text-muted-foreground text-[13px]">
                          {t("loginPage.completeSlider")}
                        </span>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => setIsCaptchaModalOpen(true)}
                        >
                          {t("loginPage.verifyButton")}
                        </Button>
                      </div>
                    )}
                  </div>
                </div>
              )}

              <Button
                type="submit"
                className="mt-2 w-full"
                disabled={loading || (captchaEnabled && !isCaptchaVerified)}
              >
                {loading ? (
                  <>
                    <Spinner className="size-4" />
                    {t("loginPage.loggingIn")}
                  </>
                ) : (
                  t("loginPage.loginButton")
                )}
              </Button>
            </form>
          )}

          {/* ── SSO / third-party login ── */}
          {!ssoConfigLoading && enabledProviders.length > 0 && (
            <div className="mt-6">
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-[11px]">
                  <span className="bg-card text-muted-foreground px-2">
                    {t("loginPage.orContinueWith") ?? "Or continue with"}
                  </span>
                </div>
              </div>

              <div className="mt-4 grid grid-cols-3 gap-2">
                {ALL_PROVIDERS.filter((p) =>
                  enabledProviders.includes(p.id),
                ).map((provider) => (
                  <Button
                    key={provider.id}
                    type="button"
                    variant="outline"
                    disabled={ssoLoading !== null}
                    onClick={() => handleSSOLogin(provider.id)}
                  >
                    {ssoLoading === provider.id ? (
                      <Spinner className="size-4" />
                    ) : (
                      provider.icon
                    )}
                    <span>{provider.label}</span>
                  </Button>
                ))}
              </div>
            </div>
          )}

          <div className="mt-4 text-center">
            <button
              type="button"
              onClick={() => navigate("/admin/reset-password")}
              className="text-accent-blue focus-visible:ring-ring/25 rounded-sm text-[13px] underline-offset-4 outline-none hover:underline focus-visible:ring-[3px]"
            >
              {t("loginPage.forgotPassword")}
            </button>
          </div>

          <div className="text-muted-foreground mt-5 text-center text-[13px]">
            {t("loginPage.noAccount")}{" "}
            <Link
              to="/admin/register"
              className="text-accent-blue rounded-sm underline-offset-4 hover:underline"
            >
              {t("loginPage.registerNow")}
            </Link>
          </div>
        </CardContent>
      </Card>

      <SliderCaptcha
        isOpen={isCaptchaModalOpen}
        onClose={() => setIsCaptchaModalOpen(false)}
        onSuccess={handleCaptchaSuccess}
      />
    </>
  );
}
