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
    { recordButton: true },
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
    expect(screen.getByLabelText("Note (optional)")).toHaveValue("BCA");

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

  // #953 S3
  it("opens on the first month with no value when the current month is recorded", async () => {
    setup([snap("s3", "2026-11", "3000000"), snap("s1", "2026-08", "1000000")]);
    await open();
    // Nov is recorded; the latest empty month before it is Oct, not a replace.
    expect(screen.getByTestId("year_month")).toHaveTextContent(/Oct/);
    expect(screen.queryByTestId("snapshot-replace-notice")).not.toBeInTheDocument();
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

    await userEvent.type(screen.getByLabelText(/Amount/), "1100000");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ year_month: "2026-09", amount: "1100000" }),
      expect.anything(),
    );
    expect(update).not.toHaveBeenCalled();
  });

  // #933: a single-entry save 10x or a tenth of the last recorded month asks first.
  // covers: INV-SNAPSHOTS-19
  describe("10x / a tenth swing", () => {
    const history = [snap("s1", "2026-10", "5000000")];
    const save = () => userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    const type = (text: string) => userEvent.type(screen.getByLabelText(/Amount/), text);

    it("asks before saving a figure 10x the last one, and saves on Save anyway", async () => {
      const { create } = setup(history);
      await open();
      await type("50000000");
      await save();

      expect(create).not.toHaveBeenCalled();
      const dialog = await screen.findByRole("alertdialog");
      expect(dialog).toHaveTextContent("Is this value right?");
      expect(dialog).toHaveTextContent(/50,000,000.*10 times or more.*5,000,000.*\(Oct 2026\)/);

      await userEvent.click(screen.getByRole("button", { name: "Save anyway" }));
      expect(create).toHaveBeenCalledTimes(1);
      expect(create).toHaveBeenCalledWith(
        expect.objectContaining({ year_month: "2026-11", amount: "50000000" }),
        expect.anything(),
      );
    });

    it("asks before saving a tenth, and Go back leaves the form as it was", async () => {
      const { create } = setup(history);
      await open();
      await type("500000");
      await save();

      expect(await screen.findByRole("alertdialog")).toHaveTextContent(
        /500,000.*a tenth or less.*5,000,000/,
      );
      await userEvent.click(screen.getByRole("button", { name: "Go back" }));
      expect(create).not.toHaveBeenCalled();
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(screen.getByLabelText(/Amount/)).toHaveValue("500,000");
    });

    it.each([
      ["an ordinary change", history, "6000000"],
      ["a zero", history, "0"],
      ["a first snapshot", [], "50000000"],
      ["a zero prior", [snap("z", "2026-10", "0")], "50000000"],
    ])("does not ask for %s", async (_name, snapshots, amount) => {
      const { create } = setup(snapshots);
      await open();
      await type(amount);
      await save();
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(create).toHaveBeenCalledTimes(1);
    });

    it("asks in replace mode too, against the month before, and replaces through update", async () => {
      const { create, update } = setup([
        snap("s2", "2026-11", "5200000"),
        snap("s1", "2026-10", "5000000"),
      ]);
      await open();
      expect(screen.getByLabelText(/Amount/)).toHaveValue("5,200,000");
      await userEvent.clear(screen.getByLabelText(/Amount/));
      await type("52000000");
      await save();

      expect(await screen.findByRole("alertdialog")).toHaveTextContent(/5,000,000.*\(Oct 2026\)/);
      await userEvent.click(screen.getByRole("button", { name: "Save anyway" }));
      expect(update).toHaveBeenCalledTimes(1);
      expect(create).not.toHaveBeenCalled();
    });
  });
});
