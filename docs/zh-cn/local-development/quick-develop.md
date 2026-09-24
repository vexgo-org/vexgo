# 本地开发

> 本节带你从零跑通本地完整环境，之后可作为日常后端与前端开发的查阅手册。

完成教程部分后，你将在本地跑起后端 API、管理后台 SPA 与默认主题，并发布一篇测试文章。操作指南部分覆盖你每天都会用到的开发循环。

读者：新手贡献者与有经验的 Go/React 开发者。不要求事先了解 VexGo，操作指南部分最好具备基础 Go 与 TypeScript 知识。

范围：环境准备、后端循环、前端循环、测试/ lint 门禁、API 代码生成、主题联调、排错。生产话题（反向代理、HTTPS、systemd、S3/SSO 完整配置）见[配置](/zh-cn/guides/configuration)与[部署](/zh-cn/guides/deployment)。

内容：

- [后端开发](/zh-cn/local-development/backend)：运行 Go 服务、配置分层、领域目录结构、新增接口。
- [前端开发](/zh-cn/local-development/frontend)：以 HMR 运行管理后台、API 基地址、嵌入式构建。
- [通用工作流](/zh-cn/local-development/workflow)：format/lint/test 门禁、swag + orval 代码生成、主题迭代、排错。

## 环境要求

| 工具 | 版本  | 说明                                     |
| ---- | ----- | ---------------------------------------- |
| Go   | 1.26+ | 后端、`go test`、`go tool` 方式的 `swag` |
| bun  | 1.3   | 前端依赖、开发服务器、构建、`orval`      |
| just | 最新  | 推荐；下文所有命令都假设已安装           |
| git  | 任意  | 拉取源码与独立默认主题仓库               |

Nix flake 可通过 `nix develop` 提供上述全部工具（已检入的 `.envrc` 会让 direnv 自动激活）。

默认本地地址与账号：

| 项目            | 取值                                                 |
| --------------- | ---------------------------------------------------- |
| 后端 + 公开站点 | `http://127.0.0.1:3001`                              |
| 管理后台 SPA    | `http://127.0.0.1:3001/admin/`                       |
| 超级管理员      | `admin@example.com` / `password`（首次登录后请修改） |

## 教程

### 第 1 步：克隆并安装依赖

```bash
git clone https://github.com/vexgo-org/vexgo.git
cd vexgo

# 安装后端依赖
go mod download

# 安装其他依赖
cd frontend
bun install
cd ..
```

> 你拉取了 Go 模块依赖与管理后台 SPA 依赖。此时还没有任何构建产物。

### 第 2 步：一次性构建被嵌入的前端

需要构建两个前端：管理后台 SPA 和默认主题。

1. 管理后台前端

```bash
bun run --cwd frontend build
```

2. 默认主题前端

```bash
# 另找一个路径执行
git clone https://github.com/vexgo-org/vexgo-default-theme.git
cd vexgo-default-theme
bun install
bun run build

mkdir -p path/to/vexgo/backend/internal/public/default-theme/
cp -r dist/* path/to/vexgo/backend/internal/public/default-theme/
```

也可以手动把 `vexgo-default-theme/dist/` 下的内容复制到 `vexgo/backend/internal/public/default-theme/`。

如果 `just` 可用（例如在 Linux 上），可以用它作为快捷方式：

```bash
just build-frontend
just build-theme
```

`just build-frontend` 执行 `bun run --cwd frontend build`，把管理后台产物写到 `backend/internal/public/dist/`（gitignored）。`just build-theme` 按固定 ref 拉取 `vexgo-org/vexgo-default-theme` 并构建到 `backend/internal/public/default-theme/`（gitignored）。两处输出都会经 `go:embed` 打进 Go 二进制，因此干净检出后必须先执行这一步，否则后端无法启动（`just server` 会先跑 `ensure-dist` 做同样保护）。

之后除非清理了上述目录，否则无需重复执行。

### 第 3 步：启动后端

```bash
go run backend/cmd/vexgo/main.go server
```

或者使用`just`

```bash
just server
```

它等价于 `go run backend/cmd/vexgo/main.go server` 加 `ensure-dist` 保护。打开 `http://127.0.0.1:3001`，应能看到默认主题首页，背后是 `./data/` 下新建的 SQLite 数据库。

常用变体：

```bash
go run backend/cmd/vexgo/main.go server -c examples/config.yml   # 加载配置文件
go run backend/cmd/vexgo/main.go server --port 8080 --data /tmp/vexgo-data
go run backend/cmd/vexgo/main.go --version                       # 仅根命令支持的版本标志
```

裸 `vexgo` 只打印帮助；`vexgo server --help` 列出 `--config/-c`、`--addr/-a`、`--port/-p`、`--data/-d`。

### 第 4 步：登录并修改密码

1. 打开 `http://127.0.0.1:3001/admin/login`。
2. 用 `admin@example.com` / `password` 登录。
3. 打开个人资料页，立即修改密码。

`/login` 这类旧顶层 URL 会 301 跳到对应的 `/admin/` 路径，并保留 query string。

### 第 5 步：发布测试文章并验证

1. 打开 `http://127.0.0.1:3001/admin/write`。
2. 新建标题为 `Hello, VexGo!` 的文章并发布。
3. 确认它出现在 `http://127.0.0.1:3001` 首页。

成功的标准是：公开站点可渲染、后台可登录、新文章能从 SPA 经 API 写入 SQLite，再经主题读出来。

## 日常循环速查

| 我想……                     | 执行                                      | 详见                                                                |
| -------------------------- | ----------------------------------------- | ------------------------------------------------------------------- |
| 运行后端 + 内嵌 SPA/主题   | `just server`                             | [后端开发](/zh-cn/local-development/backend)                        |
| 以 HMR 运行 SPA 开发服务器 | 在 `frontend/` 下 `bun run dev`           | [前端开发](/zh-cn/local-development/frontend)                       |
| 不重建即迭代主题           | `just theme ../vexgo-default-theme/dist/` | [通用工作流](/zh-cn/local-development/workflow#主题迭代)            |
| PR 前跑 format/lint/test   | `just format && just lint && just test`   | [通用工作流](/zh-cn/local-development/workflow#pr-前先过门禁)       |
| 重新生成类型化 API 客户端  | `just generate`                           | [通用工作流](/zh-cn/local-development/workflow#重新生成-api-客户端) |

配置优先级为 flags > 配置文件 > 环境变量（`.env`，见 `.env.example`）> 默认值；运行时数据在 `./data/`。全量 key 见[配置](/zh-cn/guides/configuration)。

## 下一步？

- 后端内部结构：[后端开发](/zh-cn/local-development/backend)。
- SPA 内部结构：[前端开发](/zh-cn/local-development/frontend)。
- 门禁、代码生成、主题、排错：[通用工作流](/zh-cn/local-development/workflow)。
- 系统总览：[架构](/zh-cn/concepts/architecture)。
- 全量接口：[API 参考](api.html)。
