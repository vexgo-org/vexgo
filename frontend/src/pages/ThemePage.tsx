import { useCallback, useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { PageHeader } from "@/components/PageHeader";
import { MetaList, MetaRow } from "@/components/MetaList";
import { RouteFallback } from "@/components/RouteFallback";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Check,
  AlertCircle,
  Upload,
  Eye,
  Trash2,
  ExternalLink,
  ImageIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";

function isPreviewURL(value?: string): value is string {
  if (!value) return false;
  if (value.length > 2048) return false;
  try {
    const u = new URL(value);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}

function ThemeCover({
  theme,
  alt,
  className,
  imgClassName,
}: {
  theme: ThemeInfo;
  alt: string;
  className?: string;
  imgClassName?: string;
}) {
  const { t } = useTranslation();
  const [failed, setFailed] = useState(false);
  const src = isPreviewURL(theme.preview) && !failed ? theme.preview : null;
  if (!src) {
    return (
      <div
        className={cn(
          "flex flex-col items-center justify-center gap-1.5 bg-muted text-muted-foreground",
          className,
        )}
      >
        <ImageIcon className="size-4" aria-hidden="true" />
        <span className="eyebrow">{t("themePage.noPreview")}</span>
      </div>
    );
  }
  return (
    <div className={cn("overflow-hidden bg-muted", className)}>
      <img
        src={src}
        alt={alt}
        loading="lazy"
        referrerPolicy="no-referrer"
        onError={() => setFailed(true)}
        className={cn("w-full h-full object-cover", imgClassName)}
      />
    </div>
  );
}

interface ThemeInfo {
  id: string;
  name: string;
  author: string;
  version: string;
  description: string;
  url: string;
  preview?: string;
}

export function ThemePage() {
  const navigate = useNavigate();
  const { user } = useAuth();
  const { t } = useTranslation();

  const [themes, setThemes] = useState<ThemeInfo[]>([]);
  const [activeTheme, setActiveTheme] = useState<string>("default");
  const [loading, setLoading] = useState(true);
  const [applying, setApplying] = useState<string | null>(null);
  const [message, setMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);
  const [uploading, setUploading] = useState(false);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [selectedTheme, setSelectedTheme] = useState<ThemeInfo | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [themesRes, configRes] = await Promise.all([
        unwrap(getVexGoAPI().getConfigThemes()),
        unwrap(getVexGoAPI().getConfigTheme()),
      ]);
      setThemes((themesRes.themes || []) as ThemeInfo[]);
      setActiveTheme(configRes.activeTheme || "default");
    } catch (error) {
      console.error("Failed to load theme data:", error);
      setMessage({ type: "error", text: t("themePage.loadFailed") });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    window.scrollTo(0, 0);

    if (user && user.role !== "admin" && user.role !== "super_admin") {
      navigate("/admin/my-posts");
      return;
    }

    loadData();
  }, [user, navigate, loadData]);

  const handleApplyTheme = async (themeId: string) => {
    setApplying(themeId);
    setMessage(null);
    try {
      await unwrap(getVexGoAPI().putConfigTheme({ activeTheme: themeId }));
      setActiveTheme(themeId);
      const themeName = themes.find((t) => t.id === themeId)?.name || themeId;
      setMessage({
        type: "success",
        text: t("themePage.applySuccess", { themeName }),
      });
    } catch (error) {
      console.error("Failed to apply theme:", error);
      setMessage({ type: "error", text: t("themePage.applyFailed") });
    } finally {
      setApplying(null);
    }
  };

  const handleUploadClick = () => {
    fileInputRef.current?.click();
  };

  const handlePreviewTheme = async (themeId: string) => {
    // The preview needs a short-lived signed link, which the API mints for an
    // admin. Open the tab synchronously so the popup blocker still sees the
    // click, then navigate it once the link arrives.
    const tab = window.open("about:blank", "_blank");
    if (tab) {
      tab.opener = null;
    }
    try {
      const res = await unwrap(
        getVexGoAPI().getConfigThemesIdPreviewLink(themeId),
      );
      if (!res.url) {
        throw new Error("theme preview link missing from response");
      }
      if (tab) {
        tab.location.replace(res.url);
      } else {
        window.location.assign(res.url);
      }
    } catch {
      tab?.close();
      setMessage({ type: "error", text: t("themePage.previewFailed") });
    }
  };

  const handleDeleteTheme = async (theme: ThemeInfo) => {
    if (
      theme.id === "default" ||
      activeTheme === theme.id ||
      deleting !== null
    ) {
      return;
    }
    const confirmed = window.confirm(
      t("themePage.deleteConfirm", { themeName: theme.name }),
    );
    if (!confirmed) return;
    setDeleting(theme.id);
    setMessage(null);
    try {
      await unwrap(getVexGoAPI().deleteConfigThemesId(theme.id));
      setMessage({
        type: "success",
        text: t("themePage.deleteSuccess", { themeName: theme.name }),
      });
      setSelectedTheme(null);
      await loadData();
    } catch (error) {
      console.error("Failed to delete theme:", error);
      setMessage({ type: "error", text: t("themePage.deleteFailed") });
    } finally {
      setDeleting(null);
    }
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Check the file type
    if (!file.name.endsWith(".zip")) {
      setMessage({
        type: "error",
        text: t("themePage.uploadInvalidType"),
      });
      return;
    }

    setUploading(true);
    setMessage(null);

    try {
      const res = await unwrap(
        getVexGoAPI().postConfigThemeUpload({ theme: file }),
      );
      const themeName = file.name.replace(/\.zip$/i, "");
      setMessage({
        type: "success",
        text: res.overwritten
          ? t("themePage.uploadOverwritten", { themeName })
          : t("themePage.uploadSuccess"),
      });
      // Reload the theme list
      loadData();
    } catch (error) {
      console.error("Failed to upload theme:", error);
      setMessage({ type: "error", text: t("themePage.uploadFailed") });
    } finally {
      setUploading(false);
      // Clear the file input
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  if (loading) {
    return <RouteFallback />;
  }

  return (
    <div className="max-w-6xl space-y-6">
      <PageHeader
        title={t("themePage.title")}
        description={t("themePage.description")}
        actions={
          <>
            <Button onClick={handleUploadClick} disabled={uploading}>
              {uploading ? <Spinner className="size-4" /> : <Upload />}
              {t("themePage.uploadTheme")}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".zip"
              onChange={handleFileChange}
              className="hidden"
            />
          </>
        }
      />

      {message && (
        <Alert variant={message.type === "success" ? "success" : "destructive"}>
          {message.type === "success" ? <Check /> : <AlertCircle />}
          <AlertDescription className="text-current">
            {message.text}
          </AlertDescription>
        </Alert>
      )}

      {themes.length === 0 ? (
        <div className="border-t border-border py-16 text-center">
          <p className="display text-subtitle">
            {t("themePage.noThemesFound")}
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-[repeat(auto-fill,minmax(20rem,1fr))]">
          {themes.map((theme) => {
            const isActive = activeTheme === theme.id;
            const isDefault = theme.id === "default";
            const canDelete = !isDefault && !isActive && deleting === null;
            return (
              <Dialog key={theme.id}>
                <DialogTrigger
                  nativeButton={false}
                  render={<div />}
                  className={cn(
                    "group flex cursor-pointer flex-col overflow-hidden rounded-md border bg-card text-left transition-colors",
                    isActive
                      ? "border-rule"
                      : "border-border hover:border-foreground/35",
                  )}
                  onClick={() => setSelectedTheme(theme)}
                >
                  <ThemeCover
                    theme={theme}
                    alt={`${theme.name} preview`}
                    className="h-44 w-full border-b border-border"
                  />
                  <div className="flex flex-1 flex-col gap-3 p-4">
                    <div className="flex items-baseline justify-between gap-3">
                      <h3 className="display text-subtitle min-w-0 truncate">
                        {theme.name}
                      </h3>
                      {isActive && (
                        <Badge variant="outline">
                          {t("themePage.currentBadge")}
                        </Badge>
                      )}
                    </div>
                    <MetaList>
                      <MetaRow label={t("themePage.author")}>
                        <span className="block truncate">{theme.author}</span>
                      </MetaRow>
                      <MetaRow label={t("themePage.version")}>
                        <span className="tabular-nums">{theme.version}</span>
                      </MetaRow>
                      {theme.description && (
                        <MetaRow label={t("themePage.themeDescription")}>
                          <span className="line-clamp-2">
                            {theme.description}
                          </span>
                        </MetaRow>
                      )}
                    </MetaList>
                  </div>
                  {/* The destructive action moved into the detail sheet, where
                   * it sits next to the theme it deletes; a gallery tile is
                   * for choosing, not for destroying. */}
                  <div className="flex items-center justify-between gap-2 border-t border-border px-4 py-3">
                    <Button variant="ghost" size="sm">
                      <Eye />
                      {t("themePage.viewDetails")}
                    </Button>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation();
                          handlePreviewTheme(theme.id);
                        }}
                      >
                        <ExternalLink />
                        {t("themePage.previewTheme")}
                      </Button>
                      <Button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleApplyTheme(theme.id);
                        }}
                        disabled={isActive || applying !== null}
                        variant={isActive ? "outline" : "default"}
                        size="sm"
                      >
                        {applying === theme.id && (
                          <Spinner className="size-3.5" />
                        )}
                        {isActive
                          ? t("themePage.applied")
                          : t("themePage.applyTheme")}
                      </Button>
                    </div>
                  </div>
                </DialogTrigger>
                <DialogContent className="sm:max-w-md">
                  {selectedTheme && selectedTheme.id === theme.id && (
                    <>
                      <DialogHeader>
                        <DialogTitle className="display text-subtitle">
                          {selectedTheme.name}
                        </DialogTitle>
                        <DialogDescription>
                          {t("themePage.description")}
                        </DialogDescription>
                      </DialogHeader>
                      <MetaList className="space-y-2">
                        <MetaRow label={t("themePage.author")}>
                          {selectedTheme.author}
                        </MetaRow>
                        <MetaRow label={t("themePage.version")}>
                          {selectedTheme.version}
                        </MetaRow>
                        {selectedTheme.description && (
                          <MetaRow label={t("themePage.themeDescription")}>
                            {selectedTheme.description}
                          </MetaRow>
                        )}
                        {selectedTheme.url && (
                          <MetaRow label={t("themePage.link")}>
                            <a
                              href={selectedTheme.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-accent-ink break-all hover:underline"
                            >
                              {selectedTheme.url}
                            </a>
                          </MetaRow>
                        )}
                      </MetaList>
                      <div>
                        <p className="eyebrow mb-2">
                          {t("themePage.previewCover")}
                        </p>
                        <ThemeCover
                          theme={selectedTheme}
                          alt={`${selectedTheme.name} preview`}
                          className="aspect-video w-full rounded-md"
                        />
                      </div>
                      <div className="flex flex-wrap justify-end gap-2">
                        <Button
                          variant="outline"
                          onClick={() => handlePreviewTheme(selectedTheme.id)}
                        >
                          <ExternalLink />
                          {t("themePage.previewTheme")}
                        </Button>
                        {!isDefault && (
                          <Button
                            variant="outline"
                            onClick={() => handleDeleteTheme(selectedTheme)}
                            disabled={
                              activeTheme === selectedTheme.id ||
                              !canDelete ||
                              deleting !== null
                            }
                          >
                            {deleting === selectedTheme.id ? (
                              <Spinner className="size-4" />
                            ) : (
                              <Trash2 />
                            )}
                            {t("themePage.deleteTheme")}
                          </Button>
                        )}
                        <Button
                          onClick={() => handleApplyTheme(selectedTheme.id)}
                          disabled={
                            activeTheme === selectedTheme.id ||
                            applying !== null
                          }
                          variant={
                            activeTheme === selectedTheme.id
                              ? "outline"
                              : "default"
                          }
                        >
                          {applying === selectedTheme.id && (
                            <Spinner className="size-4" />
                          )}
                          {activeTheme === selectedTheme.id
                            ? t("themePage.applied")
                            : t("themePage.applyTheme")}
                        </Button>
                      </div>
                    </>
                  )}
                </DialogContent>
              </Dialog>
            );
          })}
        </div>
      )}

      {/* The install notes are reference prose, so they read as an appendix:
       * two numbered methods separated by a rule, with the paths set in the
       * mono face rather than in a tinted box inside a tinted box. */}
      <Card>
        <CardHeader className="border-b border-border pb-4">
          <CardTitle>{t("themePage.installationInstructions")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <section className="space-y-1">
            <h3 className="eyebrow">{t("themePage.method1.title")}</h3>
            <p className="text-sm leading-relaxed">
              {t("themePage.method1.step1")}
            </p>
            <p className="text-sm leading-relaxed">
              {t("themePage.method1.step2")}
            </p>
            <p className="text-sm leading-relaxed">
              {t("themePage.method1.step3")}
            </p>
          </section>
          <section className="space-y-1 border-t border-border pt-4">
            <h3 className="eyebrow">{t("themePage.method2.title")}</h3>
            <p className="text-sm leading-relaxed">
              1. {t("themePage.instruction1")} <Code>./data/theme/</Code>{" "}
              {t("themePage.instruction2")}
            </p>
            <p className="text-sm leading-relaxed">
              2. {t("themePage.instruction3")} <Code>vexgo-theme.json</Code>{" "}
              {t("themePage.instruction4")}
            </p>
            <p className="text-sm leading-relaxed">
              3. {t("themePage.instruction5")} <Code>dist/</Code>{" "}
              {t("themePage.instruction6")}
            </p>
            <p className="text-sm leading-relaxed">
              4. {t("themePage.instruction7")}
            </p>
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t("themePage.metaRequirements")}
            </p>
          </section>
        </CardContent>
      </Card>
    </div>
  );
}

function Code({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded-sm border border-border bg-muted px-1.5 py-0.5 font-mono text-xs">
      {children}
    </code>
  );
}
