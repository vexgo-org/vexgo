import path from "path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { inspectAttr } from "kimi-plugin-inspect-react";

// https://vite.dev/config/
export default defineConfig(({ command }) => ({
  // The admin SPA lives entirely under /admin/; public pages are served by
  // the active theme (vexgo-default-theme build) so the SPA must not claim root.
  base: "/admin/",
  plugins: [
    tailwindcss(),
    // inspectAttr tags every JSX element with a `code-path` attribute so the
    // browser inspector can jump from a DOM node back to its source. It is a
    // development aid: in a production build those attributes are dead weight
    // in the bundle and visible in every visitor's DOM, and the Babel pass
    // that adds them is pure build overhead.
    ...(command === "serve" ? [inspectAttr()] : []),
    react(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    outDir: "../backend/internal/public/dist",
    // outDir is outside the project root, so Vite does not empty old output by default.
    // Not emptying it would leave files from previous builds in dist,
    // and the backend's buildAssetManifest would pick the stale bundle when
    // multiple same-named assets are sorted alphabetically, causing the SSR
    // page to load outdated frontend code.
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          // Bundle React-related libraries separately
          "react-vendor": ["react", "react-dom", "react-router-dom"],
          // Bundle the API client separately
          "utils-vendor": ["axios"],
          // Deliberately no chunk for @base-ui/react, lucide-react, sonner and
          // tailwind-merge. Naming a package here makes the whole package part of
          // the entry's static graph, so the UI primitives only the shell reached
          // were joined by the ones used exclusively by lazy routes — 50 kB of
          // code an admin login never runs. Left unnamed, Rollup keeps each
          // module in the chunk that actually imports it.
        },
      },
    },
    // Adjust the warning threshold (optional)
    chunkSizeWarningLimit: 600,
    // Generate manifest.json so the backend SSR can reference the correct entry and vendor assets.
    // The backend cannot rely on scanning the assets directory alphabetically: a single build
    // produces multiple index-<hash>.js files (lazy-loaded chunks), and name-based mapping
    // would select the wrong files.
    manifest: true,
  },
}));
