import path from "path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { inspectAttr } from "kimi-plugin-inspect-react";

// https://vite.dev/config/
export default defineConfig({
  // The admin SPA lives entirely under /admin/; public pages are served by
  // the active theme (vexgo-default-theme build) so the SPA must not claim root.
  base: "/admin/",
  plugins: [inspectAttr(), react()],
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
          // Bundle the UI component library separately
          "ui-vendor": [
            "@radix-ui/react-slot",
            "class-variance-authority",
            "clsx",
            "tailwind-merge",
            "lucide-react",
          ],
          // Bundle state-management and utility libraries separately
          "utils-vendor": ["axios", "date-fns"],
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
});
