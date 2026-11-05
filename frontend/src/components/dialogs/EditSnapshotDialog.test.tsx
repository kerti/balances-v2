// The snapshot amount goes to the server as the canonical decimal string
// whatever the user typed (ADR-0056, BUG-01): before the shared NumericInput, a
// figure typed the way the app displays it failed with INVALID_JSON_BODY.
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { renderWithProviders } from "@/test/renderWithProviders";
import {
  EditSnapshotDialog,
  type UpdateSnapshotMutationVariables,
} from "@/components/dialogs/EditSnapshotDialog";

function mutationStub() {
  const mutate = vi.fn();
  const mutation = {
    mutate,
    isError: false,
    isPending: false,
    error: null,
  } as unknown as UseMutationResult<unknown, unknown, UpdateSnapshotMutationVariables>;
  return { mutate, mutation };
}

const snapshot = {
  id: "snap-1",
  year_month: "2026-01-01T00:00:00Z",
  amount: "100",
  currency: "USD",
  as_of_date: null,
  description: null,
};

// covers: INV-PRESENTATION-13
describe("EditSnapshotDialog amount", () => {
  it("prefills the stored amount grouped, and sends a phone-comma decimal as canonical", async () => {
    const { mutate, mutation } = mutationStub();
    renderWithProviders(
      <EditSnapshotDialog
        open
        onOpenChange={() => {}}
        snapshot={{ ...snapshot, amount: "1250000" }}
        mutation={mutation}
      />,
    );
    const amount = screen.getByLabelText(/Amount/) as HTMLInputElement;
    expect(amount.value).toBe("1,250,000");

    await userEvent.clear(amount);
    await userEvent.type(amount, "48500000,50");
    expect(amount.value).toBe("48,500,000.50");
    await userEvent.click(screen.getByRole("button", { name: /save/i }));

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        snapshotId: "snap-1",
        payload: expect.objectContaining({ amount: "48500000.50", currency: "USD" }),
      }),
      expect.anything(),
    );
  });
});
