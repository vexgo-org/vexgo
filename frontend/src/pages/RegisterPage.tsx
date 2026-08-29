import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { configApi } from "@/lib/api";
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
import { SliderCaptcha } from "@/components/ui/slider-captcha";
import {
  Loader2,
  Mail,
  Lock,
  User,
  Eye,
  EyeOff,
  CheckCircle,
} from "lucide-react";
import { toast } from "sonner";
import { useSSOProviders, type SSOProvider } from "@/hooks/useSSOProviders";

// ── SSO helpers (shared with LoginPage) ─────────────────────────────────────

const SSO_STORAGE_KEY = "sso_callback_result";

function ssoLogin(provider: SSOProvider): Promise<string> {
  return new Promise((resolve, reject) => {
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
    const onStorage = (event: StorageEvent) => {
      if (event.key !== SSO_STORAGE_KEY || !event.newValue) return;
      cleanup();
      const data = JSON.parse(event.newValue) as {
        token?: string;
        error?: string;
      };
      localStorage.removeItem(SSO_STORAGE_KEY);
      if (data.token) resolve(data.token);
      else reject(new Error(data.error || "SSO login failed"));
    };
    const pollClosed = setInterval(() => {
      if (popup.closed) {
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

interface ProviderConfig {
  id: SSOProvider;
  label: string;
  icon: React.ReactNode;
  className: string;
}

const ALL_PROVIDERS: ProviderConfig[] = [
  {
    id: "github",
    label: "GitHub",
    icon: <GitHubIcon />,
    className: "border-gray-300 hover:bg-gray-50 text-gray-700",
  },
  {
    id: "google",
    label: "Google",
    icon: <GoogleIcon />,
    className: "border-gray-300 hover:bg-gray-50 text-gray-700",
  },
  {
    id: "oidc",
    label: "SSO",
    icon: <OIDCIcon />,
    className: "border-gray-300 hover:bg-gray-50 text-gray-700",
  },
];

export function RegisterPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { register, loginWithToken } = useAuth();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [usernameError, setUsernameError] = useState("");
  const [emailError, setEmailError] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [confirmPasswordError, setConfirmPasswordError] = useState("");
  const [captchaData, setCaptchaData] = useState<{
    id: string;
    token: string;
    x: number;
    y: number;
  } | null>(null);
  const [isCaptchaModalOpen, setIsCaptchaModalOpen] = useState(false);
  const [captchaEnabled, setCaptchaEnabled] = useState(false);
  const [isCaptchaVerified, setIsCaptchaVerified] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(true);
  const { providers: enabledProviders, loading: ssoConfigLoading } =
    useSSOProviders();
  const [ssoLoading, setSSOLoading] = useState<SSOProvider | null>(null);

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      const response = await configApi.getGeneralSettings();
      setCaptchaEnabled(response.data.captchaEnabled);
      setRegistrationEnabled(response.data.registrationEnabled);
    } catch (error) {
      console.error("Failed to load settings:", error);
    }
  };

  // Callback when captcha verification succeeds
  const handleCaptchaSuccess = async (data: {
    id: string;
    token: string;
    x: number;
    y: number;
  }) => {
    // The captcha dialog already pre-verified this challenge on drop; keep
    // the data so the register submit re-checks it server-side.
    setCaptchaData(data);
    setIsCaptchaVerified(true);
    // Do not auto-register; wait for the user to click the register button
  };

  // Reset the captcha state
  const resetCaptcha = () => {
    setCaptchaData(null);
    setIsCaptchaVerified(false);
  };

  const performRegister = async () => {
    setLoading(true);
    try {
      if (captchaEnabled) {
        if (!captchaData) {
          toast.error(t("registerPage.completeCaptcha"));
          setIsCaptchaModalOpen(true);
          return;
        }
        await register(username, email, password, captchaData);
      } else {
        await register(username, email, password);
      }
      setIsCaptchaModalOpen(false);
      resetCaptcha();
      navigate("/");
    } catch (err) {
      // Registration succeeded but the account needs email verification: tell
      // the user to check their inbox and send them to the login page.
      if ((err as { requiresVerification?: boolean }).requiresVerification) {
        const registrationMessage = (err as { registrationMessage?: string })
          .registrationMessage;
        navigate("/login", {
          state: {
            registrationMessage: registrationMessage || t("register.success"),
          },
          replace: true,
        });
        return;
      }
      // Backend errors such as email verification can still be shown in a dialog
      const error = err as {
        response?: { data?: { message?: string } };
        message?: string;
      };
      let errorMessage = error.response?.data?.message || error.message || "";

      // Convert English error messages from the backend into the current locale
      if (errorMessage === "Email already registered") {
        errorMessage = t("errors.emailAlreadyUsed");
      } else if (errorMessage === "Username already registered") {
        errorMessage = t("errors.usernameAlreadyUsed");
      } else if (errorMessage === "Please verify your email address first") {
        errorMessage = t("verifyEmail.emailVerificationSuccess");
      }

      toast.error(errorMessage || t("registerPage.registrationError"));
      // Keep the captcha state after a failed registration so the user can retry
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setUsernameError("");
    setEmailError("");
    setPasswordError("");
    setConfirmPasswordError("");
    let hasError = false;
    if (username.length < 3) {
      setUsernameError(t("registerPage.usernameMin"));
      hasError = true;
    }
    if (!email || !/^\S+@\S+\.\S+$/.test(email)) {
      setEmailError(t("registerPage.emailInvalid"));
      hasError = true;
    }
    if (password.length < 6) {
      setPasswordError(t("registerPage.passwordMin"));
      hasError = true;
    }
    if (password !== confirmPassword) {
      setConfirmPasswordError(t("registerPage.passwordMismatch"));
      hasError = true;
    }
    if (hasError) return;

    // Check whether registration is allowed
    if (!registrationEnabled) {
      toast.error(t("registerPage.registrationDisabled"));
      return;
    }

    // If the slider captcha is enabled, check whether verification is done
    if (captchaEnabled) {
      if (!isCaptchaVerified || !captchaData) {
        toast.error(t("registerPage.completeCaptcha"));
        setIsCaptchaModalOpen(true);
        return;
      }
    }

    performRegister();
  };

  const handleSSOLogin = async (provider: SSOProvider) => {
    setSSOLoading(provider);
    try {
      const token = await ssoLogin(provider);
      await loginWithToken(token);
      navigate("/");
    } catch (err) {
      const message = err instanceof Error ? err.message : "SSO login failed";
      toast.error(message || t("registerPage.ssoLoginFailed"));
    } finally {
      setSSOLoading(null);
    }
  };

  return (
    <div className="container mx-auto px-4 py-16 flex justify-center">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">
            {t("registerPage.createAccount")}
          </CardTitle>
          <CardDescription>{t("registerPage.joinVexgo")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4" noValidate>
            <div className="space-y-2">
              <Label htmlFor="username">
                {t("registerPage.usernameLabel")}
              </Label>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="username"
                  type="text"
                  placeholder={t("registerPage.usernamePlaceholder")}
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="pl-10"
                />
              </div>
              {usernameError && (
                <div className="text-red-500 text-sm mt-1">{usernameError}</div>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="email">{t("auth.email")}</Label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="email"
                  type="email"
                  placeholder={t("registerPage.emailPlaceholder")}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="pl-10"
                />
              </div>
              {emailError && (
                <div className="text-red-500 text-sm mt-1">{emailError}</div>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="password">{t("auth.password")}</Label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  placeholder={t("registerPage.passwordPlaceholder")}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="pl-10 pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showPassword ? (
                    <EyeOff className="w-4 h-4" />
                  ) : (
                    <Eye className="w-4 h-4" />
                  )}
                </button>
              </div>
              {passwordError && (
                <div className="text-red-500 text-sm mt-1">{passwordError}</div>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="confirmPassword">
                {t("registerPage.confirmPasswordLabel")}
              </Label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <Input
                  id="confirmPassword"
                  type={showPassword ? "text" : "password"}
                  placeholder={t("registerPage.confirmPasswordPlaceholder")}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="pl-10"
                />
              </div>
              {confirmPasswordError && (
                <div className="text-red-500 text-sm mt-1">
                  {confirmPasswordError}
                </div>
              )}
            </div>

            {/* Slider captcha area */}
            {captchaEnabled && (
              <div className="mt-4">
                <div className="flex items-center justify-between mb-2">
                  <Label className="text-sm font-medium">
                    {t("registerPage.securityVerification")}
                  </Label>
                  {isCaptchaVerified && (
                    <span className="text-xs text-green-600 flex items-center">
                      <CheckCircle className="h-3 w-3 mr-1" />
                      {t("registerPage.verifiedBadge")}
                    </span>
                  )}
                </div>
                <div className="border rounded-lg p-3 bg-gray-50">
                  {isCaptchaVerified ? (
                    <div className="flex items-center justify-between">
                      <div className="flex items-center text-sm text-green-600">
                        <CheckCircle className="h-4 w-4 mr-2" />
                        {t("registerPage.captchaCompleted")}
                      </div>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={resetCaptcha}
                        className="text-xs h-7"
                      >
                        {t("auth.resetPassword")}
                      </Button>
                    </div>
                  ) : (
                    <div className="flex items-center justify-between">
                      <span className="text-sm text-gray-600">
                        {t("registerPage.completeSlider")}
                      </span>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => setIsCaptchaModalOpen(true)}
                        className="text-xs h-7"
                      >
                        {t("registerPage.verifyButton")}
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
                  {t("registerPage.registering")}
                </>
              ) : (
                t("registerPage.registerButton")
              )}
            </Button>
          </form>

          {/* ── SSO / third-party register ── */}
          {!ssoConfigLoading && enabledProviders.length > 0 && (
            <div className="mt-6">
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs">
                  <span className="bg-background px-2 text-muted-foreground">
                    {t("registerPage.orContinueWith")}
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
                    className={`flex items-center justify-center gap-2 text-sm ${provider.className}`}
                    disabled={ssoLoading !== null}
                    onClick={() => handleSSOLogin(provider.id)}
                  >
                    {ssoLoading === provider.id ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      provider.icon
                    )}
                    <span>{provider.label}</span>
                  </Button>
                ))}
              </div>
            </div>
          )}

          <div className="mt-6 text-center text-sm">
            <span className="text-muted-foreground">
              {t("registerPage.haveAccount")}
            </span>{" "}
            <Link to="/login" className="text-primary hover:underline">
              {t("registerPage.loginNow")}
            </Link>
          </div>
        </CardContent>
      </Card>

      {/* Captcha dialog */}
      <SliderCaptcha
        isOpen={isCaptchaModalOpen}
        onClose={() => {
          // Allow the user to close the captcha dialog at any time
          setIsCaptchaModalOpen(false);
        }}
        onSuccess={handleCaptchaSuccess}
      />
    </div>
  );
}
