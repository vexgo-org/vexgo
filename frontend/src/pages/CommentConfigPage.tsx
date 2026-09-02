import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "@/lib/I18nContext";
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
import { Textarea } from "@/components/ui/textarea";
import { ArrowLeft, Save, Shield, Key, Bot } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { configApi } from "@/lib/api";
import type { CommentModerationConfig } from "@/types";

export function CommentConfigPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState<CommentModerationConfig>({
    id: 0,
    manualReviewEnabled: false,
    keywordFilterEnabled: false,
    llmReviewEnabled: false,
    modelProvider: "",
    apiKey: "",
    apiEndpoint: "",
    modelName: "gpt-3.5-turbo",
    moderationPrompt: t("commentConfig.moderationPromptPlaceholder"),
    blockKeywords: "",
    createdAt: "",
    updatedAt: "",
  });

  const loadConfig = useCallback(async () => {
    try {
      const response = await configApi.getCommentModerationConfig();
      setConfig(response.data);
    } catch (error: unknown) {
      console.error("Failed to load comment moderation config:", error);
      toast.error(t("commentConfig.loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadConfig();
  }, [loadConfig]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await configApi.updateCommentModerationConfig(config);
      toast.success(t("commentConfig.saveSuccess"));
    } catch (error: unknown) {
      console.error("Failed to save config:", error);
      const apiError = error as { response?: { data?: { error?: string } } };
      toast.error(
        apiError.response?.data?.error || t("commentConfig.saveFailed"),
      );
    } finally {
      setSaving(false);
    }
  };

  const [testing, setTesting] = useState(false);

  const handleTestConnection = async () => {
    setTesting(true);
    try {
      const response = await configApi.testCommentModeration();
      toast.success(
        `${response.data?.message ?? ""} ${response.data?.response ?? ""}`.trim(),
      );
    } catch (error: unknown) {
      console.error("Failed to test LLM moderation endpoint:", error);
      const apiError = error as { response?: { data?: { error?: string } } };
      toast.error(
        apiError.response?.data?.error || t("commentConfig.testFailed"),
      );
    } finally {
      setTesting(false);
    }
  };

  if (loading) {
    return (
      <div className="container mx-auto py-6">
        <div className="text-center py-8 text-muted-foreground">
          {t("common.loading")}
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto py-6">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => navigate("/admin")}
          >
            <ArrowLeft className="h-5 w-5" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold">{t("commentConfig.title")}</h1>
            <p className="text-muted-foreground">
              {t("commentConfig.description")}
            </p>
          </div>
        </div>
        <Button onClick={handleSave} disabled={saving}>
          <Save className="h-4 w-4 mr-2" />
          {saving ? t("commentConfig.saving") : t("commentConfig.saveConfig")}
        </Button>
      </div>

      <div className="grid gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="h-5 w-5" />
              {t("commentConfig.basicSettings")}
            </CardTitle>
            <CardDescription>
              {t("commentConfig.basicSettingsDesc")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label>{t("commentConfig.manualReview")}</Label>
                <p className="text-sm text-muted-foreground">
                  {t("commentConfig.manualReviewDesc")}
                </p>
              </div>
              <Switch
                checked={config.manualReviewEnabled}
                onCheckedChange={(checked) =>
                  setConfig({ ...config, manualReviewEnabled: checked })
                }
              />
            </div>
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label>{t("commentConfig.keywordFilter")}</Label>
                <p className="text-sm text-muted-foreground">
                  {t("commentConfig.keywordFilterDesc")}
                </p>
              </div>
              <Switch
                checked={config.keywordFilterEnabled}
                onCheckedChange={(checked) =>
                  setConfig({ ...config, keywordFilterEnabled: checked })
                }
              />
            </div>
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <Label>{t("commentConfig.llmReview")}</Label>
                <p className="text-sm text-muted-foreground">
                  {t("commentConfig.llmReviewDesc")}
                </p>
              </div>
              <Switch
                checked={config.llmReviewEnabled}
                onCheckedChange={(checked) =>
                  setConfig({ ...config, llmReviewEnabled: checked })
                }
              />
            </div>
            <p className="text-sm text-muted-foreground">
              {t("commentConfig.switchesHint")}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Key className="h-5 w-5" />
              {t("commentConfig.apiConfig")}
            </CardTitle>
            <CardDescription>
              {t("commentConfig.apiConfigDesc")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="modelProvider">
                {t("commentConfig.modelProvider")}
              </Label>
              <Select
                value={config.modelProvider}
                onValueChange={(value) =>
                  setConfig({ ...config, modelProvider: value })
                }
              >
                <SelectTrigger>
                  <SelectValue
                    placeholder={t("commentConfig.selectModelProvider")}
                  />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="openai">OpenAI</SelectItem>
                  <SelectItem value="azure">Azure OpenAI</SelectItem>
                  <SelectItem value="anthropic">Anthropic</SelectItem>
                  <SelectItem value="custom">{t("common.custom")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="modelName">{t("commentConfig.modelName")}</Label>
              <Input
                id="modelName"
                value={config.modelName}
                onChange={(e) =>
                  setConfig({ ...config, modelName: e.target.value })
                }
                placeholder={t("commentConfig.modelNamePlaceholder")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="apiKey">{t("commentConfig.apiKey")}</Label>
              <Input
                id="apiKey"
                type="password"
                value={config.apiKey}
                onChange={(e) =>
                  setConfig({ ...config, apiKey: e.target.value })
                }
                placeholder={t("commentConfig.apiKeyPlaceholder")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="apiEndpoint">
                {t("commentConfig.apiEndpoint")}
              </Label>
              <Input
                id="apiEndpoint"
                value={config.apiEndpoint}
                onChange={(e) =>
                  setConfig({ ...config, apiEndpoint: e.target.value })
                }
                placeholder={t("commentConfig.apiEndpointPlaceholder")}
              />
            </div>
            <div>
              <Button
                variant="outline"
                onClick={handleTestConnection}
                disabled={testing}
              >
                <Bot className="h-4 w-4 mr-2" />
                {testing
                  ? t("commentConfig.testing")
                  : t("commentConfig.testConnection")}
              </Button>
              <p className="text-sm text-muted-foreground mt-2">
                {t("commentConfig.testConnectionHint")}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bot className="h-5 w-5" />
              {t("commentConfig.moderationRules")}
            </CardTitle>
            <CardDescription>
              {t("commentConfig.moderationRulesDesc")}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="moderationPrompt">
                {t("commentConfig.moderationPrompt")}
              </Label>
              <Textarea
                id="moderationPrompt"
                value={config.moderationPrompt}
                onChange={(e) =>
                  setConfig({ ...config, moderationPrompt: e.target.value })
                }
                rows={6}
                placeholder={t("commentConfig.moderationPromptPlaceholder")}
              />
              <p className="text-sm text-muted-foreground">
                {t("commentConfig.placeholderHint")}
              </p>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="blockKeywords">
                {t("commentConfig.blockKeywords")}
              </Label>
              <Textarea
                id="blockKeywords"
                value={config.blockKeywords}
                onChange={(e) =>
                  setConfig({ ...config, blockKeywords: e.target.value })
                }
                rows={3}
                placeholder={t("commentConfig.blockKeywordsPlaceholder")}
              />
              <p className="text-sm text-muted-foreground">
                {t("commentConfig.blockKeywordsDesc")}
              </p>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
