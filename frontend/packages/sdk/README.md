# @vexgo/sdk

Official VexGo frontend SDK for third-party browser themes. Framework-agnostic
TypeScript over `ofetch`, with types generated from the same
`docs/swagger.json` contract as the built-in frontend.

- Browser-only: the token lifecycle lives in `localStorage`
  (`vexgo:token` / `vexgo:user`, isolated from the legacy frontend keys).
- Single entry: `createVexgoClient()` returns one group per backend domain.
- Failures throw a normalized `VexGoError`; success values are the generated
  response types (no `response.data` unwrapping).
- One active client per page (the runtime and the stored token are shared,
  which matches the one-theme-one-backend model).

## Install

While the SDK is stabilizing it is consumed through the pnpm workspace:

```ts
import { createVexgoClient } from "@vexgo/sdk";
```

## Quickstart

```ts
import { createVexgoClient, VexGoError } from "@vexgo/sdk";

const client = createVexgoClient({
  baseURL: "https://cms.example.com/api", // required, no env fallback
  onUnauthorized: () => location.assign("/login"), // optional, default: clear only
});

try {
  await client.auth.login({ email, password }); // token persisted automatically
  const posts = await client.posts.list({ page: 1, limit: 10 });
  const me = await client.auth.me();
} catch (error) {
  if (error instanceof VexGoError) {
    console.error(error.status, error.code, error.message);
  }
}
```

Token helpers: `client.getToken()` (prefers injected `getToken` when provided),
`client.setToken(token)`, `client.clearToken()`, `client.auth.logout()`.

File upload:

```ts
const file = document.querySelector("input[type=file]").files[0];
await client.upload.uploadFile(file);
```

## Domain groups

`auth captcha posts comments users upload notifications settings sso home`.
Method names mirror the backend operations (`posts.list/create/getBySlug/...`,
`settings.getGeneral/updateGeneral/...`); signatures are inferred from the
generated layer, so they track the backend automatically. The raw generated
client (`getVexGoAPI`) and all generated model types are re-exported as an
escape hatch.

## Codegen

`src/generated/` is never hand-edited. Regenerate both the legacy frontend
client and this SDK from the backend annotations with:

```bash
just generate
```

`frontend/orval.config.ts` holds two targets (`vexgo`, `vexgo-sdk`) fed by the
same `docs/swagger.json`; the SDK target uses `src/_mutator.ts`
(ofetch-based, returns unwrapped bodies).

## Compatibility

The SDK versions independently from the backend (semver). Supported backend:

| SDK   | Backend |
| ----- | ------- |
| 0.1.x | 1.x     |

## 中文摘要

- 给第三方浏览器主题用的官方 SDK，与框架无关，基于 `ofetch`。
- 只支持浏览器：token 存 `localStorage`（`vexgo:token` / `vexgo:user`，
  与老前端的键隔离）；`login` 成功自动存，`logout`/401 自动清。
- `createVexgoClient({ baseURL })` 按域分组返回全部接口；失败统一抛
  `VexGoError`（`status/code/message/data`），成功值即生成类型。
- 类型与接口来自 `just generate`（与老前端同源，`src/generated/` 禁止手改）。
- 版本独立 semver，README 声明兼容的后端版本（见上表）。
