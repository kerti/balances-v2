import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { ErasePanel } from "@/components/common/ErasePanel";
import type { Me } from "@/hooks/useSession";

const me = {
  id: "u1",
  household_id: "hh-1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  is_founder: true,
} as Me;

function serve(demoMode: boolean) {
  server.use(
    http.get("/api/me", () => HttpResponse.json(me)),
    http.get("/api/auth/methods", () =>
      HttpResponse.json({ google: false, local: true, password_reset: false, demo_mode: demoMode }),
    ),
  );
}

it("disables the erase button and says why on the public demo", async () => {
  serve(true);
  renderWithProviders(<ErasePanel />);
  expect(await screen.findByTestId("erase-demo-disabled")).toHaveTextContent(
    "Turned off on the public demo",
  );
  expect(screen.getByTestId("erase-open-button")).toBeDisabled();
});

it("keeps the erase button enabled outside demo mode", async () => {
  serve(false);
  renderWithProviders(<ErasePanel />);
  const button = await screen.findByTestId("erase-open-button");
  expect(button).toBeEnabled();
  expect(screen.queryByTestId("erase-demo-disabled")).toBeNull();
});
