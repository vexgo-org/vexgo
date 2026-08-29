# VexGo Documentation

> VexGo is a lightweight, self-hosted blog content management system built for developers and writers who value simplicity, performance, and control.

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

This site is the official documentation for VexGo. It is organized into four sections to help you find exactly what you need:

## 🚀 Getting Started

**Tutorial** — follow along and get a working VexGo instance in a few minutes.

- [Quick Start](/getting-started) — install, log in, and publish your first post.

## 📖 Guides

**How-to guides** — solve a specific problem, step by step.

- [Installation](/guides/installation) — install VexGo with Docker, Docker Compose, Nix, binaries, or from source.
- [Configuration](/guides/configuration) — configure the server, database, SSO, S3 storage, and more.
- [Deployment](/guides/deployment) — run VexGo in production behind a reverse proxy with HTTPS.

## 🧠 Concepts

**Explanation** — understand how VexGo works under the hood.

- [Architecture](/concepts/architecture) — backend layout, roles & permissions, moderation, and theming.

## 📚 Reference

**Technical reference** — look up exact details.

- [Configuration Reference](/reference/configuration) — every CLI flag, environment variable, and config-file key.
- [API Reference](/reference/api) — all REST endpoints, request/response shapes, and error codes.

---

## ✨ Key Features

- **🖥️ Modern Web Interface** — React-based admin panel for content management
- **🚀 High Performance** — built with Go and Gin
- **🔐 Secure Authentication** — JWT-based user system with role-based permissions (`guest` / `contributor` / `author` / `admin` / `super_admin`)
- **📝 Rich Content** — Markdown editor, categories, tags, drafts, likes, and comments
- **🛡️ Configurable Comment Moderation** — independent manual-review, keyword-filter, and LLM-review switches with fail-closed LLM fallback
- **🖼️ Media Management** — built-in file storage with S3-compatible support
- **🎨 Theme System** — server-side-rendered themes, switchable and uploadable from the admin panel
- **🔔 Notifications** — in-app message inbox for likes, comments, and other events
- **🔑 SSO** — login with GitHub, Google, or any OpenID Connect provider
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

## 🔗 Related Links

- [GitHub Repository](https://github.com/vexgo-org/vexgo)
- [Releases](https://github.com/vexgo-org/vexgo/releases)
- [Issue Tracker](https://github.com/vexgo-org/vexgo/issues)
- [License (AGPL-3.0)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
