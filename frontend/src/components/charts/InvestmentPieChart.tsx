// Shared pie chart for the Investments home (issue #14 slice 14d).
// Rendered twice on the page — once for category share, once for risk
// profile share. Lazy boundary so recharts code lands in a separate
// chunk.
//
// **Color choices.** Category slices use the same Tailwind 500-level
// palette as the CategoryStackChart so legends match across the page.
// Risk slices use a semantic traffic-light gradient (emerald / amber /
// red) — matches the existing P/L tone language (emerald gain,
// destructive loss) and reads at a glance.

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
};

const InvestmentPieChartImpl = lazyWithReload(() => import("./InvestmentPieChartImpl"));

export function InvestmentPieChart({ slices, currency, legendPosition = "bottom" }: Props) {
  const total = slices.reduce((s, sl) => s + sl.value, 0);
  if (total <= 0) return null;
  // Fallback height tracks the rendered container so the lazy chunk swaps in
  // without a layout jump: taller with a right-hand legend, tighter with none.
  const fallbackHeight = legendPosition === "none" ? "h-52" : "h-64";
  return (
    <Suspense fallback={<div className={`${fallbackHeight} w-full`} />}>
      <InvestmentPieChartImpl slices={slices} currency={currency} legendPosition={legendPosition} />
    </Suspense>
  );
}
