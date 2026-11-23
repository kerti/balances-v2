// Pure projection for the Tag breakdown report (ADR-0028, amended #937). The
// backend serves one month already converted to the reporting currency; this
// joins each entry to its Tag's name/colour, sorts, and totals. Holdings
// (asset + receivable + investment) and liabilities stay separate so the report
// can show liabilities as their own negative line rather than netting them into
// the holdings pie.
import type { Tag, TagBreakdown } from "@/api/types";
import { UNTAGGED_COLOR, displayTagColor } from "@/lib/tagColors";

export type TagCell = {
  tagId: string | null;
  name: string;
  color: string;
  holdings: number; // Σ asset + receivable + investment, reporting currency
  liabilities: number; // Σ liability (a positive magnitude)
  net: number; // holdings − liabilities
};

export type TagBreakdownView = {
  currency: string; // the reporting currency every amount is in
  cells: TagCell[]; // sorted: holdings desc, Untagged last
  totalHoldings: number;
  totalLiabilities: number;
};

// cellKey is the stable render/selection key for a breakdown cell: the tag id,
// or "untagged" for the null bucket. Shared by the desktop and mobile breakdown
// renderers so the pie-inclusion checkbox toggles the same key in both.
export function cellKey(c: Pick<TagCell, "tagId">) {
  return c.tagId ?? "untagged";
}

export function projectTagBreakdown(
  data: TagBreakdown,
  tags: Tag[],
  untaggedLabel: string,
): TagBreakdownView {
  const tagById = new Map(tags.map((t) => [t.id, t]));
  const cells: TagCell[] = [];
  let totalHoldings = 0;
  let totalLiabilities = 0;

  for (const e of data.tags) {
    const holdings = Number(e.holdings);
    const liabilities = Number(e.liabilities);
    if (!Number.isFinite(holdings) || !Number.isFinite(liabilities)) continue;
    // An id missing from /api/tags (e.g. just deleted) reads as Untagged styling.
    const tag = e.tag_id === null ? undefined : tagById.get(e.tag_id);
    cells.push({
      tagId: e.tag_id,
      name: tag?.name ?? untaggedLabel,
      color: tag ? displayTagColor(tag.color) : UNTAGGED_COLOR,
      holdings,
      liabilities,
      net: holdings - liabilities,
    });
    totalHoldings += holdings;
    totalLiabilities += liabilities;
  }

  cells.sort((a, b) => {
    // Untagged always sinks to the bottom; the rest by holdings desc.
    if (a.tagId === null) return 1;
    if (b.tagId === null) return -1;
    return b.holdings - a.holdings;
  });

  return { currency: data.reporting_currency, cells, totalHoldings, totalLiabilities };
}
