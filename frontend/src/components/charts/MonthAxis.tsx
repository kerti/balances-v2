import { XAxis, useChartWidth } from "recharts";
import { pickTickIndices } from "@/lib/chartTicks";

// The shared month x-axis: first data month always labelled, later labels
// thinned to the plot width (see lib/chartTicks.ts). Must render inside a chart.
export function MonthAxis({ months, yAxisWidth }: { months: string[]; yAxisWidth: number }) {
  const chartWidth = useChartWidth() ?? 0;
  // The plot sits between the y-axis and the charts' 24px right margin, which
  // leaves room for a label centred on the last month.
  const plotWidth = Math.max(0, chartWidth - yAxisWidth - 24);
  const ticks = pickTickIndices(months.length, plotWidth).map((i) => months[i]);
  return (
    <XAxis
      dataKey="month"
      ticks={ticks}
      interval={0}
      tickLine={false}
      axisLine={false}
      tickMargin={8}
      fontSize={12}
    />
  );
}
