import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { configApi } from "@/lib/api";
import type { SMTPConfig } from "@/types";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { ArrowLeft, Mail, Save, TestTube } from "lucide-react";
import { toast } from "sonner";

export function SMTPSettingsPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [config, setConfig] = useState<SMTPConfig>({
    id: 0,
    enabled: false,
    host: "",
    port: 587,
    username: "",
    password: "",
    fromEmail: "",
    fromName: t("common.siteName") || "VexGo",
    testEmail: "",
    created_at: "",
    updated_at: "",
  });

  const loadConfig = useCallback(async () => {
    try {
      const response = await configApi.getSMTPConfig();
      setConfig(response.data);
    } catch (error: unknown) {
      console.error("Failed to load SMTP config:", error);
      toast.error(t("commentConfig.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const handleSave = async () => {
    if (!config.host.trim()) {
      toast.error(t("smtpSettings.smtpHost") + t("common.required"));
      return;
    }
    if (config.port <= 0 || config.port > 65535) {
      toast.error(t("smtpSettings.smtpPort") + t("common.invalid"));
      return;
    }
    if (!config.username.trim()) {
      toast.error(t("smtpSettings.emailAccount") + t("common.required"));
      return;
    }
    if (config.enabled && !config.password.trim()) {
      toast.error(t("smtpSettings.passwordRequired"));
      return;
    }
    if (!config.fromEmail.trim()) {
      toast.error(t("smtpSettings.fromEmail") + t("common.required"));
      return;
    }

    setSaving(true);
    try {
      await configApi.updateSMTPConfig(config);
      toast.success(t("generalSettings.saveSuccess"));
    } catch (error: unknown) {
      console.error("Failed to save SMTP config:", error);
      const apiError = error as { response?: { data?: { error?: string } } };
      toast.error(
        t("smtpSettings.saveFailed") +
          ": " +
          (apiError.response?.data?.error || t("common.unknownError")),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!config.enabled) {
      toast.error(t("smtpSettings.testFirst"));
      return;
    }
    if (!config.password.trim()) {
      toast.error(t("smtpSettings.savePasswordFirst"));
      return;
    }

    setTesting(true);
    try {
      await configApi.testSMTP();
      toast.success(t("smtpSettings.testSucceeded"));
    } catch (error: unknown) {
      console.error("Failed to send test email:", error);
      const apiError = error as { response?: { data?: { error?: string } } };
      toast.error(
        t("smtpSettings.testFailed") +
          ": " +
          (apiError.response?.data?.error || t("common.unknownError")),
      );
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
      {/* Header */}
      <div className="mb-6">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate("/admin")}
          className="mb-4"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          {t("smtpSettings.backToAdmin")}
        </Button>
        <h1 className="text-3xl font-bold flex items-center gap-2">
          <Mail className="w-8 h-8" />
          {t("smtpSettings.title")}
        </h1>
        <p className="text-muted-foreground mt-2">
          {t("smtpSettings.description")}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("smtpSettings.serverConfig")}</CardTitle>
          <CardDescription>
            {t("smtpSettings.serverConfigDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Enable switch */}
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="enabled">{t("smtpSettings.enableSMTP")}</Label>
              <p className="text-sm text-muted-foreground">
                {t("smtpSettings.enableSMTPDesc")}
              </p>
            </div>
            <Switch
              id="enabled"
              checked={config.enabled}
              onCheckedChange={(checked) =>
                setConfig({ ...config, enabled: checked })
              }
            />
          </div>

          {/* SMTP server address */}
          <div className="space-y-2">
            <Label htmlFor="host">{t("smtpSettings.smtpHost")}</Label>
            <Input
              id="host"
              value={config.host}
              onChange={(e) => setConfig({ ...config, host: e.target.value })}
              placeholder={t("smtpSettings.smtpHostPlaceholder")}
              disabled={saving}
            />
          </div>

          {/* Port */}
          <div className="space-y-2">
            <Label htmlFor="port">{t("smtpSettings.smtpPort")}</Label>
            <Input
              id="port"
              type="number"
              value={config.port}
              onChange={(e) =>
                setConfig({ ...config, port: parseInt(e.target.value) || 587 })
              }
              placeholder={t("smtpSettings.smtpPortPlaceholder")}
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              {t("smtpSettings.commonPorts")}
            </p>
          </div>

          {/* Email account */}
          <div className="space-y-2">
            <Label htmlFor="username">{t("smtpSettings.emailAccount")}</Label>
            <Input
              id="username"
              value={config.username}
              onChange={(e) =>
                setConfig({ ...config, username: e.target.value })
              }
              placeholder="your-email@example.com"
              disabled={saving}
            />
          </div>

          {/* Email password / auth code */}
          <div className="space-y-2">
            <Label htmlFor="password">
              {t("smtpSettings.emailPassword")}{" "}
              {config.enabled && <span className="text-red-500">*</span>}
            </Label>
            <Input
              id="password"
              type="password"
              value={config.password}
              onChange={(e) =>
                setConfig({ ...config, password: e.target.value })
              }
              placeholder={
                config.enabled
                  ? t("smtpSettings.passwordRequired")
                  : t("smtpSettings.emailPasswordPlaceholder")
              }
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              {t("smtpSettings.passwordNote")}
            </p>
          </div>

          {/* Sender email */}
          <div className="space-y-2">
            <Label htmlFor="fromEmail">{t("smtpSettings.fromEmail")}</Label>
            <Input
              id="fromEmail"
              type="email"
              value={config.fromEmail}
              onChange={(e) =>
                setConfig({ ...config, fromEmail: e.target.value })
              }
              placeholder="noreply@yourblog.com"
              disabled={saving}
            />
          </div>

          {/* Sender name */}
          <div className="space-y-2">
            <Label htmlFor="fromName">{t("smtpSettings.fromName")}</Label>
            <Input
              id="fromName"
              value={config.fromName}
              onChange={(e) =>
                setConfig({ ...config, fromName: e.target.value })
              }
              placeholder={t("common.siteName")}
              disabled={saving}
            />
          </div>

          {/* Test email */}
          <div className="space-y-2">
            <Label htmlFor="testEmail">{t("smtpSettings.testEmail")}</Label>
            <Input
              id="testEmail"
              type="email"
              value={config.testEmail}
              onChange={(e) =>
                setConfig({ ...config, testEmail: e.target.value })
              }
              placeholder="his/her-email@example.com"
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              {t("smtpSettings.testEmailDesc")}
            </p>
          </div>

          {/* Action buttons */}
          <div className="flex gap-3 pt-4 border-t">
            <Button onClick={handleSave} disabled={saving} className="flex-1">
              <Save className="w-4 h-4 mr-2" />
              {saving ? t("smtpSettings.saving") : t("smtpSettings.saveConfig")}
            </Button>
            <Button
              variant="outline"
              onClick={handleTest}
              disabled={testing || !config.enabled}
              className="flex-1"
            >
              <TestTube className="w-4 h-4 mr-2" />
              {testing
                ? t("smtpSettings.testing")
                : t("smtpSettings.sendTestEmail")}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Help info */}
      <Card className="mt-6">
        <CardHeader>
          <CardTitle className="text-base">
            {t("smtpSettings.commonExamples")}
          </CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <div>
            <strong>{t("smtpSettings.gmailExample")}</strong>
          </div>
          <div>
            <strong>{t("smtpSettings.qqExample")}</strong>
          </div>
          <div>
            <strong>{t("smtpSettings.neteaseExample")}</strong>
          </div>
          <div>
            <strong>{t("smtpSettings.outlookExample")}</strong>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
