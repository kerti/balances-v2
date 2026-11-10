import { describe, it, expect } from "vitest";
import { copyCandidates, copyDate, previousMonth } from "@/lib/incomeCopy";
import type { Income } from "@/api/types";

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

describe("previousMonth", () => {
  it("rolls January back a year", () => {
    expect(previousMonth("2026-01")).toBe("2025-12");
    expect(previousMonth("2026-07")).toBe("2026-06");
  });
});

describe("copyDate", () => {
  it("keeps the day when the month is long enough", () => {
    expect(copyDate("2026-05-15", "2026-06")).toBe("2026-06-15");
  });
  it("clamps the 31st to the target month's last day", () => {
    expect(copyDate("2026-05-31", "2026-06")).toBe("2026-06-30");
    expect(copyDate("2026-01-31", "2026-02")).toBe("2026-02-28");
    expect(copyDate("2023-12-31", "2024-02")).toBe("2024-02-29");
  });
});

describe("copyCandidates", () => {
  it("offers only the previous month's Regular events", () => {
    const rows = [
      inc({ id: "a", date: "2026-05-25", category: "salary" }),
      inc({ id: "b", date: "2026-05-31", category: "pension" }),
      inc({ id: "c", date: "2026-05-12", category: "gift", regularity: "incidental" }),
      inc({ id: "d", date: "2026-04-01", category: "rental_income" }),
    ];
    const got = copyCandidates(rows, "2026-06");
    expect(got.map((c) => [c.source.id, c.date])).toEqual([
      ["a", "2026-06-25"],
      ["b", "2026-06-30"],
    ]);
    expect(got.every((c) => !c.alreadyThere)).toBe(true);
  });

  it("flags a source that already has a Regular event in the target month", () => {
    const rows = [
      inc({ id: "a", date: "2026-05-25", category: "salary", description: "Base" }),
      inc({ id: "b", date: "2026-05-25", category: "salary", description: "Per diem" }),
      inc({ id: "n", date: "2026-06-25", category: "salary", description: " base " }),
    ];
    const got = copyCandidates(rows, "2026-06");
    expect(got.map((c) => [c.source.id, c.alreadyThere])).toEqual([
      ["a", true],
      ["b", false],
    ]);
  });

  it("does not treat a One-off in the target month as the same source", () => {
    const rows = [
      inc({ id: "a", date: "2026-05-25" }),
      inc({ id: "n", date: "2026-06-25", regularity: "incidental" }),
    ];
    expect(copyCandidates(rows, "2026-06")[0].alreadyThere).toBe(false);
  });

  it("is empty when the previous month has none", () => {
    expect(copyCandidates([inc({ date: "2026-03-01" })], "2026-06")).toEqual([]);
  });
});
