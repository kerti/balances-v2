import type { ReactElement, ReactNode } from "react";
import { render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router";
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
// runs in production. A component that reads the URL (the detail page's
// `?tab=`) opts in with `route`, which mounts a MemoryRouter at that location;
// a test that brings its own router leaves it out.
export function renderWithProviders(
  ui: ReactElement,
  { client = makeTestQueryClient(), route }: { client?: QueryClient; route?: string } = {},
) {
  function Wrapper({ children }: { children: ReactNode }) {
    const body =
      route === undefined ? (
        children
      ) : (
        <MemoryRouter initialEntries={[route]}>{children}</MemoryRouter>
      );
    return (
      <I18nextProvider i18n={i18n}>
        <QueryClientProvider client={client}>{body}</QueryClientProvider>
      </I18nextProvider>
    );
  }
  return { client, ...render(ui, { wrapper: Wrapper }) };
}
