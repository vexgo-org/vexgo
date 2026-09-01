import { useState, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { authApi } from "@/lib/api";
import { useTranslation } from "@/lib/I18nContext";
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
import { Loader2, Mail, Lock, ArrowLeft } from "lucide-react";

export function ResetPasswordPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token");

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const [step, setStep] = useState<"request" | "reset">("request");

  // If the URL contains a token, go straight to the reset password step
  useEffect(() => {
    if (token) {
      setStep("reset");
    }
  }, [token]);

  const handleRequestReset = async (e: React.SubmitEvent) => {
    e.preventDefault();
    setError("");

    if (!email) {
      setError(t("resetPasswordPage.emailRequired"));
      return;
    }

    setLoading(true);

    try {
      await authApi.requestPasswordReset({ email });
      setSuccess(true);
      setError("");
    } catch (err: unknown) {
      const apiError = err as { response?: { data?: { error?: string } } };
      setError(
        apiError.response?.data?.error || t("resetPasswordPage.requestFailed"),
      );
    } finally {
      setLoading(false);
    }
  };

  const handleResetPassword = async (e: React.SubmitEvent) => {
    e.preventDefault();
    setError("");

    if (!token) {
      setError(t("resetPasswordPage.missingToken"));
      return;
    }

    if (password.length < 6) {
      setError(t("resetPasswordPage.passwordTooShort"));
      return;
    }

    if (password !== confirmPassword) {
      setError(t("resetPasswordPage.passwordMismatch"));
      return;
    }

    setLoading(true);

    try {
      await authApi.resetPassword({ token, password });
      setSuccess(true);
      setError("");
      // Navigate to the login page after 3 seconds
      setTimeout(() => {
        navigate("/login");
      }, 3000);
    } catch (err: unknown) {
      const apiError = err as { response?: { data?: { error?: string } } };
      setError(
        apiError.response?.data?.error || t("resetPasswordPage.resetFailed"),
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container mx-auto px-4 py-16 flex justify-center">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">
            {step === "request"
              ? t("resetPasswordPage.findPassword")
              : t("resetPasswordPage.resetPassword")}
          </CardTitle>
          <CardDescription>
            {step === "request"
              ? t("resetPasswordPage.resetInstruction")
              : t("resetPasswordPage.newPasswordInstruction")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {success && (
            <Alert variant="default" className="mb-4">
              <AlertDescription>
                {step === "request"
                  ? t("resetPasswordPage.resetLinkSent")
                  : t("resetPasswordPage.resetSuccess")}
              </AlertDescription>
            </Alert>
          )}

          {!success && (
            <form
              onSubmit={
                step === "request" ? handleRequestReset : handleResetPassword
              }
              className="space-y-4"
            >
              {step === "request" ? (
                <div className="space-y-2">
                  <Label htmlFor="email">
                    {t("resetPasswordPage.emailLabel")}
                  </Label>
                  <div className="relative">
                    <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                      id="email"
                      type="email"
                      placeholder={t("resetPasswordPage.emailPlaceholder")}
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      className="pl-10"
                      required
                    />
                  </div>
                </div>
              ) : (
                <>
                  <div className="space-y-2">
                    <Label htmlFor="password">
                      {t("resetPasswordPage.newPassword")}
                    </Label>
                    <div className="relative">
                      <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                      <Input
                        id="password"
                        type={showPassword ? "text" : "password"}
                        placeholder={t("resetPasswordPage.passwordPlaceholder")}
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        className="pl-10 pr-10"
                        required
                        minLength={6}
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                      >
                        {showPassword ? (
                          <span className="text-sm">{t("common.hide")}</span>
                        ) : (
                          <span className="text-sm">{t("common.show")}</span>
                        )}
                      </button>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="confirmPassword">
                      {t("resetPasswordPage.confirmPassword")}
                    </Label>
                    <div className="relative">
                      <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                      <Input
                        id="confirmPassword"
                        type={showPassword ? "text" : "password"}
                        placeholder={t("resetPasswordPage.confirmPlaceholder")}
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        className="pl-10"
                        required
                        minLength={6}
                      />
                    </div>
                  </div>
                </>
              )}

              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? (
                  <>
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                    {step === "request"
                      ? t("resetPasswordPage.sending")
                      : t("resetPasswordPage.resetting")}
                  </>
                ) : step === "request" ? (
                  t("resetPasswordPage.sendResetLink")
                ) : (
                  t("resetPasswordPage.resetPasswordButton")
                )}
              </Button>
            </form>
          )}

          <div className="mt-6 text-center text-sm">
            <button
              type="button"
              onClick={() => navigate("/login")}
              className="text-primary hover:underline focus:outline-none flex items-center justify-center mx-auto"
            >
              <ArrowLeft className="w-4 h-4 mr-1" />
              {t("resetPasswordPage.backToLogin")}
            </button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
