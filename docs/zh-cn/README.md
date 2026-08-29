# VexGo 文档

> VexGo 是一个轻量级、自托管的博客内容管理系统，专为重视简单、性能与可控性的开发者和写作者设计。

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

这里是 VexGo 的官方文档站，内容按四个板块组织，方便你快速找到所需信息：

## 🚀 快速开始

**教程** —— 跟着做，几分钟内跑起一个可用的 VexGo 实例。

- [快速开始](/zh-cn/getting-started) —— 安装、登录并发布你的第一篇文章。

## 📖 指南

**操作指南** —— 针对具体问题，一步步给出解决方案。

- [安装](/zh-cn/guides/installation) —— 通过 Docker、Docker Compose、Nix、二进制包或源码编译安装 VexGo。
- [配置](/zh-cn/guides/configuration) —— 配置服务器、数据库、SSO、S3 存储等。
- [部署](/zh-cn/guides/deployment) —— 在生产环境通过反向代理 + HTTPS 运行 VexGo。

## 🧠 概念

**原理讲解** —— 了解 VexGo 的内部设计。

- [架构](/zh-cn/concepts/architecture) —— 后端结构、角色与权限、内容审核、主题系统。

## 📚 参考

**技术参考** —— 查询精确的技术细节。

- [配置参考](/zh-cn/reference/configuration) —— 所有 CLI 参数、环境变量和配置文件键。
- [API 参考](/zh-cn/reference/api) —— 全部 REST 端点、请求/响应格式和错误码。

---

## ✨ 主要特性

- **🖥️ 现代化 Web 界面** —— 基于 React 的管理面板用于内容管理
- **🚀 高性能** —— 使用 Go 和 Gin 构建
- **🔐 安全认证** —— 基于 JWT 的用户系统，支持角色权限（`guest` / `contributor` / `author` / `admin` / `super_admin`）
- **📝 丰富内容** —— Markdown 编辑器、分类、标签、草稿、点赞和评论
- **🛡️ 可配置评论审核** —— 人工审核、关键词过滤、大模型审核三个独立开关，LLM 故障自动转入人工队列（fail-closed）
- **🖼️ 媒体管理** —— 内置文件存储，支持 S3 兼容服务
- **🎨 主题系统** —— 服务端渲染主题，可在管理面板切换和上传
- **🔔 通知** —— 点赞、评论等事件的站内消息收件箱
- **🔑 SSO** —— 支持 GitHub、Google 及任意 OpenID Connect 提供商登录
- **🌐 自托管** —— 完全控制你的数据和部署

## 🛠️ 技术栈

| 层次   | 技术                                  |
| ------ | ------------------------------------- |
| 后端   | Go, Gin, GORM                         |
| 数据库 | SQLite, PostgreSQL, MySQL             |
| 前端   | React, TypeScript, Vite, Tailwind CSS |
| 认证   | JWT, OAuth (GitHub, Google, OIDC)     |
| 存储   | 本地文件系统或 S3 兼容服务            |
| 邮件   | SMTP                                  |

## 🔗 相关链接

- [GitHub 仓库](https://github.com/vexgo-org/vexgo)
- [发布版本](https://github.com/vexgo-org/vexgo/releases)
- [问题追踪](https://github.com/vexgo-org/vexgo/issues)
- [许可证 (AGPL-3.0)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
