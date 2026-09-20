import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "@/lib/I18nContext";
import { getVexGoAPI } from "@/api/generated/endpoints";
import { unwrap } from "@/lib/api";

import type { AIConfig, AIModel } from "@/types";
import type { AxiosError } from "axios";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Save, TestTube, RefreshCw } from "lucide-react";
import { SkeletonForm } from "@/components/ui/skeleton";
import { PageHeader } from "@/components/PageHeader";
import { toast } from "sonner";

interface ApiErrorResponse {
  error?: string;
}

export function AISettingsPage() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [fetchingModels, setFetchingModels] = useState(false);
  const [config, setConfig] = useState<AIConfig>({
    id: "",
    enabled: false,
    provider: "openai",
    apiEndpoint: "",
    apiKey: "",
    modelName: "gpt-3.5-turbo",
    createdAt: "",
    updatedAt: "",
  });
  const [models, setModels] = useState<AIModel[]>([]);

  const loadConfig = useCallback(async () => {
    try {
      const response = await unwrap(getVexGoAPI().getConfigAi());
      setConfig(response);
    } catch (error: unknown) {
      console.error("Failed to load AI config:", error);
      toast.error(t("aiSettings.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const fetchModels = async () => {
    if (!config.apiKey || !config.apiEndpoint) {
      toast.warning(t("aiSettings.configRequired"));
      return;
    }

    setFetchingModels(true);
    try {
      const response = await unwrap(getVexGoAPI().getConfigAiModels());
      setModels(
        (response.models || []).map((m) => ({
          id: m,
          object: "model" as const,
          created: 0,
          owned_by: "",
        })),
      );
      toast.success(t("aiSettings.testSuccess"));
    } catch (error: unknown) {
      console.error("Failed to fetch the model list:", error);
      const axiosError = error as AxiosError<ApiErrorResponse>;
      const errorMessage =
        axiosError.response?.data?.error || t("common.unknownError");
      toast.error(t("aiSettings.testFailed") + ": " + errorMessage);
      setModels([]);
    } finally {
      setFetchingModels(false);
    }
  };

  const handleSave = async () => {
    if (!(config.apiEndpoint || "").trim()) {
      toast.error(t("aiSettings.apiEndpoint") + t("common.required"));
      return;
    }
    if (!(config.apiKey || "").trim()) {
      toast.error(t("aiSettings.apiKey") + t("common.required"));
      return;
    }
    if (!(config.modelName || "").trim()) {
      toast.error(t("aiSettings.selectModel"));
      return;
    }

    setSaving(true);
    try {
      await unwrap(getVexGoAPI().putConfigAi(config));
      toast.success(t("aiSettings.saveSuccess"));
    } catch (error: unknown) {
      console.error("Failed to save AI config:", error);
      const axiosError = error as AxiosError<ApiErrorResponse>;
      const errorMessage =
        axiosError.response?.data?.error || t("common.unknownError");
      toast.error(t("aiSettings.saveFailed") + ": " + errorMessage);
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!config.enabled) {
      toast.error(t("aiSettings.enableAI"));
      return;
    }
    if (!(config.apiKey || "").trim()) {
      toast.error(t("common.save") + " " + t("aiSettings.apiKey"));
      return;
    }

    setTesting(true);
    try {
      const response = await unwrap(getVexGoAPI().postConfigAiTest());
      toast.success(t("aiSettings.testSuccess") + "!");
      console.log("AI Response:", response.response);
    } catch (error: unknown) {
      console.error("Failed to test the AI connection:", error);
      const axiosError = error as AxiosError<ApiErrorResponse>;
      const errorMessage =
        axiosError.response?.data?.error || t("common.unknownError");
      toast.error(t("aiSettings.testFailed") + ": " + errorMessage);
    } finally {
      setTesting(false);
    }
  };

  if (loading) {
    return (
      <div className="mx-auto max-w-3xl space-y-6">
        <PageHeader
          title={t("aiSettings.title")}
          description={t("aiSettings.description")}
        />
        <SkeletonForm rows={5} />
      </div>
    );
  }

  // Get the currently selected model info
  const selectedModel = models.find((m) => m.id === config.modelName);

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <PageHeader
        title={t("aiSettings.title")}
        description={t("aiSettings.description")}
      />

      <Card>
        <CardHeader>
          <CardTitle>{t("aiSettings.baseSettings")}</CardTitle>
          <CardDescription>{t("commentConfig.apiConfigDesc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* Enable Switch */}
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="enabled">{t("aiSettings.enableAI")}</Label>
              <p className="text-sm text-muted-foreground">
                {t("aiSettings.enableAIDesc")}
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

          {/* Provider */}
          <div className="space-y-2">
            <Label htmlFor="provider">{t("aiSettings.provider")}</Label>
            <Select
              value={config.provider}
              onValueChange={(value) =>
                setConfig({ ...config, provider: value })
              }
              disabled={saving}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("aiSettings.selectProvider")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="openai">OpenAI</SelectItem>
                <SelectItem value="custom">
                  {t("common.custom")} (OpenAI {t("common.compatible")})
                </SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              {t("aiSettings.supportedProviders")}
            </p>
          </div>

          {/* API Endpoint */}
          <div className="space-y-2">
            <Label htmlFor="apiEndpoint">{t("aiSettings.apiEndpoint")} *</Label>
            <Input
              id="apiEndpoint"
              value={config.apiEndpoint}
              onChange={(e) =>
                setConfig({ ...config, apiEndpoint: e.target.value })
              }
              placeholder={t("aiSettings.apiEndpointPlaceholder")}
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              {t("aiSettings.apiBaseUrlExample")}
            </p>
          </div>

          {/* API Key */}
          <div className="space-y-2">
            <Label htmlFor="apiKey">
              {t("aiSettings.apiKey")}{" "}
              {config.enabled && <span className="text-destructive">*</span>}
            </Label>
            <Input
              id="apiKey"
              type="password"
              value={config.apiKey}
              onChange={(e) => setConfig({ ...config, apiKey: e.target.value })}
              placeholder={
                config.enabled
                  ? t("aiSettings.apiKeyPlaceholder")
                  : t("common.optional")
              }
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              {t("aiSettings.apiKeyNote")}
            </p>
          </div>

          {/* Model Name */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="modelName">
                {t("aiSettings.modelName")}{" "}
                {config.enabled && <span className="text-destructive">*</span>}
              </Label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={fetchModels}
                disabled={fetchingModels || !config.enabled || !config.apiKey}
              >
                <RefreshCw
                  className={`size-4 ${fetchingModels ? "animate-spin" : ""}`}
                />
                {fetchingModels
                  ? t("aiSettings.fetchingModels")
                  : t("aiSettings.fetchModels")}
              </Button>
            </div>

            {models.length > 0 ? (
              <Select
                value={config.modelName}
                onValueChange={(value) =>
                  setConfig({ ...config, modelName: value })
                }
                disabled={saving}
              >
                <SelectTrigger>
                  {/* `Select.Value` renders its children as a render function,
                    not as nodes — a `ReactNode` child here would be ignored. */}
                  <SelectValue placeholder={t("aiSettings.selectModel")}>
                    {(value: string) => {
                      const model = models.find((item) => item.id === value);
                      if (!model) return value;
                      return (
                        <span className="flex min-w-0 items-center gap-2">
                          <span className="truncate">{model.id}</span>
                          <span className="shrink-0 text-xs text-muted-foreground">
                            ({model.owned_by})
                          </span>
                        </span>
                      );
                    }}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {models.map((model) => (
                    <SelectItem key={model.id} value={model.id}>
                      <div className="flex flex-col">
                        <span>{model.id}</span>
                        <span className="text-xs text-muted-foreground">
                          {t("aiSettings.provider")}: {model.owned_by}
                        </span>
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                id="modelName"
                value={config.modelName}
                onChange={(e) =>
                  setConfig({ ...config, modelName: e.target.value })
                }
                placeholder={t("aiSettings.modelNamePlaceholder")}
                disabled={saving}
              />
            )}

            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                {t("aiSettings.availableModelCount", {
                  count: models.length,
                })}
              </span>
              {selectedModel && (
                <span className="text-success">
                  {t("aiSettings.selectedModel", { model: selectedModel.id })}
                </span>
              )}
            </div>
          </div>

          {/* Action buttons */}
          <div className="flex gap-3 pt-4 border-t">
            <Button onClick={handleSave} disabled={saving} className="flex-1">
              <Save className="size-4" />
              {saving
                ? t("aiSettings.saving")
                : t("generalSettings.saveSettings")}
            </Button>
            <Button
              variant="outline"
              onClick={handleTest}
              disabled={testing || !config.enabled}
              className="flex-1"
            >
              <TestTube className="size-4" />
              {testing ? t("aiSettings.testing") : t("aiSettings.testAI")}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Reference prose, set as an appendix: a micro-caps heading per group
       * and the facts as plain lines, rather than bullet glyphs under bold
       * labels. */}
      <Card>
        <CardHeader>
          <CardTitle>{t("aiSettings.helpInfo.title")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <section className="space-y-1">
            <h3 className="eyebrow">{t("aiSettings.helpInfo.openai.title")}</h3>
            <p className="text-sm leading-relaxed">
              {t("aiSettings.helpInfo.openai.baseUrl")}
            </p>
            <p className="text-sm leading-relaxed">
              {t("aiSettings.helpInfo.openai.apiKey")}
            </p>
            <p className="text-sm leading-relaxed">
              {t("aiSettings.helpInfo.openai.supportedModels")}
            </p>
          </section>
          <section className="space-y-1 border-t border-border pt-4">
            <h3 className="eyebrow">{t("aiSettings.helpInfo.custom.title")}</h3>
            <p className="text-sm leading-relaxed">
              {t("aiSettings.helpInfo.custom.compatible")}
            </p>
            <p className="text-sm leading-relaxed">
              {t("aiSettings.helpInfo.custom.examples")}
            </p>
            <p className="text-sm leading-relaxed">
              {t("aiSettings.helpInfo.custom.endpoint")}
            </p>
          </section>
          <section className="space-y-1 border-t border-border pt-4">
            <h3 className="eyebrow">{t("aiSettings.helpInfo.test.title")}</h3>
            <p className="text-sm leading-relaxed text-muted-foreground">
              {t("aiSettings.helpInfo.test.description")}
            </p>
          </section>
        </CardContent>
      </Card>
    </div>
  );
}
