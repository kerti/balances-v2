// Edit snapshot for the accrued shape. A time deposit (principal passed, #844)
// takes one Value with the interest derived live and sends no accrued_interest;
// a bond keeps the total-value + accrued pair exactly as before.
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EditAccruedInterestSnapshotDialog } from "@/components/dialogs/EditAccruedInterestSnapshotDialog";

const snapshot = {
  id: "s1",
  year_month: "2026-09-01T00:00:00Z",
  amount: "101500000",
  currency: "IDR",
  accrued_interest: "1500000",
  as_of_date: null,
  description: null,
};

function setup(principal?: string) {
  const mutate = vi.fn();
  renderWithProviders(
    <EditAccruedInterestSnapshotDialog
      open
      onOpenChange={vi.fn()}
      snapshot={snapshot}
      mutation={{ mutate, reset: vi.fn(), isError: false, isPending: false, error: null } as never}
      principal={principal}
    />,
  );
  return mutate;
}

describe("EditAccruedInterestSnapshotDialog", () => {
  // covers: INV-SNAPSHOTS-17
  it("time deposit: one Value, interest derived live, no accrued_interest sent", async () => {
    const mutate = setup("100000000");

    const value = screen.getByLabelText("Value (IDR)");
    expect(value).toHaveValue("101,500,000");
    expect(screen.queryByLabelText(/Accrued/)).not.toBeInTheDocument();
    expect(screen.getByTestId("snapshot-interest")).toHaveTextContent(/1,500,000/);

    await userEvent.clear(value);
    await userEvent.type(value, "99500000");
    expect(screen.getByTestId("snapshot-interest")).toHaveTextContent(/-.*500,000/);
    expect(screen.getByTestId("snapshot-interest-warning")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(mutate).toHaveBeenCalledWith(
      {
        snapshotId: "s1",
        payload: expect.objectContaining({
          amount: "99500000",
          accrued_interest: null,
          quantity: null,
          price_per_unit: null,
        }),
      },
      expect.anything(),
    );
  });

  it("bond: keeps the total-value + accrued pair and sends accrued_interest", async () => {
    const mutate = setup();

    expect(screen.getByLabelText(/Total value/)).toHaveValue("101,500,000");
    expect(screen.getByLabelText(/Accrued/)).toHaveValue("1,500,000");
    expect(screen.queryByLabelText("Value (IDR)")).not.toBeInTheDocument();
    expect(screen.queryByTestId("snapshot-interest")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(mutate).toHaveBeenCalledWith(
      {
        snapshotId: "s1",
        payload: expect.objectContaining({ amount: "101500000", accrued_interest: "1500000" }),
      },
      expect.anything(),
    );
  });
});
