import { describe, it, expect } from "vitest";
import { projectTagBreakdown } from "@/lib/tagBreakdown";
import { UNTAGGED_COLOR } from "@/lib/tagColors";
import type { Tag, TagBreakdown } from "@/api/types";

const tag = (id: string, name: string, color: string): Tag => ({
  id,
  household_id: "h1",
  name,
  color,
  created_by: null,
  created_at: "",
  updated_by: null,
  updated_at: "",
  deleted_at: null,
});

const data = (tags: TagBreakdown["tags"]): TagBreakdown => ({
  reporting_currency: "IDR",
  year_month: "2026-11-01T00:00:00Z",
  tags,
  missing_fx: [],
  fx_rates_used: {},
});

const entry = (tag_id: string | null, holdings: string, liabilities = "0") => ({
  tag_id,
  holdings,
  liabilities,
});

const tags = [tag("t1", "Alpha", "#3b82f6"), tag("t2", "Beta", "#10b981")];

// covers: INV-TAGS-05
describe("projectTagBreakdown", () => {
  it("returns no cells for no tags and carries the reporting currency", () => {
    const bd = projectTagBreakdown(data([]), tags, "Untagged");
    expect(bd.cells).toEqual([]);
    expect(bd.currency).toBe("IDR");
    expect(bd.totalHoldings).toBe(0);
  });

  it("keeps liabilities separate and nets them off holdings", () => {
    const bd = projectTagBreakdown(data([entry("t1", "160", "40")]), tags, "Untagged");
    expect(bd.cells).toHaveLength(1);
    expect(bd.cells[0]).toMatchObject({
      tagId: "t1",
      name: "Alpha",
      color: "#3b82f6",
      holdings: 160,
      liabilities: 40,
      net: 120,
    });
    expect(bd.totalHoldings).toBe(160);
    expect(bd.totalLiabilities).toBe(40);
  });

  it("puts a null tag_id into an Untagged cell with the muted colour", () => {
    const bd = projectTagBreakdown(data([entry(null, "70")]), tags, "Untagged");
    expect(bd.cells[0]).toMatchObject({
      tagId: null,
      name: "Untagged",
      color: UNTAGGED_COLOR,
      holdings: 70,
    });
  });

  it("falls back to Untagged styling for a tag id not in the tag list", () => {
    const bd = projectTagBreakdown(data([entry("gone", "5")]), tags, "Untagged");
    expect(bd.cells[0]).toMatchObject({ tagId: "gone", name: "Untagged", color: UNTAGGED_COLOR });
  });

  it("sorts tags by holdings desc with Untagged always last", () => {
    const bd = projectTagBreakdown(
      data([entry(null, "999"), entry("t1", "30"), entry("t2", "80")]),
      tags,
      "Untagged",
    );
    expect(bd.cells.map((c) => c.tagId)).toEqual(["t2", "t1", null]);
  });

  it("totals across cells and drops non-finite amounts defensively", () => {
    const bd = projectTagBreakdown(
      data([entry("t1", "100", "10"), entry("t2", "50"), entry(null, "oops")]),
      tags,
      "Untagged",
    );
    expect(bd.cells).toHaveLength(2);
    expect(bd.totalHoldings).toBe(150);
    expect(bd.totalLiabilities).toBe(10);
  });
});
