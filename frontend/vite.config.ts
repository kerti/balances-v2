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
