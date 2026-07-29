// Contract test for the Settings scalar-preference tables (#563). The screen
// speaks two visual languages on purpose — a settings table for scalar
// preferences, panel cards for actions and flows — and three things about the
// table half are easy to regress by eye and cheap to pin here:
//
//  1. the grouping (Profile and Household are one card each, of rows),
//  2. the a11y names, which is the whole reason the redundant per-field
//     `Label`s could be dropped: the row name became the `<label for>`, so a
//     careless "delete the duplicate" would leave a nameless textbox, and
//  3. error scoping — household name, reporting currency and assumed inflation
//     all ride the same `useUpdateHouseholdSettings()` PATCH, so putting them
//     in one card is exactly when one failed save could start printing under
//     all three.
//
// The panel half is here too, since it is the same policy from the other side:
// Data's three flows group into one card of `SettingsPanel`s, with Erase — the
// one irreversible action — carrying the destructive tone.
//
// Only `ReactivationCard` is stubbed: it fetches its dormant-member list on
// mount and MSW is set to fail unhandled requests loudly. Everything else
// renders for real.
import { it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { SettingsScreen } from "@/components/screens/SettingsScreen";
import type { Me } from "@/hooks/useSession";

vi.mock("@/components/common/ReactivationCard", () => ({ ReactivationCard: () => <div /> }));

const me: Me = {
  id: "u1",
  household_id: "hh-1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "light",
  carryover_date_mode: "today",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: false,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

async function renderScreen() {
  server.use(http.get("/api/me", () => HttpResponse.json(me)));
  const view = renderWithProviders(
    <ThemeProvider>
      <MemoryRouter>
        <SettingsScreen />
      </MemoryRouter>
    </ThemeProvider>,
  );
  await screen.findByLabelText("Household name");
  return view;
}

function rows(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLElement>("[data-slot=settings-row]"));
}

// The row name lives in the row's first cell; the control (and any visible
// field label of its own) in the second.
function nameCell(row: HTMLElement) {
  return row.firstElementChild as HTMLElement;
}
function controlCell(row: HTMLElement) {
  return row.lastElementChild as HTMLElement;
}

it("groups the scalar preferences into one Profile card and one Household card", async () => {
  const { container } = await renderScreen();

  const cards = Array.from(container.querySelectorAll<HTMLElement>("[data-slot=card]"));
  const tableCards = cards.filter((c) => c.querySelector("[data-slot=settings-row]"));
  expect(tableCards, "scalar preferences live in exactly two group cards").toHaveLength(2);

  const [profile, household] = tableCards;
  expect(rows(profile).map((r) => nameCell(r).firstElementChild?.textContent)).toEqual([
    "Your name",
    "Language",
    "Appearance",
    "Carry-over date",
  ]);
  expect(rows(household)).toHaveLength(4);
  expect(within(household).getByLabelText("Household name")).toHaveAttribute(
    "id",
    "household-name",
  );
  // The multi-currency toggle is promoted out from under the currency input
  // into a row of its own, rather than staying a footnote to that field.
  expect(within(household).getByRole("checkbox")).toBeInTheDocument();
  expect(rows(household).map((r) => nameCell(r).firstElementChild?.textContent)).toEqual([
    "Household name",
    "Currency",
    "Multi-currency",
    "Inflation",
  ]);
});

// covers: INV-PRESENTATION-08
it("gives every control an accessible name with no row-name / field-label duplicate", async () => {
  const { container } = await renderScreen();

  for (const row of rows(container)) {
    const rowName = nameCell(row).firstElementChild?.textContent?.trim() ?? "";
    expect(rowName, "every row states its setting's name").not.toBe("");

    // Every control in the row is named — via the row name acting as its
    // `<label for>`, or via a field label the control cell supplies itself.
    const controls = controlCell(row).querySelectorAll<HTMLElement>("input, select, textarea");
    expect(controls.length, `${rowName} renders a control`).toBeGreaterThan(0);
    for (const control of controls) {
      expect(control, `${rowName} control is named`).toHaveAccessibleName();
    }

    // ...and where the control cell does supply its own visible label, that
    // label says something the row name does not.
    for (const label of controlCell(row).querySelectorAll("label")) {
      const text = label.textContent?.trim() ?? "";
      expect(text.toLowerCase(), `${rowName} field label repeats the row name`).not.toBe(
        rowName.toLowerCase(),
      );
    }
  }

  // Spot-check the mapping the dropped `Label`s used to provide.
  expect(screen.getByLabelText("Your name")).toHaveAttribute("id", "nickname");
  expect(screen.getByLabelText("Language")).toHaveAttribute("id", "language");
  expect(screen.getByLabelText("Appearance")).toHaveAttribute("id", "theme");
  expect(screen.getByLabelText("Carry-over date")).toHaveAttribute("id", "carryover-date-mode");
  // The two rows that keep a visible field label keep it for the extra it
  // carries — "Reporting" and the unit — so the row name is not the label.
  expect(screen.getByLabelText("Reporting currency")).toHaveAttribute("id", "reporting-currency");
  expect(screen.getByLabelText("Assumed annual %")).toHaveAttribute("id", "assumed-inflation");
});

it("surfaces a failed household PATCH against one row only", async () => {
  server.use(
    http.patch("/api/household/settings", () =>
      HttpResponse.json({ code: "UNKNOWN" }, { status: 500 }),
    ),
  );
  const { container } = await renderScreen();
  const user = userEvent.setup();

  const householdName = screen.getByLabelText("Household name");
  await user.clear(householdName);
  await user.type(householdName, "Renamed");
  await user.click(
    within(
      container.querySelector<HTMLElement>("[data-slot=settings-row]:has(#household-name)")!,
    ).getByRole("button", { name: "Save" }),
  );

  const failingRow = await waitFor(() => {
    const row = container.querySelector<HTMLElement>(
      "[data-slot=settings-row]:has(#household-name)",
    )!;
    expect(row.querySelector(".text-destructive")).not.toBeNull();
    return row;
  });

  const erroring = rows(container).filter((r) => r.querySelector(".text-destructive"));
  expect(erroring, "only the row whose save failed shows the error").toEqual([failingRow]);
});

it("groups the Data flows into one card of panels, with Erase alone in the destructive tone", async () => {
  const { container } = await renderScreen();

  const groups = Array.from(container.querySelectorAll<HTMLElement>("[data-slot=card]")).filter(
    (c) => c.querySelector("[data-slot=settings-panel]"),
  );
  expect(groups, "the Data flows share one card").toHaveLength(1);

  const panels = Array.from(groups[0].querySelectorAll<HTMLElement>("[data-slot=settings-panel]"));
  expect(panels.map((p) => p.querySelector("h3")?.textContent)).toEqual([
    "Backup",
    "Restore from backup",
    "Delete this household",
  ]);

  // Panels are the *flows*, so they are not table rows — the point of the
  // policy is that neither shape leaks into the other.
  expect(groups[0].querySelector("[data-slot=settings-row]")).toBeNull();

  // Only the irreversible one is tinted.
  expect(panels.map((p) => p.dataset.tone)).toEqual(["default", "default", "destructive"]);
});

it("puts the invite Send beside the email field", async () => {
  await renderScreen();

  const email = screen.getByLabelText("Email address");
  const controlRow = email.closest("[data-slot=settings-control-row]");
  expect(controlRow, "the email field sits in a control row").not.toBeNull();
  expect(
    within(controlRow as HTMLElement).getByRole("button", { name: "Send invitation" }),
  ).toBeInTheDocument();
});
