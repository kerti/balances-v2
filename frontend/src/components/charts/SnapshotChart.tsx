import { Suspense } from "react";
import { lazyWithReload } from "@/lib/lazyWithReload";
import { RangeChips } from "./RangeChips";
import { useRangeZoom } from "./useRangeZoom";
import { monthRange } from "@/lib/months";

type SnapshotLike = {
  year_month: string;
  amount: string;
};

type CostPoint = {
  year_month: string;
  cost: number;
};

type Props = {
  // Dashboard-only (#852): the selected month ("YYYY-MM") is marked on the
  // chart and clicking a point reports the clicked month. Other charts omit both.
  selectedMonth?: string;
  onSelectMonth?: (yearMonth: string) => void;
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
  // Optional secondary composition lines drawn beneath the primary area
  // (dashboard net-worth view, ADR-0001 Presentation). Detail screens omit it.
  extraSeries?: {
    key: string;
    label: string;
    color: string;
    snapshots: SnapshotLike[];
  }[];
  primaryLabel?: string;
  primaryColor?: string;
};

// Lazy boundary so recharts + the shadcn chart wrapper land in a
// separate chunk, fetched only when a detail page actually renders the
// chart. The empty-snapshot short-circuit stays out here so we don't
// even request the chunk on empty data.
const SnapshotChartImpl = lazyWithReload(() => import("./SnapshotChartImpl"));

export function SnapshotChart({
  selectedMonth,
  onSelectMonth,
  snapshots,
  currency,
  costSeries,
  status,
  extraSeries,
  primaryLabel,
  primaryColor,
}: Props) {
  const yms = snapshots.map((s) => s.year_month.slice(0, 7)).sort();
  const months = yms.length > 0 ? monthRange(yms[0], yms[yms.length - 1]) : [];
  const zoom = useRangeZoom(
    months.length,
    selectedMonth ? { key: selectedMonth, index: months.indexOf(selectedMonth) } : undefined,
  );
  if (snapshots.length === 0) return null;
  return (
    <div className="space-y-3">
      <RangeChips chips={zoom.chips} pressed={zoom.pressed} onPick={zoom.pickPreset} />
      <Suspense fallback={<div className="h-64 w-full" />}>
        <SnapshotChartImpl
          interaction={{
            window: zoom.window,
            showBrush: zoom.showBrush,
            onWindowChange: zoom.onWindowChange,
            selectedMonth,
            onSelectMonth,
          }}
          snapshots={snapshots}
          currency={currency}
          costSeries={costSeries}
          status={status}
          extraSeries={extraSeries}
          primaryLabel={primaryLabel}
          primaryColor={primaryColor}
        />
      </Suspense>
    </div>
  );
}
