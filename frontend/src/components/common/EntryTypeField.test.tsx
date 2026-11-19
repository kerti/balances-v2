// Contract for the shared entry-type control (#594, ADR-0053 §3). It is on all
// 20 position Create/Edit dialogs, and its presence on Edit is load-bearing:
// nothing detects a mis-declared entry — a one-sided birth is indistinguishable
// from an acquisition whose funding has not been snapshotted yet — so flipping
// this control is the only remedy a household has.
//
// jsdom has no layout, so the tap floor is asserted as the class contract; the
// pixel truth lives in dialogs-mobile.spec.ts at 390px.
//
// covers: INV-PRESENTATION-08
import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryTypeField } from "@/components/common/EntryTypeField";

function renderField(props: Partial<React.ComponentProps<typeof EntryTypeField>> = {}) {
  const onChange = vi.fn();
  renderWithProviders(
    <EntryTypeField
      idPrefix="test"
      group="asset"
      value="acquired"
      onChange={onChange}
      {...props}
    />,
  );
  return { onChange };
}

describe("EntryTypeField", () => {
  // Two radios rather than a checkbox: an unchecked box says nothing, and the
  // report needs an affirmative answer to tell an acquisition from an arrival.
  it("offers both answers, with the stored one selected", () => {
    renderField({ value: "acquired" });
    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(2);
    expect(screen.getByRole("radio", { name: /paid for it from an account/i })).toBeChecked();
    expect(screen.getByRole("radio", { name: /gift or inheritance/i })).not.toBeChecked();
  });

  // #934: every default silently misstates the first month in one common case.
  it("checks neither radio while the answer is null", () => {
    renderField({ value: null });
    for (const radio of screen.getAllByRole("radio")) expect(radio).not.toBeChecked();
  });

  it("shows the error under the radios and marks them invalid", () => {
    renderField({ value: null, error: "Pick one." });
    expect(screen.getByText("Pick one.")).toBeInTheDocument();
    const [first, second] = screen.getAllByRole("radio");
    expect(first).toHaveAttribute("aria-invalid", "true");
    expect(second).toHaveAttribute("aria-invalid", "true");
    expect(first).toHaveAttribute("aria-describedby", `${first.id}-error`);
  });

  it("reports the wire value, not the label key, to the parent", async () => {
    const { onChange } = renderField();
    await userEvent.click(screen.getByRole("radio", { name: /gift or inheritance/i }));
    expect(onChange).toHaveBeenCalledWith("newly_tracked");
  });

  it("reflects an existing declaration when editing", () => {
    renderField({ value: "newly_tracked" });
    expect(screen.getByRole("radio", { name: /gift or inheritance/i })).toBeChecked();
  });

  // A debt is never "funded from money already tracked here" — nothing funds it.
  // What makes its birth two-sided is that the money it released, or the thing
  // it bought, landed in the books too. Asking a household where a mortgage was
  // funded from reads as nonsense, so the liability wording is its own set while
  // the wire values stay identical.
  it("asks a liability how the debt was taken on, not how it was funded", async () => {
    const { onChange } = renderField({ group: "liability", value: "acquired" });
    expect(screen.getByText("When did this debt start?")).toBeInTheDocument();

    expect(
      screen.queryByRole("radio", { name: /paid for it from an account/i }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /just took it on/i })).toBeChecked();

    const arrived = screen.getByRole("radio", { name: /already owed it/i });
    await userEvent.click(arrived);
    expect(onChange).toHaveBeenCalledWith("newly_tracked");
  });

  // The other three groups are all "we paid for it", so they keep one set —
  // the split is owned-vs-owed, not one wording per group.
  it.each(["asset", "receivable", "investment"] as const)(
    "keeps the funded-from wording for %s",
    (group) => {
      renderField({ group });
      expect(
        screen.getByRole("radio", { name: /paid for it from an account/i }),
      ).toBeInTheDocument();
    },
  );

  // The radio dot itself is ~13px and cannot be enlarged; the label row is the
  // hit area, so that is what has to clear the floor.
  it("gives each option a floored hit row on phones", () => {
    renderField();
    for (const radio of screen.getAllByRole("radio")) {
      expect(radio.closest("label")).toHaveClass("max-md:min-h-11");
    }
  });

  // A list screen can mount create and edit dialogs at once; a shared
  // radio-group name would let one steal the other's selection.
  it("scopes the radio group name to the id prefix", () => {
    renderField();
    for (const radio of screen.getAllByRole("radio")) {
      expect(radio).toHaveAttribute("name", "test_entry_type");
    }
  });
});
