import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { configApi } from '@/lib/api';
import type { AIConfig, AIModel } from '@/types';
import type { AxiosError } from 'axios';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ArrowLeft, Cpu, Save, TestTube, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';

interface ApiErrorResponse {
  error?: string;
}

export function AISettingsPage() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [fetchingModels, setFetchingModels] = useState(false);
  const [config, setConfig] = useState<AIConfig>({
    id: '',
    enabled: false,
    provider: 'openai',
    apiEndpoint: '',
    apiKey: '',
    modelName: 'gpt-3.5-turbo',
    createdAt: '',
    updatedAt: ''
  });
  const [models, setModels] = useState<AIModel[]>([]);

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response = await configApi.getAIConfig();
      setConfig(response.data);
    } catch (error: unknown) {
      console.error('加载 AI 配置失败:', error);
      toast.error('加载配置失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchModels = async () => {
    if (!config.apiKey || !config.apiEndpoint) {
      toast.warning('请先填写 API 密钥和端点');
      return;
    }

    setFetchingModels(true);
    try {
      const response = await configApi.getAIModels();
      setModels(response.data.models);
      toast.success(`成功获取 ${response.data.models.length} 个可用模型`);
    } catch (error: unknown) {
      console.error('获取模型列表失败:', error);
      const axiosError = error as AxiosError<ApiErrorResponse>;
      const errorMessage = axiosError.response?.data?.error || '未知错误';
      toast.error('获取模型列表失败: ' + errorMessage);
      setModels([]);
    } finally {
      setFetchingModels(false);
    }
  };

  const handleSave = async () => {
    if (!config.apiEndpoint.trim()) {
      toast.error('请输入 API 端点');
      return;
    }
    if (!config.apiKey.trim()) {
      toast.error('请输入 API 密钥');
      return;
    }
    if (!config.modelName.trim()) {
      toast.error('请选择或输入模型名称');
      return;
    }

    setSaving(true);
    try {
      await configApi.updateAIConfig(config);
      toast.success('AI 配置已保存');
    } catch (error: unknown) {
      console.error('保存 AI 配置失败:', error);
      const axiosError = error as AxiosError<ApiErrorResponse>;
      const errorMessage = axiosError.response?.data?.error || '未知错误';
      toast.error('保存失败: ' + errorMessage);
    } finally {
      setSaving(false);
    }
  };

  const handleTest = async () => {
    if (!config.enabled) {
      toast.error('请先启用 AI');
      return;
    }
    if (!config.apiKey.trim()) {
      toast.error('请先保存 API 密钥');
      return;
    }

    setTesting(true);
    try {
      const response = await configApi.testAI();
      toast.success('AI 连接测试成功！');
      console.log('AI 回复:', response.data.response);
    } catch (error: unknown) {
      console.error('测试 AI 连接失败:', error);
      const axiosError = error as AxiosError<ApiErrorResponse>;
      const errorMessage = axiosError.response?.data?.error || '未知错误';
      toast.error('测试失败: ' + errorMessage);
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

  // 获取当前选中的模型信息
  const selectedModel = models.find(m => m.id === config.modelName);

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* 头部 */}
      <div className="mb-6">
        <Button 
          variant="ghost" 
          size="sm" 
          onClick={() => navigate('/admin')}
          className="mb-4"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          返回管理后台
        </Button>
        <h1 className="text-3xl font-bold flex items-center gap-2">
          <Cpu className="w-8 h-8" />
          大模型设置
        </h1>
        <p className="text-muted-foreground mt-2">
          配置大模型 API，用于 AI 辅助功能
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>大模型配置</CardTitle>
          <CardDescription>
            填写 OpenAI 兼容的大模型 API 信息以启用 AI 功能
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          {/* 启用开关 */}
          <div className="flex items-center justify-between">
            <div className="space-y-0.5">
              <Label htmlFor="enabled">启用大模型</Label>
              <p className="text-sm text-muted-foreground">
                开启后系统将使用大模型 API 进行 AI 辅助功能
              </p>
            </div>
            <Switch
              id="enabled"
              checked={config.enabled}
              onCheckedChange={(checked) => setConfig({ ...config, enabled: checked })}
            />
          </div>

          {/* 提供商 */}
          <div className="space-y-2">
            <Label htmlFor="provider">大模型提供商</Label>
            <Select
              value={config.provider}
              onValueChange={(value) => setConfig({ ...config, provider: value })}
              disabled={saving}
            >
              <SelectTrigger>
                <SelectValue placeholder="选择提供商" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="openai">OpenAI</SelectItem>
                <SelectItem value="custom">自定义 (OpenAI 兼容)</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              目前支持 OpenAI 及兼容接口
            </p>
          </div>

          {/* API 端点 */}
          <div className="space-y-2">
            <Label htmlFor="apiEndpoint">API 端点 URL *</Label>
            <Input
              id="apiEndpoint"
              value={config.apiEndpoint}
              onChange={(e) => setConfig({ ...config, apiEndpoint: e.target.value })}
              placeholder="例如: https://api.openai.com/v1/chat/completions"
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              OpenAI 官方 Base Url: https://api.openai.com/v1
            </p>
          </div>

          {/* API 密钥 */}
          <div className="space-y-2">
            <Label htmlFor="apiKey">
              API 密钥 {config.enabled && <span className="text-red-500">*</span>}
            </Label>
            <Input
              id="apiKey"
              type="password"
              value={config.apiKey}
              onChange={(e) => setConfig({ ...config, apiKey: e.target.value })}
              placeholder={config.enabled ? "请输入 API 密钥" : "如需启用请填写"}
              disabled={saving}
            />
            <p className="text-xs text-muted-foreground">
              注意: API 密钥仅用于认证，请妥善保管
            </p>
          </div>

          {/* 模型名称 */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="modelName">
                模型名称 {config.enabled && <span className="text-red-500">*</span>}
              </Label>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={fetchModels}
                disabled={fetchingModels || !config.enabled || !config.apiKey}
              >
                <RefreshCw className={`w-4 h-4 mr-2 ${fetchingModels ? 'animate-spin' : ''}`} />
                {fetchingModels ? '获取中...' : '获取模型列表'}
              </Button>
            </div>
            
            {models.length > 0 ? (
              <Select
                value={config.modelName}
                onValueChange={(value) => setConfig({ ...config, modelName: value })}
                disabled={saving}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择模型">
                    {selectedModel ? (
                      <div className="flex items-center gap-2">
                        <span>{selectedModel.id}</span>
                        <span className="text-xs text-muted-foreground">
                          ({selectedModel.owned_by})
                        </span>
                      </div>
                    ) : (
                      config.modelName
                    )}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {models.map((model) => (
                    <SelectItem key={model.id} value={model.id}>
                      <div className="flex flex-col">
                        <span>{model.id}</span>
                        <span className="text-xs text-muted-foreground">
                          提供商: {model.owned_by}
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
                onChange={(e) => setConfig({ ...config, modelName: e.target.value })}
                placeholder="例如: gpt-3.5-turbo, gpt-4"
                disabled={saving}
              />
            )}
            
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                {models.length > 0
                  ? `共 ${models.length} 个可用模型`
                  : '点击"获取模型列表"从API获取可用模型'
                }
              </span>
              {selectedModel && (
                <span className="text-green-600">
                  已选择: {selectedModel.id}
                </span>
              )}
            </div>
          </div>

          {/* 操作按钮 */}
          <div className="flex gap-3 pt-4 border-t">
            <Button 
              onClick={handleSave} 
              disabled={saving}
              className="flex-1"
            >
              <Save className="w-4 h-4 mr-2" />
              {saving ? '保存中...' : '保存配置'}
            </Button>
            <Button 
              variant="outline" 
              onClick={handleTest}
              disabled={testing || !config.enabled}
              className="flex-1"
            >
              <TestTube className="w-4 h-4 mr-2" />
              {testing ? '测试中...' : '测试连接'}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 帮助信息 */}
      <Card className="mt-6">
        <CardHeader>
          <CardTitle className="text-base">配置说明</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <div>
            <strong>OpenAI 官方 API:</strong>
            <ul className="list-disc list-inside ml-4 mt-1 space-y-1">
              <li>Base Url: https://api.openai.com/v1</li>
              <li>需要有效的 API 密钥（从 OpenAI 平台获取）</li>
              <li>支持的模型: gpt-3.5-turbo, gpt-4, gpt-4-turbo-preview 等</li>
            </ul>
          </div>
          <div className="pt-2">
            <strong>自定义兼容接口:</strong>
            <ul className="list-disc list-inside ml-4 mt-1 space-y-1">
              <li>支持任何 OpenAI 兼容的 API 接口</li>
              <li>例如: 本地部署的 Ollama, vLLM, 或其他中转服务</li>
              <li>确保端点 URL 指向 chat/completions 接口</li>
            </ul>
          </div>
          <div className="pt-2">
            <strong>测试功能:</strong>
            <p className="text-muted-foreground mt-1">
              点击"测试连接"按钮，系统会发送一个简单的测试请求到 AI API，验证配置是否正确。
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
