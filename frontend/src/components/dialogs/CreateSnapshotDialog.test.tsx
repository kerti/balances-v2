// The month field is a MonthPickerPopover, not a native month input,
// but the payload keeps the "YYYY-MM" shape the API expects.
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { renderWithProviders } from "@/test/renderWithProviders";
import {
  CreateSnapshotDialog,
  type CreateSnapshotPayload,
} from "@/components/dialogs/CreateSnapshotDialog";

describe("CreateSnapshotDialog month picker", () => {
  it("submits the picked month as YYYY-MM", async () => {
    const mutate = vi.fn();
    const mutation = {
      mutate,
      reset: vi.fn(),
      isError: false,
      isPending: false,
      error: null,
    } as unknown as UseMutationResult<unknown, unknown, CreateSnapshotPayload>;
    renderWithProviders(<CreateSnapshotDialog currency="IDR" mutation={mutation} />);

    await userEvent.click(screen.getByRole("button", { name: /new/i }));
    await userEvent.click(screen.getByLabelText("Month"));
    await userEvent.click(await screen.findByTestId("month-picker-cell-2026-02"));
    await userEvent.type(screen.getByLabelText(/Amount/), "100");
    await userEvent.click(screen.getByRole("button", { name: "Save snapshot" }));

    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ year_month: "2026-02", amount: "100", currency: "IDR" }),
      expect.anything(),
    );
  });
});
