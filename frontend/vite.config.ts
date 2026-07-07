import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// API_PROXY_TARGET lets the E2E run point the dev server at a backend on a
// non-default port (the balances_e2e instance) without disturbing the
// developer's 8080 dev backend. Defaults to 8080 for normal `npm run dev`.
// See ADR-0024.
const apiProxyTarget = process.env.API_PROXY_TARGET ?? "http://localhost:8080";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    host: true,
    proxy: {
      "/healthz": apiProxyTarget,
      "/api": apiProxyTarget,
    },
  },
  // @react-pdf/renderer is lazy-loaded (ReportPdfButton, ADR-0044) behind a
  // dynamic import(), and its dependency graph (fonts, layout engine, pdfkit)
  // is far larger than any other lazy chunk here (~1.4MB vs recharts' ~18KB).
  // Without this, Vite defers pre-bundling it until the first request that
  // actually imports it — on a cold cache (fresh CI checkout) that first
  // fetch+transform can take longer than a test's default locator timeout.
  // Listing it here makes the dev server pre-bundle it eagerly at startup,
  // inside the webServer's own (generous) readiness window instead.
  optimizeDeps: {
    include: ["@react-pdf/renderer"],
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
