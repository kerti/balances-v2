import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";

// API_PROXY_TARGET lets the E2E run point the dev server at a backend on a
// non-default port (the balances_e2e instance) without disturbing the
// developer's 8080 dev backend. Defaults to 8080 for normal `npm run dev`.
// See ADR-0024.
const apiProxyTarget = process.env.API_PROXY_TARGET ?? "http://localhost:8080";

// The installable PWA (ADR-0055). Two rules shape everything here: the service
// worker precaches the built shell and nothing else — no runtimeCaching, so no
// /api response ever lands in a cache where a stale screen could read as
// current data — and it never swaps the app out from under the user (see
// registerType). The icons are rasterized by `make brand`, not at build time.
const pwa = VitePWA({
  // 'prompt', not 'autoUpdate': an auto-reload on a new deploy would discard a
  // half-filled snapshot or transaction dialog. UpdateBanner offers the reload
  // and lets the user pick the moment.
  registerType: "prompt",
  // UpdateBanner registers the worker (virtual:pwa-register/react); the plugin
  // must not inject a second registration script.
  injectRegister: null,
  manifest: {
    name: "Balances",
    short_name: "Balances",
    description: "Track your household net worth without itemising every transaction.",
    start_url: "/",
    scope: "/",
    display: "standalone",
    // #14161A is the graphite plate (ADR-0054) and the dark theme's background,
    // the app's default theme — so the splash matches the first paint.
    background_color: "#14161A",
    theme_color: "#14161A",
    icons: [
      { src: "/icons/icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/icons/icon-512.png", sizes: "512x512", type: "image/png" },
      {
        src: "/icons/icon-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  },
  workbox: {
    globPatterns: ["**/*.{js,css,html,svg,png,woff2}"],
    navigateFallback: "/index.html",
    // The Go binary owns these, not the SPA. Without the denylist an installed
    // worker would answer a navigation to them (the OAuth redirects under
    // /api/auth, a PDF opened by URL) with the cached shell.
    navigateFallbackDenylist: [/^\/api\//, /^\/healthz$/],
  },
});

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), pwa],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    host: true,
    allowedHosts: process.env.VITE_DEV_ALLOWED_HOSTS?.split(",") ?? [],
    proxy: {
      "/healthz": apiProxyTarget,
      "/api": apiProxyTarget,
    },
  },
  build: {
    rolldownOptions: {
      output: {
        manualChunks: (id) => {
          if (!id.includes("node_modules")) return;
          if (id.includes("react-dom") || id.includes("/react/")) return "react";
          if (id.includes("@tanstack")) return "react-query";
          if (id.includes("radix-ui") || id.includes("@radix-ui")) return "radix";
          if (id.includes("lucide-react")) return "lucide";
        },
      },
    },
  },
});
