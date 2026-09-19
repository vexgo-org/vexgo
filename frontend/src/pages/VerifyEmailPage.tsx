import { useEffect, useState } from "react";
import { useSearchParams, Link, useNavigate } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { CheckCircle, XCircle } from "lucide-react";

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
        const response = await unwrap(
          getVexGoAPI().getAuthEmailVerify({ token }),
        );
        setStatus("success");
        setMessage(
          response.message || t("verifyEmail.emailVerificationSuccess"),
        );

        // Check whether re-login is required (email change succeeded)
        if (response.require_relogin) {
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
    <Card>
      <CardHeader className="text-center">
        <CardTitle className="text-lg">{t("verifyEmail.title")}</CardTitle>
        <CardDescription>
          {status === "loading" && t("verifyEmail.verifying")}
          {status === "success" && t("verifyEmail.success")}
          {status === "error" && t("verifyEmail.failed")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {status === "loading" ? (
          <div className="flex justify-center py-2">
            <Spinner className="text-muted-foreground size-5" />
          </div>
        ) : (
          <Alert
            variant={status === "success" ? "success" : "destructive"}
            className="items-center"
          >
            {status === "success" ? <CheckCircle /> : <XCircle />}
            <AlertDescription className="text-current">
              {message}
            </AlertDescription>
          </Alert>
        )}

        <div className="flex gap-2">
          {requireRelogin ? (
            <Button className="flex-1" onClick={() => navigate("/admin/login")}>
              {t("verifyEmail.goToLogin")}
            </Button>
          ) : (
            <>
              <Button asChild className="flex-1">
                <Link to="/admin/login">{t("verifyEmail.goToLogin")}</Link>
              </Button>
              {status === "error" && (
                <Button variant="outline" asChild className="flex-1">
                  <Link to="/admin">{t("verifyEmail.backToHome")}</Link>
                </Button>
              )}
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
