# VexGo 文档

> VexGo 是一个用 Go 编写的自托管博客 CMS。一个二进制文件同时提供 JSON API、服务端渲染的公开页面和管理面板，默认使用 SQLite。

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

文档分为五个板块。

## 快速开始

教程：跟着做，几分钟内跑起一个可用的实例。

- [快速开始](/zh-cn/getting-started)：安装、登录并发布你的第一篇文章。

## 指南

针对具体任务的操作指南。

- [安装](/zh-cn/guides/installation)：通过 Docker、Docker Compose、Nix、二进制包或源码编译安装 VexGo。
- [配置](/zh-cn/guides/configuration)：配置服务器、数据库、SSO、S3 存储等。
- [部署](/zh-cn/guides/deployment)：在生产环境通过反向代理 + HTTPS 运行 VexGo。

## 本地开发

先跑通本地环境，再按日常循环查阅。

- [快速开始](/zh-cn/local-development/quick-develop)：约 10 分钟第一次跑通，涵盖后端、管理后台与默认主题。
- [后端](/zh-cn/local-development/backend)：运行、配置与扩展 Go 后端。
- [前端](/zh-cn/local-development/frontend)：运行、修改与重建管理后台 SPA。
- [主题开发](/zh-cn/local-development/theme-development)：编写、打包并安装自定义主题。
- [通用工作流](/zh-cn/local-development/workflow)：format/lint/test 门禁、API 代码生成与排错。

## 概念

VexGo 的内部设计。

- [架构](/zh-cn/concepts/architecture)：后端结构、角色与权限、内容审核、SSO。
- [主题系统](/zh-cn/concepts/theming)：主题如何变成服务端渲染的页面。

## 参考

精确的技术细节。

- [配置参考](/zh-cn/reference/configuration)：所有 CLI 参数、环境变量和配置文件键。
- [主题模板参考](/zh-cn/reference/theme-templates)：全部主题文件、模板上下文字段和辅助函数。
- [API 参考](api.html)：全部 REST 端点、请求/响应格式和错误码。

---

## 特性

- 基于 React 的管理面板，用于内容管理
- Go 和 Gin 后端
- 基于 JWT 的身份认证，支持五种角色（`guest` / `contributor` / `author` / `admin` / `super_admin`）
- Markdown 编辑器，支持分类、标签、草稿、点赞和评论
- 评论审核：人工审核、关键词过滤、大模型审核三个独立开关，LLM 故障时转入待审队列（fail-closed）
- 文件存储在本地磁盘，或任意 S3 兼容服务
- 服务端渲染主题，可在管理面板切换和上传
- 站内通知，用于点赞、评论等事件
- 支持 GitHub、Google 及任意 OpenID Connect 提供商登录
- 自托管，数据和部署都在你的控制之下

## 技术栈

| 层次   | 技术                                  |
| ------ | ------------------------------------- |
| 后端   | Go, Gin, GORM                         |
| 数据库 | SQLite, PostgreSQL, MySQL             |
| 前端   | React, TypeScript, Vite, Tailwind CSS |
| 认证   | JWT, OAuth (GitHub, Google, OIDC)     |
| 存储   | 本地文件系统或 S3 兼容服务            |
| 邮件   | SMTP                                  |

## 相关链接

- [GitHub 仓库](https://github.com/vexgo-org/vexgo)
- [发布版本](https://github.com/vexgo-org/vexgo/releases)
- [问题追踪](https://github.com/vexgo-org/vexgo/issues)
- [许可证 (AGPL-3.0)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
