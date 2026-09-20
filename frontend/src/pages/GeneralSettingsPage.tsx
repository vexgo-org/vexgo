import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Save, Settings, Upload, Trash2 } from "lucide-react";
import { SkeletonForm } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/PageHeader";
import { toast } from "sonner";

export function GeneralSettingsPage() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<GeneralSettings>({
    id: "",
    captchaEnabled: false,
    registrationEnabled: true,
    allowGuestViewPosts: true,
    siteName: t("common.siteName") || "VexGo",
    siteDescription: "",
    siteIcon: "",
    itemsPerPage: 20,
    siteLanguage: "en",
    createdAt: "",
    updatedAt: "",
  });
  const [themeLanguages, setThemeLanguages] = useState<string[]>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadConfig = useCallback(async () => {
    try {
      const response = await unwrap(getVexGoAPI().getConfigGeneral());
      setConfig((prev) => ({ ...prev, ...response }));
    } catch (error) {
      console.error("Failed to load general settings:", error);
      toast.error(t("generalSettings.loadFailed"));
    } finally {
      setLoading(false);
    }
    try {
      const theme = await unwrap(getVexGoAPI().getConfigTheme());
      const active =
        (theme as { activeTheme?: string }).activeTheme || "default";
      const langs = await unwrap(
        getVexGoAPI().getConfigThemesIdLanguages(active),
      );
      const list = (langs as { languages?: string[] }).languages || [];
      setThemeLanguages(list);
    } catch {
      setThemeLanguages([]);
    }
  }, [t]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const handleSave = async () => {
    if (!(config.siteName || "").trim()) {
      toast.error(t("generalSettings.siteNameRequired"));
      return;
    }
    if ((config.itemsPerPage || 10) <= 0 || (config.itemsPerPage || 10) > 100) {
      toast.error(t("generalSettings.itemsPerPageInvalid"));
      return;
    }

    setSaving(true);
    try {
      await unwrap(getVexGoAPI().putConfigGeneral(config));
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
      const response = await unwrap(getVexGoAPI().postUpload({ file }));
      if (response.file?.url) {
        setConfig({ ...config, siteIcon: response.file.url });
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
      <div className="mx-auto max-w-3xl space-y-6">
        <PageHeader
          title={t("generalSettings.title")}
          description={t("generalSettings.description")}
        />
        <SkeletonForm rows={5} />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageHeader
        title={t("generalSettings.title")}
        description={t("generalSettings.description")}
      />

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
                <div className="border-border relative size-16 overflow-hidden rounded-sm border">
                  <img
                    src={config.siteIcon}
                    alt={t("generalSettings.siteIcon")}
                    className="size-full object-cover"
                  />
                  <button
                    type="button"
                    onClick={handleRemoveIcon}
                    aria-label={t("common.remove")}
                    title={t("common.remove")}
                    className="bg-destructive text-destructive-foreground focus-visible:ring-ring/35 absolute -top-1.5 -right-1.5 rounded-sm p-1 outline-none focus-visible:ring-[3px]"
                  >
                    <Trash2 className="size-3" />
                  </button>
                </div>
              ) : (
                <div className="border-border bg-muted/30 flex size-16 items-center justify-center rounded-sm border border-dashed">
                  <Settings className="size-5 text-muted-foreground" />
                </div>
              )}
              <div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => fileInputRef.current?.click()}
                >
                  <Upload />
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

          {/* Site language */}
          <div className="space-y-2">
            <Label htmlFor="siteLanguage">
              {t("generalSettings.siteLanguage")}
            </Label>
            <Select
              value={config.siteLanguage || "en"}
              onValueChange={(value: string) =>
                setConfig({ ...config, siteLanguage: value })
              }
            >
              <SelectTrigger id="siteLanguage">
                <SelectValue placeholder={t("generalSettings.siteLanguage")} />
              </SelectTrigger>
              <SelectContent>
                {Array.from(new Set(["en", "zh", ...themeLanguages])).map(
                  (lang) => (
                    <SelectItem key={lang} value={lang}>
                      {lang === "zh"
                        ? t("generalSettings.siteLanguageZh")
                        : lang === "en"
                          ? t("generalSettings.siteLanguageEn")
                          : lang}
                    </SelectItem>
                  ),
                )}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              {t("generalSettings.siteLanguageDesc")}
            </p>
            {themeLanguages.length > 0 &&
              config.siteLanguage &&
              !themeLanguages.includes(config.siteLanguage) && (
                <p className="text-warning text-2xs">
                  {t("generalSettings.siteLanguageMissing")}
                </p>
              )}
          </div>

          {/* Enable slider captcha */}
          <div className="border-border flex items-center justify-between border-t pt-4">
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
          <div className="border-border flex items-center justify-between border-t pt-4">
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
          <div className="border-border flex items-center justify-between border-t pt-4">
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
      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={saving} size="lg">
          {saving ? (
            <>{t("generalSettings.saving")}</>
          ) : (
            <>
              <Save className="size-4" />
              {t("generalSettings.saveSettings")}
            </>
          )}
        </Button>
      </div>
    </div>
  );
}
