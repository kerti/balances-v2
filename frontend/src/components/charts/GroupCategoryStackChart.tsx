// Generic 100%-stacked category-share chart (epic #204) — the group-Home
// twin of CategoryStackChart, which is hardwired to the 5 investment
// categories. This one takes its categories (key + label + color) as a prop,
// so AssetsHome (bankAccount/property/vehicle) and LiabilitiesHome
// (personal/institutional) reuse one impl. Lazy boundary so recharts lands in
// a separate chunk, matching the investment chart.

import { lazy, Suspense } from "react";
import { RangeChips } from "./RangeChips";
import { useRangeZoom } from "./useRangeZoom";
import type { GroupCategoryTimePoint } from "@/lib/groupHomeAggregates";

const GroupCategoryStackChartImpl = lazy(() => import("./GroupCategoryStackChartImpl"));

export type GroupStackCategory = {
  key: string;
  label: string;
  color: string;
};

type Props = {
  series: GroupCategoryTimePoint[];
  categories: GroupStackCategory[];
  currency: string;
};

export function GroupCategoryStackChart({ series, categories, currency }: Props) {
  const zoom = useRangeZoom(series.length);
  if (series.length < 2) return null;
  return (
    <div className="space-y-3">
      <RangeChips chips={zoom.chips} pressed={zoom.pressed} onPick={zoom.pickPreset} />
      <Suspense fallback={<div className="h-64 w-full" />}>
        <GroupCategoryStackChartImpl
          series={series}
          categories={categories}
          currency={currency}
          window={zoom.window}
          showBrush={zoom.showBrush}
          onWindowChange={zoom.onWindowChange}
        />
      </Suspense>
    </div>
  );
}
