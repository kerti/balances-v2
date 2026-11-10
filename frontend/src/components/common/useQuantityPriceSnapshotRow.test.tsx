import { describe, it, expect } from "vitest";
import { renderHook } from "@testing-library/react";
import type { UseMutationResult } from "@tanstack/react-query";
import i18n from "@/i18n";
import type { UpdateQuantityPriceSnapshotMutationVariables } from "@/components/dialogs/EditQuantityPriceSnapshotDialog";
import {
  useQuantityPriceSnapshotRow,
  type QuantityPriceSnapshotLike,
} from "@/components/common/useQuantityPriceSnapshotRow";

const update = {} as UseMutationResult<
  unknown,
  unknown,
  UpdateQuantityPriceSnapshotMutationVariables
>;
const remove = {} as UseMutationResult<unknown, unknown, string>;

function snap(quantity: string): QuantityPriceSnapshotLike {
  return {
    id: "s1",
    year_month: "2026-10-01T00:00:00Z",
    amount: "18751.48",
    currency: "IDR",
    quantity,
    price_per_unit: "12.5",
    as_of_date: null,
    description: null,
  };
}

// covers: INV-PRESENTATION-01
describe("useQuantityPriceSnapshotRow quantityText (#801)", () => {
  it("shows a stored 8-dp quantity grouped, with every significant place", async () => {
    await i18n.changeLanguage("en-GB");
    const { result } = renderHook(() =>
      useQuantityPriceSnapshotRow(snap("1500.12345678"), "sh", update, remove),
    );
    expect(result.current.quantityText).toBe("1,500.12345678 sh");
  });

  it("drops padded zeros instead of showing the stored precision", async () => {
    await i18n.changeLanguage("en-GB");
    const { result } = renderHook(() =>
      useQuantityPriceSnapshotRow(snap("10.00000000"), "g", update, remove),
    );
    expect(result.current.quantityText).toBe("10 g");
  });
});
