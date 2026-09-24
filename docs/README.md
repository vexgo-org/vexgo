# VexGo Documentation

> VexGo is a self-hosted blog CMS written in Go. One binary serves the JSON API, the server-side rendered public pages, and the admin panel.

[![Go Version](https://img.shields.io/github/go-mod/go-version/vexgo-org/vexgo)](https://go.dev/)
[![License](https://img.shields.io/github/license/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/vexgo-org/vexgo/build-and-test.yml?branch=main)](https://github.com/vexgo-org/vexgo/actions)
[![Release](https://img.shields.io/github/v/release/vexgo-org/vexgo)](https://github.com/vexgo-org/vexgo/releases)

The documentation is split into five sections.

## Getting Started

Tutorial: follow along to get a working instance running in a few minutes.

- [Quick Start](/getting-started): install VexGo, log in, and publish your first post.

## Guides

How-to guides for specific tasks.

- [Installation](/guides/installation): install VexGo with Docker, Docker Compose, Nix, a binary release, or from source.
- [Configuration](/guides/configuration): configure the server, database, SSO, S3 storage, and more.
- [Deployment](/guides/deployment): run VexGo in production behind a reverse proxy with HTTPS.

## Local Development

Set up a local stack, then look up the daily loops.

- [Quick Develop](/local-development/quick-develop): first run in about 10 minutes, covering the backend, admin SPA, and default theme.
- [Backend](/local-development/backend): run, configure, and extend the Go backend.
- [Frontend](/local-development/frontend): run, change, and rebuild the admin SPA.
- [Theme Development](/local-development/theme-development): write, package, and install a custom theme.
- [Workflow](/local-development/workflow): format/lint/test gates, API codegen, and troubleshooting.

## Concepts

How VexGo is designed.

- [Architecture](/concepts/architecture): backend structure, roles and permissions, moderation, and SSO.
- [Theming](/concepts/theming): how themes become server-side-rendered pages.

## Reference

Precise technical details.

- [Configuration Reference](/reference/configuration): every CLI flag, environment variable, and config file key.
- [Theme Templates Reference](/reference/theme-templates): every theme file, template context field, and helper function.
- [API Reference](api.html): every REST endpoint, request/response shape, and error code.

---

## Features

- React admin panel for managing content
- Go and Gin backend
- JWT authentication with role-based permissions (`guest` / `contributor` / `author` / `admin` / `super_admin`)
- Markdown editor with categories, tags, drafts, likes, and comments
- Comment moderation with independent manual-review, keyword-filter, and LLM-review switches; the LLM check fails closed
- File storage on local disk or any S3-compatible service
- Server-side rendered themes, switchable and uploadable from the admin panel
- In-app notifications for likes, comments, and other events
- Login with GitHub, Google, or any OpenID Connect provider
- Self-hosted, so your data and deployment stay under your control

## Technology Stack

| Layer          | Technology                                 |
| -------------- | ------------------------------------------ |
| Backend        | Go, Gin, GORM                              |
| Database       | SQLite, PostgreSQL, MySQL                  |
| Frontend       | React, TypeScript, Vite, Tailwind CSS      |
| Authentication | JWT, OAuth (GitHub, Google, OIDC)          |
| Storage        | Local filesystem or S3-compatible services |
| Email          | SMTP                                       |

## Links

- [GitHub repository](https://github.com/vexgo-org/vexgo)
- [Releases](https://github.com/vexgo-org/vexgo/releases)
- [Issue tracker](https://github.com/vexgo-org/vexgo/issues)
- [License (AGPL-3.0)](https://github.com/vexgo-org/vexgo/blob/main/LICENSE)
