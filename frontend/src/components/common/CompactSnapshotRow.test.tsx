// The phone snapshot-history row (#952): month · total · ⋯, note line only when a
// note exists, expand-on-tap breakdown only for qty×price / accrued rows.
//
// covers: INV-PRESENTATION-36
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { SnapshotCard } from "@/components/common/SnapshotCard";
import { QuantityPriceSnapshotCard } from "@/components/common/QuantityPriceSnapshotCard";
import { AccruedInterestSnapshotCard } from "@/components/common/AccruedInterestSnapshotCard";

const mutation = { mutate: vi.fn(), isPending: false } as never;

const base = {
  id: "s1",
  year_month: "2026-06-01T00:00:00Z",
  amount: "1050.00",
  currency: "USD",
  as_of_date: null,
  description: null as string | null,
};

describe("phone snapshot rows", () => {
  it("shows the note line only when the snapshot has a note", () => {
    const { unmount } = renderWithProviders(
      <SnapshotCard snapshot={base} updateMutation={mutation} deleteMutation={mutation} />,
    );
    expect(screen.queryByTestId("snapshot-note")).not.toBeInTheDocument();
    unmount();

    renderWithProviders(
      <SnapshotCard
        snapshot={{ ...base, description: "Statement checked" }}
        updateMutation={mutation}
        deleteMutation={mutation}
      />,
    );
    const note = screen.getByTestId("snapshot-note");
    expect(note).toHaveTextContent("Statement checked");
    expect(note).toHaveClass("truncate");
  });

  it("keeps the statement date on the second line, beside the note", () => {
    renderWithProviders(
      <SnapshotCard
        snapshot={{ ...base, as_of_date: "2026-06-25", description: "Bonus included" }}
        updateMutation={mutation}
        deleteMutation={mutation}
      />,
    );
    const line = screen.getByTestId("snapshot-note");
    expect(line).toHaveTextContent(/25/);
    expect(line).toHaveTextContent(/ · Bonus included$/);
  });

  it("gives plain rows no expand toggle, only the row menu", () => {
    renderWithProviders(
      <SnapshotCard snapshot={base} updateMutation={mutation} deleteMutation={mutation} />,
    );
    expect(screen.getByTestId("snapshot-amount")).toHaveTextContent(/1,?050/);
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(1);
    expect(buttons[0]).toHaveAccessibleName(/snapshot actions/i);
    expect(screen.queryByText("Month")).not.toBeInTheDocument();
  });

  it("expands a qty×price row to quantity and price, and collapses again", async () => {
    renderWithProviders(
      <QuantityPriceSnapshotCard
        snapshot={{ ...base, quantity: "60", price_per_unit: "17.50" }}
        quantityUnit="sh"
        updateMutation={mutation}
        deleteMutation={mutation}
      />,
    );
    expect(screen.queryByTestId("snapshot-breakdown")).not.toBeInTheDocument();
    const toggle = screen.getByRole("button", { name: /2026/, expanded: false });
    await userEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const panel = screen.getByTestId("snapshot-breakdown");
    expect(panel).toHaveTextContent("Quantity");
    expect(panel).toHaveTextContent(/60 sh/);
    expect(panel).toHaveTextContent("Price");
    await userEvent.click(toggle);
    expect(screen.queryByTestId("snapshot-breakdown")).not.toBeInTheDocument();
  });

  it("expands an accrued row to principal and accrued", async () => {
    renderWithProviders(
      <AccruedInterestSnapshotCard
        snapshot={{ ...base, accrued_interest: "50.00" }}
        updateMutation={mutation}
        deleteMutation={mutation}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /2026/, expanded: false }));
    const panel = screen.getByTestId("snapshot-breakdown");
    expect(panel).toHaveTextContent("Principal");
    expect(panel).toHaveTextContent(/1,?000/);
    expect(panel).toHaveTextContent("Accrued");
  });

  it("does not toggle the breakdown when the row menu is opened", async () => {
    renderWithProviders(
      <AccruedInterestSnapshotCard
        snapshot={{ ...base, accrued_interest: "50.00" }}
        updateMutation={mutation}
        deleteMutation={mutation}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /snapshot actions/i }));
    expect(screen.queryByTestId("snapshot-breakdown")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /2026/ })).toHaveAttribute("aria-expanded", "false");
  });
});
