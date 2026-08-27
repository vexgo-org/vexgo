import { useEffect, useState } from "react";
import { useSearchParams, Link, useNavigate } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { authApi } from "@/lib/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Loader2, CheckCircle, XCircle } from "lucide-react";

export function VerifyEmailPage() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token");
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [status, setStatus] = useState<"loading" | "success" | "error">(
    token ? "loading" : "error",
  );
  const [message, setMessage] = useState(
    token ? "" : t("verifyEmail.tokenEmpty"),
  );
  const [requireRelogin, setRequireRelogin] = useState(false);

  useEffect(() => {
    if (!token) {
      return;
    }

    const verifyEmail = async () => {
      try {
        const response = await authApi.verifyEmail(token);
        setStatus("success");
        setMessage(
          response.data.message || t("verifyEmail.emailVerificationSuccess"),
        );

        // Check whether re-login is required (email change succeeded)
        if (response.data.require_relogin) {
          setRequireRelogin(true);
          // Clear the local login state
          localStorage.removeItem("token");
          localStorage.removeItem("user");
        }
      } catch (err: unknown) {
        setStatus("error");
        const errorMessage =
          err instanceof Error ? err.message : t("verifyEmail.failed");
        setMessage(errorMessage);
      }
    };

    verifyEmail();
  }, [token, t]);

  return (
    <div className="container mx-auto px-4 py-16 flex justify-center">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl flex items-center justify-center gap-2">
            {status === "loading" && (
              <Loader2 className="w-6 h-6 animate-spin" />
            )}
            {status === "success" && (
              <CheckCircle className="w-6 h-6 text-green-500" />
            )}
            {status === "error" && <XCircle className="w-6 h-6 text-red-500" />}
            {t("verifyEmail.title")}
          </CardTitle>
          <CardDescription>
            {status === "loading" && t("verifyEmail.verifying")}
            {status === "success" && t("verifyEmail.success")}
            {status === "error" && t("verifyEmail.failed")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-center text-muted-foreground">{message}</p>

          <div className="flex gap-3">
            {requireRelogin ? (
              <Button
                asChild
                className="flex-1"
                onClick={() => navigate("/login")}
              >
                {t("verifyEmail.goToLogin")}
              </Button>
            ) : (
              <>
                <Button asChild className="flex-1">
                  <Link to="/login">{t("verifyEmail.goToLogin")}</Link>
                </Button>
                {status === "error" && (
                  <Button variant="outline" asChild className="flex-1">
                    <Link to="/">{t("verifyEmail.backToHome")}</Link>
                  </Button>
                )}
              </>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
