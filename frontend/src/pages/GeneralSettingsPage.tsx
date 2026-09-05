import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "@/lib/I18nContext";
import { configApi, uploadApi } from "@/lib/api";
import type { GeneralSettings } from "@/types";
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
import { ArrowLeft, Settings, Save, Upload, Trash2 } from "lucide-react";
import { toast } from "sonner";

export function GeneralSettingsPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<GeneralSettings>({
    id: 0,
    captchaEnabled: false,
    registrationEnabled: true,
    allowGuestViewPosts: true,
    siteName: t("common.siteName") || "VexGo",
    siteDescription: "",
    siteIcon: "",
    itemsPerPage: 20,
    created_at: "",
    updated_at: "",
  });
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadConfig = useCallback(async () => {
    try {
      const response = await configApi.getGeneralSettings();
      setConfig(response.data);
    } catch (error) {
      console.error("Failed to load general settings:", error);
      toast.error(t("generalSettings.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const handleSave = async () => {
    if (!config.siteName.trim()) {
      toast.error(t("generalSettings.siteNameRequired"));
      return;
    }
    if (config.itemsPerPage <= 0 || config.itemsPerPage > 100) {
      toast.error(t("generalSettings.itemsPerPageInvalid"));
      return;
    }

    setSaving(true);
    try {
      await configApi.updateGeneralSettings(config);
      toast.success(t("generalSettings.saveSuccess"));
    } catch (error) {
      console.error("Failed to save general settings:", error);
      const err = error as { response?: { data?: { error?: string } } };
      toast.error(
        t("generalSettings.saveFailed") +
          ": " +
          (err.response?.data?.error || t("common.unknownError")),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleIconUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.type.startsWith("image/")) {
      toast.error(t("generalSettings.iconInvalidType"));
      return;
    }

    try {
      const response = await uploadApi.uploadFile(file);
      if (response.data.file?.url) {
        setConfig({ ...config, siteIcon: response.data.file.url });
      }
      toast.success(t("generalSettings.iconUploadSuccess"));
    } catch (error) {
      console.error("Failed to upload icon:", error);
      toast.error(t("generalSettings.iconUploadFailed"));
    }

    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleRemoveIcon = () => {
    setConfig({ ...config, siteIcon: "" });
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
          {t("generalSettings.backToAdmin")}
        </Button>
        <h1 className="text-3xl font-bold flex items-center gap-2">
          <Settings className="w-8 h-8" />
          {t("generalSettings.title")}
        </h1>
        <p className="text-muted-foreground mt-2">
          {t("generalSettings.description")}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("generalSettings.basicSettings")}</CardTitle>
          <CardDescription>
            {t("generalSettings.basicSettingsDesc")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Site name */}
          <div className="space-y-2">
            <Label htmlFor="siteName">{t("generalSettings.siteName")}</Label>
            <Input
              id="siteName"
              value={config.siteName}
              onChange={(e) =>
                setConfig({ ...config, siteName: e.target.value })
              }
              placeholder={t("generalSettings.siteNamePlaceholder")}
            />
          </div>

          {/* Site description */}
          <div className="space-y-2">
            <Label htmlFor="siteDescription">
              {t("generalSettings.siteDescription")}
            </Label>
            <Input
              id="siteDescription"
              value={config.siteDescription}
              onChange={(e) =>
                setConfig({ ...config, siteDescription: e.target.value })
              }
              placeholder={t("generalSettings.siteDescriptionPlaceholder")}
            />
          </div>

          {/* Site icon */}
          <div className="space-y-2">
            <Label>{t("generalSettings.siteIcon")}</Label>
            <div className="flex items-center gap-4">
              {config.siteIcon ? (
                <div className="relative w-16 h-16 rounded-lg border overflow-hidden">
                  <img
                    src={config.siteIcon}
                    alt="Site Icon"
                    className="w-full h-full object-cover"
                  />
                  <button
                    type="button"
                    onClick={handleRemoveIcon}
                    className="absolute -top-2 -right-2 bg-destructive text-destructive-foreground rounded-full p-0.5"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              ) : (
                <div className="w-16 h-16 rounded-lg border border-dashed flex items-center justify-center bg-muted/30">
                  <Settings className="w-6 h-6 text-muted-foreground" />
                </div>
              )}
              <div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <Upload className="w-4 h-4 mr-2" />
                  {t("generalSettings.iconUpload")}
                </Button>
                <p className="text-xs text-muted-foreground mt-1">
                  {t("generalSettings.iconDesc")}
                </p>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/png,image/jpeg,image/svg+xml,image/x-icon,image/vnd.microsoft.icon"
                  className="hidden"
                  onChange={handleIconUpload}
                />
              </div>
            </div>
          </div>

          {/* Items per page */}
          <div className="space-y-2">
            <Label htmlFor="itemsPerPage">
              {t("generalSettings.itemsPerPage")}
            </Label>
            <Input
              id="itemsPerPage"
              type="number"
              min={1}
              max={100}
              value={config.itemsPerPage}
              onChange={(e) =>
                setConfig({
                  ...config,
                  itemsPerPage: parseInt(e.target.value) || 20,
                })
              }
              placeholder={t("generalSettings.itemsPerPagePlaceholder")}
            />
            <p className="text-xs text-muted-foreground">
              {t("generalSettings.itemsPerPageDesc")}
            </p>
          </div>

          {/* Enable slider captcha */}
          <div className="flex items-center justify-between rounded-lg border p-4">
            <div className="space-y-0.5">
              <Label htmlFor="captchaEnabled">
                {t("generalSettings.captcha")}
              </Label>
              <p className="text-sm text-muted-foreground">
                {t("generalSettings.captchaDesc")}
              </p>
            </div>
            <Switch
              id="captchaEnabled"
              checked={config.captchaEnabled}
              onCheckedChange={(checked) =>
                setConfig({ ...config, captchaEnabled: checked })
              }
            />
          </div>

          {/* Allow registration */}
          <div className="flex items-center justify-between rounded-lg border p-4">
            <div className="space-y-0.5">
              <Label htmlFor="registrationEnabled">
                {t("generalSettings.registration")}
              </Label>
              <p className="text-sm text-muted-foreground">
                {t("generalSettings.registrationDesc")}
              </p>
            </div>
            <Switch
              id="registrationEnabled"
              checked={config.registrationEnabled}
              onCheckedChange={(checked) =>
                setConfig({ ...config, registrationEnabled: checked })
              }
            />
          </div>

          {/* Allow guests to view posts */}
          <div className="flex items-center justify-between rounded-lg border p-4">
            <div className="space-y-0.5">
              <Label htmlFor="allowGuestViewPosts">
                {t("generalSettings.allowGuestViewPosts")}
              </Label>
              <p className="text-sm text-muted-foreground">
                {t("generalSettings.allowGuestViewPostsDesc")}
              </p>
            </div>
            <Switch
              id="allowGuestViewPosts"
              checked={config.allowGuestViewPosts}
              onCheckedChange={(checked) =>
                setConfig({ ...config, allowGuestViewPosts: checked })
              }
            />
          </div>
        </CardContent>
      </Card>

      {/* Save button */}
      <div className="mt-6 flex justify-end">
        <Button onClick={handleSave} disabled={saving} size="lg">
          {saving ? (
            <>{t("generalSettings.saving")}</>
          ) : (
            <>
              <Save className="w-4 h-4 mr-2" />
              {t("generalSettings.saveSettings")}
            </>
          )}
        </Button>
      </div>
    </div>
  );
}
