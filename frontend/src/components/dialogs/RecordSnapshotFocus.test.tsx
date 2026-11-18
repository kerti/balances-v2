// Record snapshot opens with its first value input focused (#944), not the month
// picker's previous-month arrow; the picker stays one Shift+Tab away.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { CreateQuantityPriceSnapshotDialog } from "@/components/dialogs/CreateQuantityPriceSnapshotDialog";
import { CreateAccruedInterestSnapshotDialog } from "@/components/dialogs/CreateAccruedInterestSnapshotDialog";

const mutation = () =>
  ({ mutate: vi.fn(), reset: vi.fn(), isError: false, isPending: false, error: null }) as never;

const open = () => userEvent.click(screen.getByRole("button", { name: "Record snapshot" }));

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 10, 9, 0, 0));
});
afterEach(() => vi.useRealTimers());

describe("Record snapshot initial focus", () => {
  it("focuses Amount for an amount-only type, and the month picker is reachable with Shift+Tab", async () => {
    renderWithProviders(
      <CreateSnapshotDialog
        currency="IDR"
        snapshots={[]}
        createMutation={mutation()}
        updateMutation={mutation()}
      />,
    );
    await open();
    expect(screen.getByLabelText(/^Amount/)).toHaveFocus();

    const trigger = screen.getByTestId("year_month");
    for (let i = 0; i < 5 && document.activeElement !== trigger; i++) {
      await userEvent.tab({ shift: true });
    }
    expect(trigger).toHaveFocus();
  });

  it("focuses Quantity for a quantity × price type", async () => {
    renderWithProviders(
      <CreateQuantityPriceSnapshotDialog
        currency="USD"
        snapshots={[]}
        createMutation={mutation()}
        updateMutation={mutation()}
      />,
    );
    await open();
    expect(screen.getByLabelText("Quantity")).toHaveFocus();
  });

  it("focuses Total value for a bond, and Value for a time deposit", async () => {
    const { unmount } = renderWithProviders(
      <CreateAccruedInterestSnapshotDialog
        currency="IDR"
        snapshots={[]}
        createMutation={mutation()}
        updateMutation={mutation()}
      />,
    );
    await open();
    expect(screen.getByLabelText(/^Total value/)).toHaveFocus();
    unmount();

    renderWithProviders(
      <CreateAccruedInterestSnapshotDialog
        currency="IDR"
        snapshots={[]}
        createMutation={mutation()}
        updateMutation={mutation()}
        principal="100000000"
      />,
    );
    await open();
    expect(screen.getByLabelText("Value (IDR)")).toHaveFocus();
  });
});
