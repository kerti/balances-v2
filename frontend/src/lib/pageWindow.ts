// Which page numbers a pagination control should actually render (#572).
//
// `PaginationControls` used to render one link per page, unconditionally, so the
// control's width grew with the data: a household with a few years of Income
// rows pushed the last pages (and the Next arrow with them) off the right edge
// of a phone screen, unreachable. The `PaginationEllipsis` primitive to fix it
// with had existed, unused, since the control was written.
//
// The window keeps a **constant item count** (`siblings * 2 + 5`) at every page,
// so the control does not resize or reflow its neighbours as you page through —
// near the ends the run of numbers extends to fill what the missing ellipsis
// would have occupied, rather than the whole row shrinking. First and last are
// always reachable in one tap, which is what makes a truncated control as
// capable as the full one for the two moves people actually make (step through,
// or jump to an end).
export type PageItem = number | "ellipsis";

function range(from: number, to: number): number[] {
  return Array.from({ length: to - from + 1 }, (_, i) => from + i);
}

// `siblings` is how many pages flank the current one. The caller picks it by
// available width, not by taste: 1 on the web, 0 on a phone, where the items are
// at the 44px tap floor and seven of them is all a 390px viewport holds.
export function pageWindow(page: number, totalPages: number, siblings = 1): PageItem[] {
  if (totalPages < 1) return [];

  const current = Math.min(Math.max(page, 1), totalPages);
  // first + last + current + both sibling runs + both ellipses.
  const slots = siblings * 2 + 5;
  if (totalPages <= slots) return range(1, totalPages);

  // The run that replaces an omitted ellipsis: current + siblings + the two
  // slots the ellipsis and its page would have taken.
  const run = siblings * 2 + 3;
  const nearStart = current - siblings <= 2;
  const nearEnd = current + siblings >= totalPages - 1;

  if (nearStart) return [...range(1, run), "ellipsis", totalPages];
  if (nearEnd) return [1, "ellipsis", ...range(totalPages - run + 1, totalPages)];

  // An ellipsis standing in for a single hidden page is worse than the page: it
  // costs the same slot, reads as "there is more here" and hides nothing. So
  // each gap collapses to the page itself when that's all it covers — which also
  // keeps the item count constant, since it's one item either way.
  const first = current - siblings;
  const last = current + siblings;
  const leftGap: PageItem = first - 1 === 2 ? 2 : "ellipsis";
  const rightGap: PageItem = last + 1 === totalPages - 1 ? totalPages - 1 : "ellipsis";
  return [1, leftGap, ...range(first, last), rightGap, totalPages];
}
