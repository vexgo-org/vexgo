# VexGo Documentation

> VexGo is a lightweight, self-hosted blog content management system built for developers and writers who value simplicity, performance, and control.

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

This is the official VexGo documentation site. It is organized into five sections so you can find what you need quickly:

## 🚀 Getting Started

**Tutorial** — follow along and get a working VexGo instance running in minutes.

- [Quick Start](/getting-started) — install VexGo, log in, and publish your first post.

## 📖 Guides

**How-to** — step-by-step solutions to a specific problem.

- [Installation](/guides/installation) — install VexGo with Docker, Docker Compose, Nix, a binary release, or from source.
- [Configuration](/guides/configuration) — configure the server, database, SSO, S3 storage, and more.
- [Deployment](/guides/deployment) — run VexGo in production behind a reverse proxy with HTTPS.

## 🛠️ Local Development

**Tutorial + How-to** — go from zero to a running local stack, then look up the daily loops.

- [Quick Develop](/local-development/quick-develop) — first run in about 10 minutes: backend, admin SPA, and default theme.
- [Backend](/local-development/backend) — run, configure, and extend the Go backend.
- [Frontend](/local-development/frontend) — run, change, and rebuild the admin SPA.
- [Theme Development](/local-development/theme-development) — write, package, and install a custom theme.
- [Workflow](/local-development/workflow) — format/lint/test gates, API codegen, and troubleshooting.

## 🧠 Concepts

**Explanation** — understand how VexGo is designed.

- [Architecture](/concepts/architecture) — backend structure, roles and permissions, moderation, and SSO.
- [Theming](/concepts/theming) — how themes become server-side-rendered pages.

## 📚 Reference

**Information** — look up precise technical details.

- [Configuration Reference](/reference/configuration) — every CLI flag, environment variable, and config file key.
- [Theme Templates Reference](/reference/theme-templates) — every theme file, template context field, and helper function.
- [API Reference](api.html) — every REST endpoint, request/response shape, and error code.

---

## ✨ Key Features

- **🖥️ Modern Web Interface** — React-based admin panel for content management
- **🚀 High Performance** — built with Go and Gin
- **🔐 Secure Authentication** — JWT-based user system with role-based permissions (`guest` / `contributor` / `author` / `admin` / `super_admin`)
- **📝 Rich Content** — Markdown editor, categories, tags, drafts, likes, and comments
- **🛡️ Configurable Comment Moderation** — independent manual-review, keyword-filter, and LLM-review switches, with fail-closed LLM fallback
- **🖼️ Media Management** — built-in file storage with S3-compatible support
- **🎨 Theme System** — server-side-rendered themes, switchable and uploadable from the admin panel
- **🔔 Notifications** — in-app notification inbox for likes, comments, and other events
- **🔑 SSO** — log in with GitHub, Google, or any OpenID Connect provider
- **🌐 Self-Hosted** — complete control over your data and deployment

## 🛠️ Technology Stack

| Layer          | Technology                                 |
| -------------- | ------------------------------------------ |
| Backend        | Go, Gin, GORM                              |
| Database       | SQLite, PostgreSQL, MySQL                  |
| Frontend       | React, TypeScript, Vite, Tailwind CSS      |
| Authentication | JWT, OAuth (GitHub, Google, OIDC)          |
| Storage        | Local filesystem or S3-compatible services |
| Email          | SMTP                                       |

## 🔗 Links

- [GitHub repository](https://github.com/vexgo-org/vexgo)
- [Releases](https://github.com/vexgo-org/vexgo/releases)
- [Issue tracker](https://github.com/vexgo-org/vexgo/issues)
- [License (AGPL-3.0)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
