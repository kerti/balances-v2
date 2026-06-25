import { Suspense } from "react";
import { lazyWithReload } from "@/lib/lazyWithReload";

type SnapshotLike = {
  year_month: string;
  amount: string;
};

type CostPoint = {
  year_month: string;
  cost: number;
};

type Props = {
  snapshots: SnapshotLike[];
  currency: string;
  // Optional parallel cost-basis series (one entry per snapshot, same
  // year_month values). When provided, renders a second line in muted
  // slate beneath the value area — the gap reads as unrealized P/L.
  // Investment detail screens pass this via `lib/costBasis`; non-
  // investment groups (assets / liabilities / receivables) omit it.
  costSeries?: CostPoint[];
  // Terminal investment status ('sold' | 'matured'). When set, the chart
  // drops the truthful 0-value close snapshot from the value line (so it
  // ends at the last real value instead of cratering to 0) and labels that
  // point with a Sold/Matured marker (#25). Non-investment groups omit it.
  status?: string | null;
};

// Lazy boundary so recharts + the shadcn chart wrapper land in a
// separate chunk, fetched only when a detail page actually renders the
// chart. The empty-snapshot short-circuit stays out here so we don't
// even request the chunk on empty data.
const SnapshotChartImpl = lazyWithReload(() => import("./SnapshotChartImpl"));

export function SnapshotChart({
  snapshots,
  currency,
  costSeries,
  status,
}: Props) {
  if (snapshots.length === 0) return null;
  return (
    <Suspense fallback={<div className="h-64 w-full" />}>
      <SnapshotChartImpl
        snapshots={snapshots}
        currency={currency}
        costSeries={costSeries}
        status={status}
      />
    </Suspense>
  );
}
