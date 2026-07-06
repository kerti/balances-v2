import { describe, it, expect } from "vitest";
import { byText, byNumberNullsLast } from "@/lib/sort";

type Row = { name: string; amount: number | null };

const sortBy = (rows: Row[], cmp: (a: Row, b: Row) => number) =>
  [...rows].sort(cmp).map((r) => r.name);

describe("byText", () => {
  const rows: Row[] = [
    { name: "Charlie", amount: 0 },
    { name: "alice", amount: 0 },
    { name: "Bob", amount: 0 },
  ];
  const cmp = byText<Row>((r) => r.name);

  it("sorts ascending case-insensitively", () => {
    expect(sortBy(rows, (a, b) => cmp(a, b, "asc"))).toEqual(["alice", "Bob", "Charlie"]);
  });

  it("sorts descending", () => {
    expect(sortBy(rows, (a, b) => cmp(a, b, "desc"))).toEqual(["Charlie", "Bob", "alice"]);
  });
});

describe("byNumberNullsLast", () => {
  const rows: Row[] = [
    { name: "b", amount: 200 },
    { name: "none", amount: null },
    { name: "a", amount: 100 },
  ];
  const cmp = byNumberNullsLast<Row>((r) => r.amount);

  it("sorts ascending with nulls last", () => {
    expect(sortBy(rows, (a, b) => cmp(a, b, "asc"))).toEqual(["a", "b", "none"]);
  });

  it("keeps nulls last even when descending", () => {
    expect(sortBy(rows, (a, b) => cmp(a, b, "desc"))).toEqual(["b", "a", "none"]);
  });

  it("treats two nulls as equal", () => {
    expect(cmp({ name: "x", amount: null }, { name: "y", amount: null }, "asc")).toBe(0);
  });
});
