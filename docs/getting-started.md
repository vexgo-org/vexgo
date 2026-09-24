# Quick Start

> Tutorial: install VexGo, log in for the first time, and publish your first post. It takes about 5 minutes.

By the end you will have a running VexGo instance with one published post.

## Before you begin

You need:

- A machine running Linux, macOS, Windows, or FreeBSD, or one with Docker installed
- An internet connection
- A web browser

No prior knowledge of Go, React, or databases is required.

## Step 1: Start VexGo

Pick either method; both start the same server.

### Option A: Run with Docker (recommended for trying out)

If you have Docker installed, run:

```bash
docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest ./vexgo server
```

### Option B: Run the binary directly

1. Go to the [Releases page](https://github.com/vexgo-org/vexgo/releases) and download the binary for your system and architecture (e.g. `vexgo-linux-amd64`).
2. Make it executable and run it:

```bash
chmod +x vexgo-linux-amd64
./vexgo-linux-amd64 server
```

> VexGo started an HTTP server on port `3001` and created a SQLite database in the `./data` directory. There is no separate database to install.

## Step 2: Open the site

Visit:

```
http://127.0.0.1:3001
```

You should see the VexGo home page.

## Step 3: Log in

1. Click **Log in** (top right corner).
2. Use the default super admin account:

   | Field    | Value               |
   | -------- | ------------------- |
   | Email    | `admin@example.com` |
   | Password | `password`          |

3. Click **Log in**.

Direct link: `http://127.0.0.1:3001/admin/login`. Legacy top-level URLs such as `/login` 301-redirect to their `/admin/` equivalent with the query string preserved.

## Step 4: Change the default password

The default password is public knowledge, so change it first.

1. Click your avatar in the top right corner and open your **Profile**.
2. Change your password and save.

> Anyone who can reach your instance can log in with the default credentials. Change the password immediately, and set a strong `JWT_SECRET` and `SETTINGS_ENCRYPTION_KEY` (used to encrypt the SMTP password and AI/comment-moderation API keys at rest). See [Deployment](/guides/deployment) before exposing the instance publicly.

## Step 5: Write your first post

1. Click **New Post** (or **Write** in the navigation). Direct link: `http://127.0.0.1:3001/admin/write`.
2. Enter a title, for example: `Hello, VexGo!`
3. Write some content in the Markdown editor.
4. Select a **category** (the default category already exists).
5. Click **Publish**.

Your post now appears on the home page.

## Step 6: Explore the admin panel

With the super admin account you can manage the whole site. The admin panel at `http://127.0.0.1:3001/admin/` lets you:

- Moderate pending posts and comments, if moderation is enabled
- Manage users and roles
- Change site settings such as the site name, registration, and captcha
- Install and switch themes

## What's next?

- [Production Deployment](/guides/deployment) covers reverse proxies, HTTPS, and systemd.
- The [Configuration Guide](/guides/configuration) explains config files, environment variables, and databases.
- [Architecture](/concepts/architecture) explains how VexGo is built.
- The [API Reference](api.html) documents every REST endpoint.
