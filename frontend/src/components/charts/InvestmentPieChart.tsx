// Shared pie chart for the Investments home (issue #14 slice 14d).
// Rendered twice on the page — once for category share, once for risk
// profile share. Lazy boundary so recharts code lands in a separate
// chunk.
//
// **Color choices.** Category slices use the same tokens as the
// CategoryStackChart so legends match across the page; risk slices use the
// one-hue brass ramp from lib/categoryColors.ts.

import { Suspense } from "react";
import { lazyWithReload } from "@/lib/lazyWithReload";

export type PieSlice = {
  key: string;
  label: string;
  value: number;
  color: string;
};

type Props = {
  slices: PieSlice[];
  currency: string;
  legendPosition?: "bottom" | "right" | "none";
  // Ordinal slices (risk low→high): keep the legend in slice order instead of
  // Recharts' default alphabetical sort.
  ordered?: boolean;
};

const InvestmentPieChartImpl = lazyWithReload(() => import("./InvestmentPieChartImpl"));

export function InvestmentPieChart({
  slices,
  currency,
  legendPosition = "bottom",
  ordered = false,
}: Props) {
  const total = slices.reduce((s, sl) => s + sl.value, 0);
  if (total <= 0) return null;
  // Fallback height tracks the rendered container so the lazy chunk swaps in
  // without a layout jump: taller with a right-hand legend, tighter with none.
  const fallbackHeight = legendPosition === "none" ? "h-52" : "h-64";
  return (
    <Suspense fallback={<div className={`${fallbackHeight} w-full`} />}>
      <InvestmentPieChartImpl
        slices={slices}
        currency={currency}
        legendPosition={legendPosition}
        ordered={ordered}
      />
    </Suspense>
  );
}
