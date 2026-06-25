import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import {
  MutationCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import "./index.css";
import App from "./App.tsx";
import i18n from "./i18n";
import { ThemeProvider } from "./theme/ThemeProvider";
import { Toaster } from "./components/ui/sonner";

// Any successful write may change a monthly-report input — a snapshot,
// transaction, income event, position metadata/lifecycle, or FX rate. Reports
// regenerate lazily on read (ADR-0006), so invalidating ['reports'] after every
// mutation keeps the dashboard's net worth fresh. Done globally rather than per
// hook because ADR-0006 warns that enumerating every input drifts silently when
// one is missed — the same fragility applies on the client. Cheap for a
// single-household app: the refetch's server-side regen is a no-op when nothing
// actually went stale, and only fires when the dashboard query is mounted.
const queryClient = new QueryClient({
  mutationCache: new MutationCache({
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["reports"] });
    },
  }),
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <App />
          <Toaster />
        </QueryClientProvider>
      </ThemeProvider>
    </I18nextProvider>
  </StrictMode>,
);
