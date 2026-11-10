import { describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CopyRegularIncomeDialog } from "@/components/dialogs/CopyRegularIncomeDialog";
import type { Income } from "@/api/types";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());

function inc(over: Partial<Income>): Income {
  return {
    id: "i",
    household_id: "h",
    date: "2026-05-10",
    amount: "100",
    currency: "IDR",
    category: "salary",
    description: null,
    ownership_type: "joint",
    sole_owner_user_id: null,
    regularity: "routine",
    created_by: null,
    created_at: "",
    updated_by: null,
    updated_at: "",
    ...over,
  };
}

const may = [
  inc({ id: "sal", date: "2026-05-25", amount: "15000000", category: "salary" }),
  inc({ id: "pen", date: "2026-05-31", amount: "2000000", category: "pension" }),
  inc({ id: "gft", date: "2026-05-12", amount: "500", category: "gift", regularity: "incidental" }),
];

function captureBulk() {
  const bodies: { rows: Record<string, unknown>[] }[] = [];
  server.use(
    http.post("/api/income/bulk", async ({ request }) => {
      const body = (await request.json()) as { rows: Record<string, unknown>[] };
      bodies.push(body);
      return HttpResponse.json(body.rows, { status: 201 });
    }),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  return bodies;
}

describe("CopyRegularIncomeDialog", () => {
  it("renders nothing when the previous month has no Regular events", () => {
    renderWithProviders(
      <CopyRegularIncomeDialog income={[may[2], inc({ date: "2026-03-01" })]} month="2026-06" />,
    );
    expect(screen.queryByTestId("copy-income-trigger")).not.toBeInTheDocument();
  });

  it("prefills Regular rows only, clamps the 31st, and saves them in one request and one toast", async () => {
    const bodies = captureBulk();
    renderWithProviders(<CopyRegularIncomeDialog income={may} month="2026-06" />);
    await userEvent.click(screen.getByTestId("copy-income-trigger"));

    expect(await screen.findAllByTestId("copy-income-row")).toHaveLength(2);
    await userEvent.click(screen.getByTestId("copy-income-save"));

    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0].rows).toEqual([
      expect.objectContaining({ date: "2026-06-25", amount: "15000000", category: "salary" }),
      expect.objectContaining({ date: "2026-06-30", amount: "2000000", category: "pension" }),
    ]);
    await waitFor(() => expect(raisedToasts(toast as never)).toHaveLength(1));
  });

  it("saves edited amounts and skips unticked rows", async () => {
    const bodies = captureBulk();
    renderWithProviders(<CopyRegularIncomeDialog income={may} month="2026-06" />);
    await userEvent.click(screen.getByTestId("copy-income-trigger"));

    const amount = await screen.findByTestId("copy-income-amount-0");
    await userEvent.clear(amount);
    await userEvent.type(amount, "16000000");
    await userEvent.click(screen.getByTestId("copy-income-tick-1"));
    await userEvent.click(screen.getByTestId("copy-income-save"));

    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0].rows).toHaveLength(1);
    expect(bodies[0].rows[0]).toEqual(
      expect.objectContaining({ category: "salary", amount: "16000000" }),
    );
  });

  it("flags and unticks a source already copied into the month", async () => {
    const bodies = captureBulk();
    const copied = inc({ id: "new", date: "2026-06-25", amount: "15000000", category: "salary" });
    renderWithProviders(<CopyRegularIncomeDialog income={[...may, copied]} month="2026-06" />);
    await userEvent.click(screen.getByTestId("copy-income-trigger"));

    expect(await screen.findByTestId("copy-income-flag-0")).toHaveTextContent(/already has/i);
    expect(screen.getByTestId("copy-income-tick-0")).not.toBeChecked();
    expect(screen.getByTestId("copy-income-tick-1")).toBeChecked();
    expect(screen.queryByTestId("copy-income-flag-1")).not.toBeInTheDocument();

    await userEvent.click(screen.getByTestId("copy-income-save"));
    await waitFor(() => expect(bodies).toHaveLength(1));
    expect(bodies[0].rows).toHaveLength(1);
    expect(bodies[0].rows[0]).toEqual(expect.objectContaining({ category: "pension" }));
  });

  it("writes nothing on cancel, and explains when no row is ticked", async () => {
    const bodies = captureBulk();
    renderWithProviders(<CopyRegularIncomeDialog income={may} month="2026-06" />);
    await userEvent.click(screen.getByTestId("copy-income-trigger"));
    await userEvent.click(await screen.findByTestId("copy-income-tick-0"));
    await userEvent.click(screen.getByTestId("copy-income-tick-1"));
    await userEvent.click(screen.getByTestId("copy-income-save"));
    expect(await screen.findByTestId("field-error-copy_rows")).toBeInTheDocument();
    expect(bodies).toHaveLength(0);

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(bodies).toHaveLength(0);
    expect(screen.queryByTestId("copy-income-row")).not.toBeInTheDocument();
  });
});
