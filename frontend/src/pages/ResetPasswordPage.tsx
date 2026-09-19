import { useState, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

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
import { Spinner } from "@/components/ui/spinner";
import { Eye, EyeOff, Mail, Lock, ArrowLeft } from "lucide-react";

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
      await unwrap(getVexGoAPI().postAuthPasswordResetRequest({ email }));
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
      await unwrap(getVexGoAPI().postAuthPasswordReset({ token, password }));
      setSuccess(true);
      setError("");
      // Navigate to the login page after 3 seconds
      setTimeout(() => {
        navigate("/admin/login");
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
    <Card>
      <CardHeader className="text-center">
        <CardTitle className="text-lg">
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
        <div className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          {success && (
            <Alert variant="success">
              <AlertDescription className="text-current">
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
                    <Mail className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                    <Input
                      id="email"
                      type="email"
                      placeholder={t("resetPasswordPage.emailPlaceholder")}
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      className="pl-9"
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
                      <Lock className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                      <Input
                        id="password"
                        type={showPassword ? "text" : "password"}
                        placeholder={t("resetPasswordPage.passwordPlaceholder")}
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        className="pr-9 pl-9"
                        required
                        minLength={6}
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

                  <div className="space-y-2">
                    <Label htmlFor="confirmPassword">
                      {t("resetPasswordPage.confirmPassword")}
                    </Label>
                    <div className="relative">
                      <Lock className="text-muted-foreground absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
                      <Input
                        id="confirmPassword"
                        type={showPassword ? "text" : "password"}
                        placeholder={t("resetPasswordPage.confirmPlaceholder")}
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        className="pl-9"
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
                    <Spinner className="size-4" />
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

          <div className="text-center">
            <button
              type="button"
              onClick={() => navigate("/admin/login")}
              className="text-accent-blue focus-visible:ring-ring/25 mx-auto flex items-center justify-center gap-1 text-[13px] underline-offset-4 outline-none hover:underline focus-visible:ring-[3px]"
            >
              <ArrowLeft className="size-3.5" />
              {t("resetPasswordPage.backToLogin")}
            </button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
