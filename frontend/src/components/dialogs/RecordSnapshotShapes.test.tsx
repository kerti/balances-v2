// Record snapshot for the two investment shapes (#788): quantity × price and
// accrued interest. Same rules as the amount-only dialog: an empty month starts
// empty with "Same as last month", a recorded month replaces through the update
// mutation with the fields prefilled.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateQuantityPriceSnapshotDialog } from "@/components/dialogs/CreateQuantityPriceSnapshotDialog";
import { CreateAccruedInterestSnapshotDialog } from "@/components/dialogs/CreateAccruedInterestSnapshotDialog";

function stubMutation() {
  const mutate = vi.fn();
  return {
    mutate,
    mutation: { mutate, reset: vi.fn(), isError: false, isPending: false, error: null } as never,
  };
}

type Snap = {
  id: string;
  year_month: string;
  amount: string;
  as_of_date: string | null;
  description: string | null;
};
type QuantityPriceSnap = Snap & { quantity: string; price_per_unit: string };
type AccruedSnap = Snap & { accrued_interest: string };

const base = <T extends object>(id: string, ym: string, extra: T): Snap & T => ({
  id,
  year_month: `${ym}-01T00:00:00Z`,
  amount: "0",
  as_of_date: null,
  description: null,
  ...extra,
});

const open = () => userEvent.click(screen.getByRole("button", { name: "Record snapshot" }));

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 10, 9, 0, 0));
});
afterEach(() => vi.useRealTimers());

describe("Record snapshot, quantity × price", () => {
  const history = [
    base("s1", "2026-09", {
      amount: "6600",
      quantity: "60",
      price_per_unit: "110",
      as_of_date: "2026-09-29",
    }),
  ];

  function setup(snapshots: QuantityPriceSnap[]) {
    const create = stubMutation();
    const update = stubMutation();
    renderWithProviders(
      <CreateQuantityPriceSnapshotDialog
        currency="USD"
        snapshots={snapshots}
        createMutation={create.mutation}
        updateMutation={update.mutation}
      />,
    );
    return { create: create.mutate, update: update.mutate };
  }

  it("starts empty, and Same as last month fills quantity and price but not the date", async () => {
    const { create } = setup(history);
    await open();

    expect(screen.getByLabelText("Quantity")).toHaveValue("");
    expect(screen.getByLabelText(/Price per unit/)).toHaveValue("");
    expect(screen.getByTestId("snapshot-last-recorded")).toHaveTextContent(
      /Last recorded:.*6,600.*\(Sep 2026\)/,
    );

    await userEvent.click(screen.getByTestId("snapshot-same-as-last"));

    expect(screen.getByLabelText("Quantity")).toHaveValue("60");
    expect(screen.getByLabelText(/Price per unit/)).toHaveValue("110");
    expect(screen.getByLabelText("Statement date (optional)")).toHaveValue("");

    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        year_month: "2026-10",
        quantity: "60",
        price_per_unit: "110",
        amount: "6600",
        as_of_date: null,
      }),
      expect.anything(),
    );
  });

  // covers: INV-SNAPSHOTS-12
  it("replaces a recorded month through the update mutation", async () => {
    const { create, update } = setup([
      base("s9", "2026-11", {
        amount: "7000",
        quantity: "70",
        price_per_unit: "100",
      }),
      ...history,
    ]);
    await open();

    expect(screen.getByTestId("snapshot-replace-notice")).toHaveTextContent(
      /Nov 2026 already has .*7,000/,
    );
    expect(screen.getByLabelText("Quantity")).toHaveValue("70");
    expect(screen.getByLabelText(/Price per unit/)).toHaveValue("100");

    await userEvent.clear(screen.getByLabelText("Quantity"));
    await userEvent.type(screen.getByLabelText("Quantity"), "80");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    expect(update).toHaveBeenCalledWith(
      {
        snapshotId: "s9",
        payload: expect.objectContaining({
          quantity: "80",
          price_per_unit: "100",
          amount: "8000",
          accrued_interest: null,
        }),
      },
      expect.anything(),
    );
    expect(create).not.toHaveBeenCalled();
  });
});

describe("Record snapshot, time deposit (one Value, #844)", () => {
  const history = [
    base("s1", "2026-09", {
      amount: "101500000",
      accrued_interest: "1500000",
      as_of_date: "2026-09-30",
    }),
  ];

  function setup(snapshots: AccruedSnap[]) {
    const create = stubMutation();
    const update = stubMutation();
    renderWithProviders(
      <CreateAccruedInterestSnapshotDialog
        currency="IDR"
        snapshots={snapshots}
        createMutation={create.mutation}
        updateMutation={update.mutation}
        principal="100000000"
      />,
    );
    return { create: create.mutate, update: update.mutate };
  }

  // covers: INV-SNAPSHOTS-17
  it("shows one Value input and no Accrued input", async () => {
    setup(history);
    await open();

    expect(screen.getByLabelText("Value (IDR)")).toHaveValue("");
    expect(screen.queryByLabelText(/Accrued/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Total value/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Of which principal/)).not.toBeInTheDocument();
  });

  // covers: INV-SNAPSHOTS-17
  it("derives the interest live as Value minus the principal, and warns below it", async () => {
    setup(history);
    await open();
    const interest = screen.getByTestId("snapshot-interest");
    expect(interest).toHaveTextContent("—");

    await userEvent.type(screen.getByLabelText("Value (IDR)"), "101500000");
    expect(interest).toHaveTextContent(/Of which interest:.*1,500,000/);
    expect(screen.queryByTestId("snapshot-interest-warning")).not.toBeInTheDocument();

    // Equal to the principal: zero interest, no warning.
    await userEvent.clear(screen.getByLabelText("Value (IDR)"));
    await userEvent.type(screen.getByLabelText("Value (IDR)"), "100000000");
    expect(interest).toHaveTextContent(/Of which interest:.*0/);
    expect(screen.queryByTestId("snapshot-interest-warning")).not.toBeInTheDocument();

    // Below it: negative interest, a soft warning.
    await userEvent.clear(screen.getByLabelText("Value (IDR)"));
    await userEvent.type(screen.getByLabelText("Value (IDR)"), "99000000");
    expect(interest).toHaveTextContent(/-.*1,000,000/);
    expect(screen.getByTestId("snapshot-interest-warning")).toHaveTextContent(
      "Value is below the principal",
    );
  });

  // covers: INV-SNAPSHOTS-17
  it("saves below the principal, sending no accrued_interest for the server to derive", async () => {
    const { create } = setup(history);
    await open();

    await userEvent.type(screen.getByLabelText("Value (IDR)"), "99000000");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        year_month: "2026-10",
        amount: "99000000",
        accrued_interest: null,
        quantity: null,
        price_per_unit: null,
      }),
      expect.anything(),
    );
  });

  it("Same as last month fills the Value, and a recorded month replaces through update", async () => {
    const { create, update } = setup([
      base("s9", "2026-11", { amount: "102000000", accrued_interest: "2000000" }),
      ...history,
    ]);
    await open();

    // The recorded month opens in replace mode with its Value prefilled.
    expect(screen.getByLabelText("Value (IDR)")).toHaveValue("102,000,000");
    expect(screen.getByTestId("snapshot-interest")).toHaveTextContent(/2,000,000/);

    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    expect(update).toHaveBeenCalledWith(
      {
        snapshotId: "s9",
        payload: expect.objectContaining({ amount: "102000000", accrued_interest: null }),
      },
      expect.anything(),
    );
    expect(create).not.toHaveBeenCalled();
  });
});

describe("Record snapshot, accrued interest", () => {
  const history = [
    base("s1", "2026-09", {
      amount: "50250000",
      accrued_interest: "250000",
      as_of_date: "2026-09-30",
    }),
  ];

  function setup(snapshots: AccruedSnap[]) {
    const create = stubMutation();
    const update = stubMutation();
    renderWithProviders(
      <CreateAccruedInterestSnapshotDialog
        currency="IDR"
        snapshots={snapshots}
        createMutation={create.mutation}
        updateMutation={update.mutation}
        couponDisposition="accrues"
      />,
    );
    return { create: create.mutate, update: update.mutate };
  }

  it("starts empty, and Same as last month fills total and accrued but not the date", async () => {
    const { create } = setup(history);
    await open();

    expect(screen.getByLabelText(/Total value/)).toHaveValue("");
    expect(screen.getByLabelText(/Accrued/)).toHaveValue("");

    await userEvent.click(screen.getByTestId("snapshot-same-as-last"));

    expect(screen.getByLabelText(/Total value/)).toHaveValue("50,250,000");
    expect(screen.getByLabelText(/Accrued/)).toHaveValue("250,000");
    expect(screen.getByLabelText("Statement date (optional)")).toHaveValue("");

    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));
    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        year_month: "2026-10",
        amount: "50250000",
        accrued_interest: "250000",
        as_of_date: null,
      }),
      expect.anything(),
    );
  });

  // covers: INV-SNAPSHOTS-12
  it("replaces a recorded month through the update mutation", async () => {
    const { create, update } = setup([
      base("s9", "2026-11", { amount: "51000000", accrued_interest: "300000" }),
      ...history,
    ]);
    await open();

    expect(screen.getByTestId("snapshot-replace-notice")).toHaveTextContent(
      /Nov 2026 already has .*51,000,000/,
    );
    expect(screen.getByLabelText(/Total value/)).toHaveValue("51,000,000");
    expect(screen.getByLabelText(/Accrued/)).toHaveValue("300,000");

    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    expect(update).toHaveBeenCalledWith(
      {
        snapshotId: "s9",
        payload: expect.objectContaining({
          amount: "51000000",
          accrued_interest: "300000",
          quantity: null,
          price_per_unit: null,
        }),
      },
      expect.anything(),
    );
    expect(create).not.toHaveBeenCalled();
  });
});
