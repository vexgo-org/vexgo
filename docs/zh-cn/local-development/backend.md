# 后端开发

> **操作指南** —— 运行、配置与扩展 Go 后端（Gin + GORM）。请先读完[本地开发](/zh-cn/local-development/quick-develop)教程，确保内嵌 SPA 与主题已存在。

## 运行服务

```bash
just server                                   # 标准本地运行
just run server -c examples/config.yml        # 加载配置文件
just run server --addr 127.0.0.1 --port 3001 --data ./data
go run backend/cmd/vexgo/main.go server       # 不用 just 的等价写法
```

`backend/cmd/vexgo/main.go` 很薄：经 `cli.Execute` 解析配置后调用 `app.New(cfg)`。真正的装配在 `backend/internal/app`（组合根，负责选择缓存后端、构建加解密器、注入各领域的 `Deps`）。

配置分层（优先级从高到低）：CLI flags > 配置文件（`vexgo server -c`，见 `examples/`）> 环境变量（`.env.example`）> `internal/config/config.go` 中的默认值（`keyDefaults` + `mapstructure` tag；不要再引入按来源各自解析的逻辑）。运行时数据（SQLite 文件、上传文件）默认在 `./data/`。

`vexgo dev` 等价于 `vexgo server` 再加一个仅开发用的 `--theme-dir` 参数；见[通用工作流](/zh-cn/local-development/workflow#主题迭代)。

## 代码在哪里

`backend/internal/<domain>/` 下每个领域都是同样的三层：

| 文件            | 职责                          | 约束                                                                   |
| --------------- | ----------------------------- | ---------------------------------------------------------------------- |
| `handler.go`    | HTTP 解析与响应渲染           | 不得直接碰 GORM；绑定 `types.go` 中的请求类型                          |
| `service.go`    | 业务逻辑                      | 只依赖本领域的 `Repository` 接口                                       |
| `repository.go` | `Repository` 接口 + GORM 实现 | 所有查询都在这里，包括防止 N+1 的批量查询                              |
| `types.go`      | API 线上类型，带 JSON tag     | 只声明一次，供 `swag` + `orval` 消费；handler 可把请求直接交给 service |

配套结构：

- `backend/internal/router` —— `RegisterAPIRoutes` 把各领域路由挂到 `/api` 下（group 级别应用可选 JWT 中间件），依赖聚合在 `router.Deps`。`router_test.go` 锁死了 method+path 全表：增删改路由必须同步更新该表。
- `backend/internal/app` —— 装配所有领域；`backend/cmd/vexgo/main.go` 只做配置解析与调用。
- 不导入其他后端包的叶子包：`config`、`model`、`secrets`、`cache`。`model` 放 GORM 模型与 `Notifier`/`FileRemover` 缝隙（`model/interfaces.go`）；不得导入应用逻辑。
- `backend/internal/public` —— SSR 引擎、主题 serving、嵌入资源。`backend/internal/settings` —— 站点设置与主题管理。
- 跨域调用走消费方声明的接口（`notification` 实现 `model.Notifier`，`upload` 实现 `model.FileRemover`）。`mailer.Service` 是有意的例外：以具体 `*mailer.Service` 注入 `auth` 与 `settings`。
- 每一层都要透传 `context.Context`；handler 传 `c.Request.Context()`。依赖经 `Deps` 结构体显式注入，不用全局变量（唯一的全局可变状态是测试缝隙 `mailer.SetMailCaptureHook`，测试中用 `t.Cleanup` 还原）。

## 实操：新增或修改接口

1. 在领域 `types.go` 中声明请求/响应结构体并写好 JSON tag。
2. 在 handler 上写 `swag` 注解（`@Summary`、`@Param`、`@Success`、`@Failure`、`@Router`）；通用 API 块在 `backend/cmd/vexgo/main.go`。
3. `handler.go` 写解析、`service.go` 写逻辑、`repository.go` 写查询。
4. 在领域 handler 中注册路由，并更新 `backend/internal/router/router_test.go`，让锁定的路由表保持一致。
5. 跑 `just generate` (或者其等价命令，见`justfile`) 刷新 `docs/swagger.json` 与 `frontend/src/api/generated/`；两者禁止手改。然后 `just format && just lint && just test`。

文件上传用 `@Accept multipart/form-data` 加 `@Param <name> formData file true "<desc>"`。注解格式保持 `swag` 干净（`just format` 会跑 `go tool swag fmt backend/`）。

## 实操：测试后端

```bash
go test ./backend/internal/post/...   # 单个领域
go test ./...                         # 全量（just test 会先跑 ensure-dist）
```

Service 测试一般经各领域的 `newTestDB`/`newTestService` 跑内存 SQLite；DB 难以覆盖的注入缝隙用 fake（`model.Notifier`、`model.FileRemover`、`mailer.SetMailCaptureHook`）。`page` 用 fake `Repository`。新增用户可见行为、修 bug、边界条件（空输入、非法输入、权限边界）都要求有测试。

## 排错

| 现象                                   | 可能原因与修复                                                                                                                  |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `go run` 报缺 `dist` / `default-theme` | 干净检出后未构建嵌入产物。跑 `just build-frontend && just build-theme`（`just server` 也会自动跑 `ensure-dist`）。              |
| 配置不生效                             | 高优先级覆盖了低优先级。按 flags > `-c` 文件 > env > 默认值排查；确认 key 在 `keyDefaults` 中存在且 `mapstructure` tag 对得上。 |
| SPA 调新路由 404                       | `@Router` 路径/方法与注册路由不一致，或忘了更新 `router_test.go`。三处对齐后再 `just generate`。                                |
| CI 报 `docs/swagger.json is stale`     | 改了后端注解但没重新生成。跑 `just generate` 并提交结果。                                                                       |

下一步：SPA 循环见[前端开发](/zh-cn/local-development/frontend)，门禁与代码生成见[通用工作流](/zh-cn/local-development/workflow)。
