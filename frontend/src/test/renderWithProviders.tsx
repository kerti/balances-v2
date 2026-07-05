import type { ReactElement, ReactNode } from "react";
import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import i18n from "@/i18n";

// A fresh QueryClient per render keeps cache state from leaking between tests;
// retries off so an intentional error handler surfaces immediately instead of
// being retried into a timeout.
export function makeTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

// Wraps a component in the same providers `main.tsx` gives the app minus the
// router — the list screens take an `onSelect` callback, not a route — so a
// component test exercises hooks (react-query) and copy (i18next) the way it
// runs in production.
export function renderWithProviders(
  ui: ReactElement,
  { client = makeTestQueryClient() }: { client?: QueryClient } = {},
) {
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      </I18nextProvider>
    );
  }
  return { client, ...render(ui, { wrapper: Wrapper }) };
}
