/**
 * Generates the built-in theme's Go-template HTML pages from the React
 * templates in src/templates. Run with bun (`bun scripts/generate.tsx`):
 *
 *   1. renderToString each page component — dynamic parts are literal
 *      {{...}} Go template actions emitted via the go() helper,
 *   2. strip React's <!-- --> separator comment nodes,
 *   3. write index.html / post.html / user.html / 404.html into
 *      backend/internal/public/default-theme (embedded into the binary).
 */
import { copyFileSync, cpSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { renderToString } from "react-dom/server";

import { HomeTemplate } from "../src/templates/HomeTemplate";
import { LinksTemplate } from "../src/templates/LinksTemplate";
import { NotFoundTemplate } from "../src/templates/NotFoundTemplate";
import { PageTemplate } from "../src/templates/PageTemplate";
import { PostTemplate } from "../src/templates/PostTemplate";
import { TimelineTemplate } from "../src/templates/TimelineTemplate";
import { UserTemplate } from "../src/templates/UserTemplate";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, "../../backend/internal/public/default-theme");
mkdirSync(outDir, { recursive: true });

function emit(name: string, element: React.ReactNode) {
  let html = renderToString(element);

  // React 19 hoists <link rel="preload" as="image"> for every <img> src into
  // <head>. The template data there is out of scope (e.g. {{.CoverImage}} sits
  // outside any {{range}}), so drop these injected preloads entirely.
  html = html.replace(/<link rel="preload" as="image"[^>]*\/>/g, "");

  // Strip React's separator comment nodes between adjacent text/elements.
  html = html.replace(/<!-- -->/g, "");

  // React HTML-escapes quotes and ampersands even in text nodes. Decode them
  // inside Go template actions only, so {{date .CreatedAt "2006-01-02"}}
  // stays valid Go template syntax. Static HTML entities outside actions are
  // left untouched.
  html = html.replace(
    /\{\{([\s\S]*?)\}\}/g,
    (_, inner: string) =>
      "{{" +
      inner
        .replace(/&quot;/g, '"')
        .replace(/&#39;/g, "'")
        .replace(/&amp;/g, "&") +
      "}}",
  );

  writeFileSync(resolve(outDir, name), `<!DOCTYPE html>\n${html}\n`);
}

emit("index.html", HomeTemplate());
emit("post.html", PostTemplate());
emit("page.html", PageTemplate());
emit("timeline.html", TimelineTemplate());
emit("links.html", LinksTemplate());
emit("user.html", UserTemplate());
emit("404.html", NotFoundTemplate());

// Static widget assets ship inside the theme so /theme-assets/... can serve
// them with the same prefix as style.css.
const assetsDir = resolve(outDir, "assets");
mkdirSync(assetsDir, { recursive: true });
copyFileSync(
  resolve(here, "../widget/comments.js"),
  resolve(assetsDir, "comments.js"),
);

// Theme-owned i18n dictionaries (i18n/<lang>.json) ship inside the theme so
// the backend loader can serve any language the theme provides. The backend
// never hardcodes languages; adding a file here is enough to support one.
cpSync(resolve(here, "../i18n"), resolve(outDir, "i18n"), {
  recursive: true,
});

// Theme seed pages (seed/*.md) ship inside the theme so the backend can
// create the theme's default pages on activation. Third-party themes follow
// the same layout: seed/<slug>.md with YAML frontmatter.
cpSync(resolve(here, "../seed"), resolve(outDir, "seed"), { recursive: true });

console.log(`default theme templates written to ${outDir}`);
