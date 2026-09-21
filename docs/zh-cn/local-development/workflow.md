# 通用工作流

> **操作指南** —— VexGo 本地开发的共享门禁、API 代码生成、主题迭代与排错清单。假设[本地开发](/zh-cn/local-development/quick-develop)教程已经跑通。

## PR 前先过门禁

```bash
just format   # gofumpt -w -extra . && prettier --write ... && go tool swag fmt backend/
just lint     # golangci-lint、deadcode、prettier --check、gofumpt 检查、oxlint、gopls、swag fmt、OpenAPI 新鲜度
just test     # ensure-dist + go test ./...
just build    # 管理后台 SPA + 默认主题 + 后端
```

开 PR 前的最低要求（见 `CONTRIBUTING.md`）：

```bash
just format
just lint
bun run build   # 前端类型门禁（tsc -b）+ 打包
go build ./...
go test ./...
```

`just lint` 会在 `docs/swagger.json` 过期或 `swag` 注解格式不对时失败。若无 `just`，按 `justfile` 中的对应 recipe 手动执行即可。

## 重新生成 API 客户端

类型化的前端 API 客户端是生成的，不是手写的：

后端 handler 上的 `swag` 注解 → `docs/swagger.json` → `orval` → `frontend/src/api/generated/`

```bash
just generate            # 先重新生成 docs/swagger.json，再生成 TS 客户端
just check-openapi-fresh # CI 门禁：docs/swagger.json 过期即失败
```

规则：

- 改后端类型/注解，再重新生成；禁止手改 `docs/swagger.json` 或 `frontend/src/api/generated/`。
- 文件上传用 `@Accept multipart/form-data` 加 `@Param <name> formData file true "<desc>"`。
- 增删改路由时同步更新 `backend/internal/router/router_test.go`，并让 `@Router` 路径/方法与注册路由完全一致，否则生成的客户端会调到 404 URL。
- 前端统一经 `@/api/generated/endpoints` 的 `getVexGoAPI()` 调用 API。

## 主题迭代

`vexgo dev` 与 `vexgo server` 一样启动服务，多一个仅开发用的 `--theme-dir` 参数，用本地目录覆盖当前主题：

```bash
just theme ../vexgo-default-theme/dist/
# = go run backend/cmd/vexgo/main.go dev --theme-dir ../vexgo-default-theme/dist/
```

用它迭代主题时无需重建内嵌默认主题，也无需上传。`server` 会拒绝 `--theme-dir`（见 `backend/internal/cli/cli_test.go`）。主题完整写法见[主题开发](/zh-cn/local-development/theme-development)与[主题系统](/zh-cn/concepts/theming)。

模板解析陷阱：HTML 属性里的 `go()` 表达式不得含双引号 —— React 会把它转义成 `&quot;`，导致模板解析失败。请用 helper（`date`、`truncate`、`userURL`、`categoryURL`）代替。

## 排错

| 现象                                 | 修复                                                                                      |
| ------------------------------------ | ----------------------------------------------------------------------------------------- |
| 干净检出后后端起不来                 | 跑 `just build-frontend && just build-theme`；两处输出是 gitignored，但 `go:embed` 必需。 |
| 改完前端 `just server` 看不到        | 重建：`just build-frontend`，再刷新 `/admin/`。                                           |
| 端口被占用                           | 换 `--port`（如 `just run server --port 8080`），SPA 开发服务器的 `VITE_API_URL` 同步改。 |
| `docs/swagger.json is stale`         | 跑 `just generate` 并提交结果。                                                           |
| `Swag annotations are not formatted` | 跑 `just format`（内含 `go tool swag fmt backend/`）。                                    |
| Prettier/oxlint/gofumpt 失败         | 先跑 `just format`，再跑 `just lint` 看剩下的真问题。                                     |

## 相关阅读

- [安装](/zh-cn/guides/installation) —— 全量安装方式，含源码编译。
- [配置](/zh-cn/guides/configuration) —— 配置文件、环境变量、数据库。
- [部署](/zh-cn/guides/deployment) —— 反向代理、HTTPS、systemd、生产加固。
- [主题开发](/zh-cn/local-development/theme-development)、[主题系统](/zh-cn/concepts/theming)、[主题模板参考](/zh-cn/reference/theme-templates)。
- [架构](/zh-cn/concepts/architecture)、[API 参考](api.html)。
- `CONTRIBUTING.md` —— 工作流、issue/PR/commit 规范、Definition of Done。
