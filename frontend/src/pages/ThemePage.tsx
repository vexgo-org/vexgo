import { useCallback, useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/useAuth";
import { useTranslation } from "@/lib/I18nContext";
import api from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Loader2,
  Check,
  AlertCircle,
  Palette,
  Upload,
  Eye,
} from "lucide-react";
import { cn } from "@/lib/utils";

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
  const [selectedTheme, setSelectedTheme] = useState<ThemeInfo | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [themesRes, configRes] = await Promise.all([
        api.get<{ themes: ThemeInfo[] }>("/themes"),
        api.get<{ activeTheme: string }>("/config/theme", {
          params: { _t: Date.now() }, // add a timestamp to bust the cache
        }),
      ]);
      setThemes(themesRes.data.themes || []);
      setActiveTheme(configRes.data.activeTheme || "default");
      console.log("activeTheme from API:", configRes.data.activeTheme);
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
      navigate("/");
      return;
    }

    loadData();
  }, [user, navigate, loadData]);

  const handleApplyTheme = async (themeId: string) => {
    setApplying(themeId);
    setMessage(null);
    try {
      await api.put("/config/theme", { activeTheme: themeId });
      setActiveTheme(themeId);
      const themeName = themes.find((t) => t.id === themeId)?.name || themeId;
      setMessage({
        type: "success",
        text: t("themePage.applySuccess", { themeName }),
      });

      // Reload the page after a short delay so the new theme takes effect
      // setTimeout(() => {
      //   window.location.reload();
      // }, 1500);
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

    const formData = new FormData();
    formData.append("theme", file);

    try {
      await api.post("/themes/upload", formData, {
        headers: {
          "Content-Type": "multipart/form-data",
        },
      });
      setMessage({ type: "success", text: t("themePage.uploadSuccess") });
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
    return (
      <div className="flex items-center justify-center min-h-screen">
        <Loader2 className="w-8 h-8 animate-spin" />
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-6xl">
      <div className="space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <h1 className="text-3xl font-bold flex items-center gap-2">
              <Palette className="w-8 h-8" />
              {t("themePage.title")}
            </h1>
            <p className="text-muted-foreground mt-2">
              {t("themePage.description")}
            </p>
          </div>
          <Button
            onClick={handleUploadClick}
            disabled={uploading}
            className="flex items-center gap-2"
          >
            {uploading && <Loader2 className="w-4 h-4 animate-spin" />}
            <Upload className="w-4 h-4" />
            {t("themePage.uploadTheme")}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".zip"
            onChange={handleFileChange}
            className="hidden"
          />
        </div>

        {message && (
          <Alert
            className={cn(
              message.type === "success"
                ? "border-green-500/50 bg-green-500/10"
                : "border-red-500/50 bg-red-500/10",
            )}
          >
            {message.type === "success" ? (
              <Check className="w-4 h-4 text-green-500" />
            ) : (
              <AlertCircle className="w-4 h-4 text-red-500" />
            )}
            <AlertDescription
              className={cn(
                message.type === "success" ? "text-green-500" : "text-red-500",
              )}
            >
              {message.text}
            </AlertDescription>
          </Alert>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {themes.length === 0 ? (
            <Card className="col-span-full">
              <CardContent className="pt-6">
                <p className="text-center text-muted-foreground">
                  {t("themePage.noThemesFound")}
                </p>
              </CardContent>
            </Card>
          ) : (
            themes.map((theme) => (
              <Dialog key={theme.id}>
                <DialogTrigger asChild>
                  <div
                    className={cn(
                      "cursor-pointer transition-all hover:shadow-md rounded-lg overflow-hidden bg-card border",
                      activeTheme === theme.id
                        ? "border-primary border-2"
                        : "border-border",
                    )}
                    onClick={() => setSelectedTheme(theme)}
                  >
                    {theme.preview && (
                      <div className="w-full h-48 overflow-hidden">
                        <img
                          src={`/api/theme/${theme.id}/preview`}
                          alt={`${theme.name} preview`}
                          className="w-full h-full object-cover"
                        />
                      </div>
                    )}
                    <div className="p-4">
                      <div className="flex items-center justify-between gap-2 flex-wrap mb-2">
                        <h3 className="font-semibold text-sm">{theme.name}</h3>
                        {activeTheme === theme.id && (
                          <Badge className="bg-primary text-primary-foreground text-xs">
                            {t("themePage.currentBadge")}
                          </Badge>
                        )}
                      </div>
                      <div className="space-y-1 text-xs text-muted-foreground mb-4">
                        <p>
                          <span className="font-semibold">
                            {t("themePage.author")}:
                          </span>{" "}
                          {theme.author}
                        </p>
                        <p>
                          <span className="font-semibold">
                            {t("themePage.version")}:
                          </span>{" "}
                          {theme.version}
                        </p>
                        {theme.description && (
                          <p className="line-clamp-2">
                            <span className="font-semibold">
                              {t("themePage.themeDescription")}:
                            </span>{" "}
                            {theme.description}
                          </p>
                        )}
                      </div>
                      <div className="flex justify-between items-center">
                        <Button
                          variant="secondary"
                          size="sm"
                          className="flex items-center gap-1 text-xs"
                        >
                          <Eye className="w-3 h-3" />
                          {t("themePage.viewDetails")}
                        </Button>
                        <Button
                          onClick={(e) => {
                            e.stopPropagation();
                            handleApplyTheme(theme.id);
                          }}
                          disabled={
                            activeTheme === theme.id || applying !== null
                          }
                          variant={
                            activeTheme === theme.id ? "secondary" : "default"
                          }
                          size="sm"
                          className="text-xs"
                        >
                          {applying === theme.id && (
                            <Loader2 className="w-3 h-3 mr-1 animate-spin" />
                          )}
                          {activeTheme === theme.id
                            ? t("themePage.applied")
                            : t("themePage.applyTheme")}
                        </Button>
                      </div>
                    </div>
                  </div>
                </DialogTrigger>
                <DialogContent className="sm:max-w-md">
                  {selectedTheme && (
                    <>
                      <DialogHeader>
                        <DialogTitle className="flex items-center justify-between">
                          {selectedTheme.name}
                          {activeTheme === selectedTheme.id && (
                            <Badge className="bg-primary text-primary-foreground">
                              {t("themePage.currentBadge")}
                            </Badge>
                          )}
                        </DialogTitle>
                        <DialogDescription>
                          <div className="space-y-2 text-sm">
                            <p>
                              <span className="font-semibold">
                                {t("themePage.author")}:
                              </span>{" "}
                              {selectedTheme.author}
                            </p>
                            <p>
                              <span className="font-semibold">
                                {t("themePage.version")}:
                              </span>{" "}
                              {selectedTheme.version}
                            </p>
                            {selectedTheme.description && (
                              <p>
                                <span className="font-semibold">
                                  {t("themePage.themeDescription")}:
                                </span>{" "}
                                {selectedTheme.description}
                              </p>
                            )}
                            {selectedTheme.url && (
                              <p>
                                <span className="font-semibold">
                                  {t("themePage.link")}:
                                </span>
                                <a
                                  href={selectedTheme.url}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="text-primary hover:underline break-all"
                                >
                                  {selectedTheme.url}
                                </a>
                              </p>
                            )}
                          </div>
                        </DialogDescription>
                      </DialogHeader>
                      {selectedTheme.preview && (
                        <div className="mt-4">
                          <img
                            src={`/api/theme/${selectedTheme.id}/preview`}
                            alt={`${selectedTheme.name} preview`}
                            className="w-full h-auto rounded-md shadow-sm"
                          />
                        </div>
                      )}
                      <div className="mt-6 flex justify-end">
                        <Button
                          onClick={() => handleApplyTheme(selectedTheme.id)}
                          disabled={
                            activeTheme === selectedTheme.id ||
                            applying !== null
                          }
                          variant={
                            activeTheme === selectedTheme.id
                              ? "secondary"
                              : "default"
                          }
                        >
                          {applying === selectedTheme.id && (
                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
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
            ))
          )}
        </div>

        <Card className="bg-muted border-border">
          <CardHeader>
            <CardTitle className="text-foreground">
              {t("themePage.installationInstructions")}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-foreground space-y-2">
            <p>
              <strong>{t("themePage.method1.title")}</strong>
            </p>
            <p>{t("themePage.method1.step1")}</p>
            <p>{t("themePage.method1.step2")}</p>
            <p>{t("themePage.method1.step3")}</p>
            <p className="mt-4">
              <strong>{t("themePage.method2.title")}</strong>
            </p>
            <p>
              1. {t("themePage.instruction1")}{" "}
              <code className="bg-muted px-2 py-1 rounded">./data/theme/</code>{" "}
              {t("themePage.instruction2")}
            </p>
            <p>
              2. {t("themePage.instruction3")}{" "}
              <code className="bg-muted px-2 py-1 rounded">
                vexgo-theme.json
              </code>{" "}
              {t("themePage.instruction4")}
            </p>
            <p>
              3. {t("themePage.instruction5")}{" "}
              <code className="bg-muted px-2 py-1 rounded">dist/</code>{" "}
              {t("themePage.instruction6")}
            </p>
            <p>4. {t("themePage.instruction7")}</p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
