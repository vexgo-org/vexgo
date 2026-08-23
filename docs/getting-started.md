# Quick Start

> **Tutorial** — in this lesson you will install VexGo, log in for the first time, and publish your first post. It takes about 5 minutes.

This tutorial is written for complete beginners. By the end you will have a running VexGo instance with one published blog post.

## Before You Begin

You need:

- A machine running **Linux** or **macOS** (or a machine with **Docker** installed)
- An internet connection
- A web browser

No prior knowledge of Go, React, or databases is required.

## Step 1: Start VexGo

Choose the method that fits you best. Both start the same server.

### Option A: Run with Docker (recommended for trying out)

If you have Docker installed, run:

```bash
docker run -d --name vexgo -p 3001:3001 -v ./data:/app/data ghcr.io/vexgo-org/vexgo:latest
```

### Option B: Run the binary directly

1. Go to the [Releases page](https://github.com/vexgo-org/vexgo/releases) and download the binary for your system and architecture (e.g. `vexgo-linux-amd64`).
2. Make it executable and run it:

```bash
chmod +x vexgo-linux-amd64
./vexgo-linux-amd64
```

> **What just happened?** VexGo started an HTTP server on port `3001` and created a SQLite database in the `./data` directory. That's the whole server — no separate database to install.

## Step 2: Open the Site

Open your browser and visit:

```
http://127.0.0.1:3001
```

You should see the VexGo home page.

## Step 3: Log In

1. Click **Log in** (top right corner).
2. Use the default super admin account:

   | Field    | Value               |
   | -------- | ------------------- |
   | Email    | `admin@example.com` |
   | Password | `password`          |

3. Click **Log in**.

## Step 4: Change the Default Password

The default password is public knowledge — change it before doing anything else.

1. Click your avatar in the top right corner and open your **Profile**.
2. Change your password and save.

> **Security note:** anyone who can reach your instance can log in with the default credentials. Change the password immediately, and set a strong `JWT_SECRET` before deploying publicly. See [Deployment](/guides/deployment) for production hardening.

## Step 5: Write Your First Post

1. Click **New Post** (or **Write** in the navigation).
2. Enter a title, for example: `Hello, VexGo!`
3. Write some content in the Markdown editor.
4. Select a **category** (the default category already exists).
5. Click **Publish**.

Your post now appears on the home page, visible to everyone who visits your site.

## Step 6: Explore the Admin Panel

With the super admin account you can manage the whole site. From the admin panel you can:

- Moderate **pending posts and comments** (if moderation is enabled)
- Manage **users and roles**
- Change **site settings** (site name, registration, captcha)
- Install and switch **themes**

## What's Next?

Now that VexGo is running, you can go deeper:

- **Deploy it for real** — [Production Deployment](/guides/deployment) covers reverse proxies, HTTPS, and systemd.
- **Tune the configuration** — the [Configuration Guide](/guides/configuration) explains config files, environment variables, and databases.
- **Understand the internals** — [Architecture](/concepts/architecture) explains how VexGo is built.
- **Look up endpoints** — the [API Reference](/reference/api) documents every REST endpoint.
