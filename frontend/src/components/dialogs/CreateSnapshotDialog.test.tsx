// Record snapshot, amount-only shape (ADR-0046 amendment, #788). The month field
// is a MonthPickerPopover, not a native month input, but the payload keeps the
// "YYYY-MM" shape the API expects. A month that already has a value switches the
// dialog to replace mode: the write goes through the update (PATCH) mutation, the
// create (POST) mutation is never called.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { renderWithProviders } from "@/test/renderWithProviders";
import {
  CreateSnapshotDialog,
  type CreateSnapshotPayload,
} from "@/components/dialogs/CreateSnapshotDialog";

function stubMutation<V>() {
  const mutate = vi.fn();
  return {
    mutate,
    mutation: {
      mutate,
      reset: vi.fn(),
      isError: false,
      isPending: false,
      error: null,
    } as unknown as UseMutationResult<unknown, unknown, V>,
  };
}

const snap = (id: string, year_month: string, amount: string, extra = {}) => ({
  id,
  year_month: `${year_month}-01T00:00:00Z`,
  amount,
  as_of_date: null,
  description: null,
  ...extra,
});

function setup(snapshots: ReturnType<typeof snap>[] | undefined) {
  const create = stubMutation<CreateSnapshotPayload>();
  const update = stubMutation<{ snapshotId: string; payload: unknown }>();
  renderWithProviders(
    <CreateSnapshotDialog
      currency="IDR"
      snapshots={snapshots}
      createMutation={create.mutation}
      updateMutation={update.mutation as never}
    />,
  );
  return { create: create.mutate, update: update.mutate };
}

const open = () => userEvent.click(screen.getByRole("button", { name: "Record snapshot" }));

describe("CreateSnapshotDialog (Record snapshot)", () => {
  beforeEach(() => {
    // Only Date: userEvent and the popover rely on real timers.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 10, 10, 9, 0, 0));
  });
  afterEach(() => vi.useRealTimers());

  it("submits the picked month as YYYY-MM", async () => {
    const { create } = setup([]);

    await open();
    await userEvent.click(screen.getByLabelText("Month"));
    await userEvent.click(await screen.findByTestId("month-picker-cell-2026-02"));
    await userEvent.type(screen.getByLabelText(/Amount/), "100");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ year_month: "2026-02", amount: "100", currency: "IDR" }),
      expect.anything(),
    );
  });

  it("a first snapshot opens on the current month with nothing to copy", async () => {
    setup([]);
    await open();

    expect(screen.getByTestId("year_month")).toHaveTextContent(/Nov/);
    expect(screen.getByLabelText(/Amount/)).toHaveValue("");
    expect(screen.queryByTestId("snapshot-replace-notice")).not.toBeInTheDocument();
    expect(screen.queryByTestId("snapshot-last-recorded")).not.toBeInTheDocument();
  });

  it("opens on the month after the latest snapshot, amount empty, with Last recorded", async () => {
    setup([snap("s2", "2026-09", "12500000"), snap("s1", "2026-08", "9000000")]);
    await open();

    // Month after the latest (Sep) is Oct, not the current Nov.
    expect(screen.getByTestId("year_month")).toHaveTextContent(/Oct/);
    expect(screen.getByLabelText(/Amount/)).toHaveValue("");
    expect(screen.queryByTestId("snapshot-replace-notice")).not.toBeInTheDocument();
    expect(screen.getByTestId("snapshot-last-recorded")).toHaveTextContent(
      /Last recorded:.*12,500,000.*\(Sep 2026\)/,
    );
  });

  it("Same as last month fills the amount and leaves the statement date blank", async () => {
    const { create } = setup([snap("s1", "2026-09", "12500000", { as_of_date: "2026-09-28" })]);
    await open();

    await userEvent.click(screen.getByTestId("snapshot-same-as-last"));

    expect(screen.getByLabelText(/Amount/)).toHaveValue("12,500,000");
    expect(screen.getByLabelText("Statement date (optional)")).toHaveValue("");

    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ year_month: "2026-10", amount: "12500000", as_of_date: null }),
      expect.anything(),
    );
  });

  // covers: INV-SNAPSHOTS-12
  it("opens in replace mode when the current month is already recorded", async () => {
    const { create, update } = setup([
      snap("s1", "2026-11", "12500000", { as_of_date: "2026-11-05", description: "BCA" }),
    ]);
    await open();

    // Capped at the current month, which already has a value.
    expect(screen.getByTestId("year_month")).toHaveTextContent(/Nov/);
    expect(screen.getByTestId("snapshot-replace-notice")).toHaveTextContent(
      /Nov 2026 already has .*12,500,000.*saving replaces it/,
    );
    expect(screen.queryByTestId("snapshot-last-recorded")).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Amount/)).toHaveValue("12,500,000");
    expect(screen.getByLabelText("Description (optional)")).toHaveValue("BCA");

    await userEvent.clear(screen.getByLabelText(/Amount/));
    await userEvent.type(screen.getByLabelText(/Amount/), "13000000");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    // Replaces the live row through the update path; never POSTs a second one.
    expect(update).toHaveBeenCalledWith(
      {
        snapshotId: "s1",
        payload: expect.objectContaining({
          amount: "13000000",
          currency: "IDR",
          as_of_date: "2026-11-05",
        }),
      },
      expect.anything(),
    );
    expect(create).not.toHaveBeenCalled();
  });

  it("switches to replace mode when an already-recorded month is picked, and back", async () => {
    const { create, update } = setup([
      snap("s2", "2026-10", "2000000"),
      snap("s1", "2026-08", "1000000"),
    ]);
    await open();
    // Latest is Oct, so it opens on Nov (empty): create mode.
    expect(screen.queryByTestId("snapshot-replace-notice")).not.toBeInTheDocument();
    await userEvent.type(screen.getByLabelText(/Amount/), "5");

    await userEvent.click(screen.getByLabelText("Month"));
    await userEvent.click(await screen.findByTestId("month-picker-cell-2026-08"));
    expect(screen.getByTestId("snapshot-replace-notice")).toHaveTextContent(/Aug 2026 already has/);
    expect(screen.getByLabelText(/Amount/)).toHaveValue("1,000,000");

    // Back to an empty month: the other month's value is not carried over.
    await userEvent.click(screen.getByLabelText("Month"));
    await userEvent.click(await screen.findByTestId("month-picker-cell-2026-09"));
    expect(screen.queryByTestId("snapshot-replace-notice")).not.toBeInTheDocument();
    expect(screen.getByLabelText(/Amount/)).toHaveValue("");
    // "Same as last month" now copies the latest snapshot before Sep: Aug.
    expect(screen.getByTestId("snapshot-last-recorded")).toHaveTextContent(/\(Aug 2026\)/);

    await userEvent.type(screen.getByLabelText(/Amount/), "7");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ year_month: "2026-09", amount: "7" }),
      expect.anything(),
    );
    expect(update).not.toHaveBeenCalled();
  });
});
