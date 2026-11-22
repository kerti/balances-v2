// Edit snapshot for the quantity × price shape: the dialog previews the amount
// the server will store (quantity × price_per_unit, exact, rounded half up to 4
// places) and sends that figure (#968).
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EditQuantityPriceSnapshotDialog } from "@/components/dialogs/EditQuantityPriceSnapshotDialog";

const snapshot = {
  id: "s1",
  year_month: "2026-09-01T00:00:00Z",
  amount: "6600",
  currency: "USD",
  quantity: "60",
  price_per_unit: "110",
  as_of_date: null,
  description: null,
};

function setup() {
  const mutate = vi.fn();
  renderWithProviders(
    <EditQuantityPriceSnapshotDialog
      open
      onOpenChange={vi.fn()}
      snapshot={snapshot}
      mutation={{ mutate, reset: vi.fn(), isError: false, isPending: false, error: null } as never}
    />,
  );
  return mutate;
}

async function retype(label: string | RegExp, value: string) {
  const input = screen.getByLabelText(label);
  await userEvent.clear(input);
  await userEvent.type(input, value);
}

describe("EditQuantityPriceSnapshotDialog", () => {
  // covers: INV-SNAPSHOTS-20
  it("previews 0.1 x 3 as exactly 0.3 and sends that", async () => {
    const mutate = setup();
    expect(screen.getByTestId("snapshot-total-value")).toHaveTextContent("$6,600.00");

    await retype("Quantity", "0.1");
    await retype(/Price per unit/, "3");
    expect(screen.getByTestId("snapshot-total-value")).toHaveTextContent("$0.30");

    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(mutate).toHaveBeenCalledWith(
      {
        snapshotId: "s1",
        payload: expect.objectContaining({ quantity: "0.1", price_per_unit: "3", amount: "0.3" }),
      },
      expect.anything(),
    );
  });

  // covers: INV-SNAPSHOTS-20
  it("sends every digit of a 20-digit product", async () => {
    const mutate = setup();
    await retype("Quantity", "123456789012");
    await retype(/Price per unit/, "98765432");

    await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
    expect(mutate).toHaveBeenCalledWith(
      {
        snapshotId: "s1",
        payload: expect.objectContaining({ amount: "12193263100103033184" }),
      },
      expect.anything(),
    );
  });
});
