import { Cell, Pie, PieChart } from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { formatCurrency } from "@/lib/format";
import type { PieSlice } from "./InvestmentPieChart";

type Props = {
  slices: PieSlice[];
  currency: string;
  legendPosition?: "bottom" | "right" | "none";
};

export default function InvestmentPieChartImpl({
  slices,
  currency,
  legendPosition = "bottom",
}: Props) {
  // Drop empty slices so the pie's `nameKey`-driven legend stays tight.
  const present = slices.filter((s) => s.value > 0);
  const total = present.reduce((s, sl) => s + sl.value, 0);

  const chartConfig: ChartConfig = Object.fromEntries(
    present.map((s) => [s.key, { label: s.label, color: s.color }]),
  );

  // Recharts' `Pie` reads `dataKey` for the wedge magnitude and
  // `nameKey` for the legend identity; we shape rows accordingly.
  const data = present.map((s) => ({
    key: s.key,
    label: s.label,
    value: s.value,
    fill: s.color,
  }));

  const isRight = legendPosition === "right";
  const isNone = legendPosition === "none";

  // Container height is kept close to the donut so there's little dead vertical
  // space: `h-64` around the 220px right-legend donut (the legend sits beside
  // it, vertically centered) and the 176px bottom-legend donut; `h-52` when the
  // legend is dropped entirely (Tags mobile — the cards below double as it).
  const containerHeight = isRight
    ? "h-64 w-full max-w-sm mx-auto"
    : isNone
      ? "h-52 w-full"
      : "h-64 w-full";

  return (
    <ChartContainer config={chartConfig} className={containerHeight}>
      <PieChart margin={isNone ? { top: 0, right: 0, bottom: 0, left: 0 } : undefined}>
        <ChartTooltip
          content={
            <ChartTooltipContent
              nameKey="key"
              hideLabel
              formatter={(value, _name, item) => {
                const pct = total > 0 ? (Number(value) / total) * 100 : 0;
                const amount = formatCurrency(String(value), currency);
                const label = item.payload.label as string;
                return `${label}: ${amount} (${pct.toFixed(1)}%)`;
              }}
            />
          }
        />
        <Pie
          data={data}
          dataKey="value"
          nameKey="key"
          innerRadius={isRight ? 60 : 48}
          outerRadius={isRight ? 110 : 88}
          paddingAngle={1}
          isAnimationActive={false}
        >
          {data.map((d) => (
            <Cell key={d.key} fill={d.fill} stroke="var(--background)" />
          ))}
        </Pie>
        {isNone ? null : isRight ? (
          <ChartLegend
            layout="vertical"
            verticalAlign="middle"
            align="right"
            content={<ChartLegendContent nameKey="key" className="flex-col items-start" />}
          />
        ) : (
          <ChartLegend content={<ChartLegendContent nameKey="key" />} />
        )}
      </PieChart>
    </ChartContainer>
  );
}
