// 100%-stacked category share chart (issue #14 slice 14d). Lazy
// boundary so the recharts + shadcn-chart code lands in a separate
// chunk alongside the other home charts.

import { Suspense } from "react";
import { lazyWithReload } from "@/lib/lazyWithReload";
import { RangeChips } from "./RangeChips";
import { useRangeZoom } from "./useRangeZoom";
import type { CategoryTimePoint } from "@/lib/homeAggregates";

const CategoryStackChartImpl = lazyWithReload(() => import("./CategoryStackChartImpl"));

type Props = {
  series: CategoryTimePoint[];
  currency: string;
};

export function CategoryStackChart({ series, currency }: Props) {
  const zoom = useRangeZoom(series.length);
  if (series.length < 2) return null;
  return (
    <div className="space-y-3">
      <RangeChips chips={zoom.chips} pressed={zoom.pressed} onPick={zoom.pickPreset} />
      <Suspense fallback={<div className="h-64 w-full" />}>
        <CategoryStackChartImpl
          series={series}
          currency={currency}
          window={zoom.window}
          showBrush={zoom.showBrush}
          onWindowChange={zoom.onWindowChange}
        />
      </Suspense>
    </div>
  );
}
