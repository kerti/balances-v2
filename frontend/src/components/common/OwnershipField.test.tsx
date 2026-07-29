// Contract for the shared ownership control (#541). All 22 position
// Create/Edit dialogs had hand-inlined this block; the copies drifted only in
// their i18n namespace and radio-group name, but the duplication meant the
// mobile tap floor could not be fixed in one place. These assertions pin what
// the extraction has to preserve — the joint/sole choice, the conditional
// sole-owner select, and the callback surface the dialogs drive their form
// state through — plus the floor that motivated it.
//
// jsdom has no layout, so the tap floor is asserted as the class contract; the
// pixel truth lives in dialogs-mobile.spec.ts at 390px.
//
// covers: INV-PRESENTATION-08
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { OwnershipField } from "@/components/common/OwnershipField";

const me = {
  id: "user-1",
  household_id: "hh-1",
  household_display_name: "Household",
  display_name: "Member One",
  nickname: null,
  email: "one@example.com",
  picture_url: null,
  locale: "en-GB",
  theme: "system",
  carryover_date_mode: "month_end",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: false,
  assumed_annual_inflation: "3",
  is_founder: true,
};

const members = [
  { id: "user-1", display_name: "Member One", nickname: null },
  { id: "user-2", display_name: "Member Two", nickname: "Two" },
];

beforeEach(() => {
  server.use(
    http.get("/api/me", () => HttpResponse.json(me)),
    http.get("/api/household/members", () => HttpResponse.json(members)),
  );
});

function renderField(props: Partial<React.ComponentProps<typeof OwnershipField>> = {}) {
  const onChange = vi.fn();
  const onSoleOwnerChange = vi.fn();
  renderWithProviders(
    <OwnershipField
      idPrefix="test"
      value="joint"
      onChange={onChange}
      soleOwnerID={null}
      onSoleOwnerChange={onSoleOwnerChange}
      {...props}
    />,
  );
  return { onChange, onSoleOwnerChange };
}

describe("OwnershipField", () => {
  it("reports the picked ownership type to the parent", async () => {
    const { onChange } = renderField();
    await userEvent.click(screen.getByRole("radio", { name: /sole owner/i }));
    expect(onChange).toHaveBeenCalledWith("sole");
  });

  // The sole-owner select is the whole reason the block was conditional; a
  // joint position has no single owner to name.
  it("shows the sole-owner select only when sole is picked", async () => {
    renderField();
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();

    renderField({ value: "sole" });
    expect(await screen.findByRole("combobox")).toBeInTheDocument();
  });

  it("lists household members by preferred name and reports the pick", async () => {
    const { onSoleOwnerChange } = renderField({ value: "sole", soleOwnerID: "user-1" });
    const select = await screen.findByRole("combobox");
    // user-2 has a nickname, so it wins over display_name (preferredName).
    expect(await screen.findByRole("option", { name: "Two" })).toBeInTheDocument();
    await userEvent.selectOptions(select, "user-2");
    expect(onSoleOwnerChange).toHaveBeenCalledWith("user-2");
  });

  // The radio dot itself is ~13px and cannot be enlarged; the label row is the
  // hit area, so that is what has to clear the floor.
  it("gives each option a floored hit row on phones", () => {
    renderField();
    for (const name of [/joint/i, /sole owner/i]) {
      const row = screen.getByRole("radio", { name }).closest("label");
      expect(row).toHaveClass("max-md:min-h-11");
    }
  });

  // Two of these can be mounted at once (a list screen renders create and edit
  // dialogs); a shared radio-group name would let one steal the other's
  // selection. Eight dialogs previously hardcoded `name="ownership_type"`.
  it("scopes the radio group name to the id prefix", () => {
    renderField({ value: "joint" });
    expect(screen.getByRole("radio", { name: /joint/i })).toHaveAttribute(
      "name",
      "test_ownership_type",
    );
  });
});
