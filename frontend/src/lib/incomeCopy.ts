// Copy last month's regular income (#846): pure helpers for the review dialog.
// Income stays observed (ADR-0001/0008) — these only *propose* rows; nothing is
// written until the household reviews and saves.
import type { Income } from "@/api/types";

// "2026-03" -> "2026-02"; January rolls back into the previous year.
export function previousMonth(ym: string): string {
  const [y, m] = ym.split("-").map(Number);
  const idx = y * 12 + (m - 1) - 1;
  return `${Math.floor(idx / 12)}-${String((idx % 12) + 1).padStart(2, "0")}`;
}

function daysInMonth(ym: string): number {
  const [y, m] = ym.split("-").map(Number);
  return new Date(Date.UTC(y, m, 0)).getUTCDate();
}

// The same day of the month in `targetYm`, clamped to its last day (31 -> 30,
// 28 or 29).
export function copyDate(date: string, targetYm: string): string {
  const day = Math.min(Number(date.slice(8, 10)), daysInMonth(targetYm));
  return `${targetYm}-${String(day).padStart(2, "0")}`;
}

// Two Regular events are "the same source" when they share category, owner,
// currency and description: income has no payer column, and the description is
// what tells base pay from a per-diem within one category (CONTEXT.md).
export function incomeSourceKey(r: Income): string {
  return [
    r.category,
    r.ownership_type,
    r.sole_owner_user_id ?? "",
    r.currency,
    (r.description ?? "").trim().toLowerCase(),
  ].join("|");
}

export type CopyCandidate = {
  source: Income;
  date: string;
  // True when the target month already holds a Regular event from this source.
  alreadyThere: boolean;
};

// Every Regular (routine) event of the month before `targetYm`, dated into
// `targetYm`. One-off (incidental) events are never offered. Oldest first.
export function copyCandidates(all: Income[], targetYm: string): CopyCandidate[] {
  const from = previousMonth(targetYm);
  const existing = new Set(
    all
      .filter((r) => r.regularity === "routine" && r.date.slice(0, 7) === targetYm)
      .map(incomeSourceKey),
  );
  return all
    .filter((r) => r.regularity === "routine" && r.date.slice(0, 7) === from)
    .sort((a, b) => a.date.localeCompare(b.date) || a.id.localeCompare(b.id))
    .map((source) => ({
      source,
      date: copyDate(source.date, targetYm),
      alreadyThere: existing.has(incomeSourceKey(source)),
    }));
}
