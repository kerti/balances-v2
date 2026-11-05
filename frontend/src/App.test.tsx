import { it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { Outlet } from "react-router";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";

// The shell pulls in sidebar queries unrelated to routing; stub it to a bare
// outlet so the real route table is what's under test.
vi.mock("@/components/shell/AppShell", () => ({ AppShell: () => <Outlet /> }));

it("renders the Not found screen, with a dashboard link, for an unknown path", async () => {
  // createBrowserRouter reads the location when App's module loads.
  window.history.pushState(null, "", "/does-not-exist");
  server.use(
    http.get("/api/me", () =>
      HttpResponse.json({ id: "u1", household_id: "hh-1", display_name: "Pat", locale: "en-GB" }),
    ),
  );
  const { default: App } = await import("@/App");
  renderWithProviders(<App />);

  expect(await screen.findByTestId("not-found-screen")).toBeInTheDocument();
  expect(screen.getByTestId("not-found-dashboard-link")).toHaveAttribute("href", "/");
  expect(window.location.pathname).toBe("/does-not-exist");
});
