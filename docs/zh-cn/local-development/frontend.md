# 前端开发

> 运行、修改与重建管理后台 SPA（React + TypeScript，Vite、Tailwind、shadcn/ui）的实操指南。请先读完[本地开发](/zh-cn/local-development/quick-develop)教程，确保 `bun install` 已完成且后端在运行。

SPA 全部位于 `/admin/` 下；公开页面由主题服务端渲染，SPA 绝不能占用站点根。`bun run build` 把产物写到 `backend/internal/public/dist/`（gitignored），Go 二进制再将其 embed 并对外 serving。改完前端必须重建，后端才会生效；Vite 开发服务器不会经过后端代理。

## 运行开发服务器

对于本项目，推荐更改前端后重新构建管理主题，并重新运行后端。

```bash
bun run --cwd frontend build
go run backend/cmd/vexgo/main.go server
```

也可以启动一个后端后，前端开启开发服务器。

```bash
cd frontend
bun install
bun run dev
```

这会启动带 HMR 的 Vite（默认 `http://localhost:5173`）。后端另开一个终端跑 `just server`（默认 `http://127.0.0.1:3001`），两者配合联调。

API 基地址来自 `VITE_API_URL`（见 `frontend/.env.example`）：

```bash
VITE_API_URL=http://localhost:3001/api
```

解析逻辑在 `frontend/src/api/customAxios.ts`：`import.meta.env.VITE_API_URL || "/api"`。共享 axios 实例会附加 `localStorage` 中的 token，并在非登录接口遇到 401 时跳到 `/admin/login`。开发态 SPA 调不通 API 时，先查 `VITE_API_URL`（端口错、缺 `/api` 后缀、后端没启动）。

## 代码在哪里

| 路径                     | 内容                                                                                                |
| ------------------------ | --------------------------------------------------------------------------------------------------- |
| `src/pages/`             | 路由页面                                                                                            |
| `src/components/`        | 业务组件；`src/components/ui/` 放 shadcn/ui 基元                                                    |
| `src/api/generated/`     | 生成的 API 客户端；禁止手改，用 `just generate`                                                     |
| `src/locales/`           | i18n 文案；`en-US.ts` 与 `zh-CN.ts` 保持同步                                                        |
| `src/lib/`、`src/types/` | 共享逻辑与共享类型                                                                                  |
| `vite.config.ts`         | `base: "/admin/"`、`outDir: ../backend/internal/public/dist`、`emptyOutDir: true`、`manifest: true` |

约定：prettier + oxlint（`frontend/.oxlintrc.json`；`typescript/no-explicit-any` 是 error）；`@` 映射到 `frontend/src/`；共享类型放 `src/types/`、复用逻辑放 `src/lib/`；代码与注释用英文。调 API 统一经 `@/api/generated/endpoints` 的 `getVexGoAPI()`。

## 实操：改完 SPA 并在生产模式验证

```bash
cd frontend
bun run dev        # 用 HMR 对着 just server 迭代
bun run build      # tsc -b + vite build + 拷贝 theme manifest
```

仓库根下 `just build-frontend` 是同一构建。前端重建后一般无需重启后端，刷新 `/admin/` 即可：`emptyOutDir: true` 加 manifest 就是为了避免 `dist` 里残留旧 hash chunk 导致后端 serve 到过期 JS：不要关掉它们。

## 实操：PR 前验证

```bash
cd frontend
bun run lint       # oxlint
bun run build      # 类型门禁（tsc -b）+ 生产包
```

前端没有测试框架：`bun run build` 即类型门禁，行为变化在开发服务器里手动验证。仓库级门禁（`just format`、`just lint`、`go test ./...`）见[通用工作流](/zh-cn/local-development/workflow#pr-前先过门禁)。

## 排错

| 现象                                         | 可能原因与修复                                                                                                                        |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| SPA 反复跳登录 / 401                         | 后端没启动、`VITE_API_URL` 配错、或 token 过期。先确认 `http://localhost:3001/api` 可达，修正 `.env`，必要时清 `localStorage` token。 |
| `bun run dev` 可见改动，`just server` 看不到 | 符合预期：后端 serve 的是嵌入构建。跑 `bun run build`（或 `just build-frontend`）后再刷新 `/admin/`。                                 |
| 构建通过但后端还是旧 JS                      | `backend/internal/public/dist/` 有残留。确认 `emptyOutDir: true` 未被关掉后重建；不要手工删单个 chunk。                               |
| `any` / 未使用 import 报 lint 错             | `typescript/no-explicit-any` 是 error。模块边界写显式类型，删掉无用 import。                                                          |
| 某种语言缺文案                               | `en-US.ts` 与 `zh-CN.ts` 漂移了。两边补齐同一个 key。                                                                                 |

下一步：API 侧见[后端开发](/zh-cn/local-development/backend)，代码生成与门禁见[通用工作流](/zh-cn/local-development/workflow)。
