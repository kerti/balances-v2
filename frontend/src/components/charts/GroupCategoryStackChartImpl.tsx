import { Area, AreaChart, Brush, CartesianGrid, YAxis } from "recharts";
import { MonthAxis } from "@/components/charts/MonthAxis";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { sliceWindow, type IndexWindow } from "@/lib/chartRange";
import { formatChartMonth, formatCurrency } from "@/lib/format";
import type { GroupCategoryTimePoint } from "@/lib/groupHomeAggregates";
import type { GroupStackCategory } from "./GroupCategoryStackChart";

type Props = {
  series: GroupCategoryTimePoint[];
  categories: GroupStackCategory[];
  currency: string;
  // Range zoom (#901): the visible window over the sorted months. With the
  // Brush the chart keeps every month and the Brush narrows the view; without
  // it (phones) the chart draws only the window.
  window: IndexWindow;
  showBrush: boolean;
  onWindowChange: (w: IndexWindow) => void;
};

type Row = { month: string } & Record<string, number | string>;

function toRows(series: GroupCategoryTimePoint[], categories: GroupStackCategory[]): Row[] {
  return [...series]
    .sort((a, b) => a.year_month.localeCompare(b.year_month))
    .map((p) => {
      const row: Row = { month: formatChartMonth(new Date(p.year_month)) };
      for (const c of categories) {
        row[c.key] = p.byCategory[c.key] ?? 0;
      }
      return row;
    });
}

export default function GroupCategoryStackChartImpl({
  series,
  categories,
  currency,
  window: win,
  showBrush,
  onWindowChange,
}: Props) {
  const data = toRows(series, categories);
  // Ticks follow the visible window: with the Brush, chartData is every month.
  const visible = sliceWindow(data, win);
  const brush = showBrush && data.length > 2;
  const chartData = brush ? data : visible;

  // Drop categories that are zero across every month — keeps the legend tight
  // for households that only hold one subtype.
  const present = categories.filter((c) => data.some((row) => Number(row[c.key]) > 0));

  const chartConfig: ChartConfig = Object.fromEntries(
    present.map((c) => [c.key, { label: c.label, color: c.color }]),
  );

  return (
    <ChartContainer config={chartConfig} className={brush ? "h-80 w-full" : "h-64 w-full"}>
      <AreaChart
        data={chartData}
        stackOffset="expand"
        margin={{ left: 0, right: 24, top: 12, bottom: 0 }}
      >
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        <MonthAxis months={visible.map((p) => p.month)} yAxisWidth={48} />
        <YAxis
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          fontSize={12}
          width={48}
          tickFormatter={(v: number) => `${Math.round(v * 100)}%`}
          domain={[0, 1]}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              labelFormatter={(label) => label}
              // Render a full row per category — colored indicator + label +
              // share% + native amount — so each band is identifiable, matching
              // the cost-vs-value chart. (ChartTooltipContent renders *only* the
              // formatter's output when one is set, so it must supply the
              // indicator/label itself.)
              formatter={(value, name, item) => {
                const key = String(name);
                const seriesLabel = (chartConfig as ChartConfig)[key]?.label;
                const total = present.reduce((s, c) => s + (Number(item.payload[c.key]) || 0), 0);
                const pct = total > 0 ? (Number(value) / total) * 100 : 0;
                return (
                  <div className="flex w-full items-center justify-between gap-3">
                    <span className="flex items-center gap-1.5 text-muted-foreground">
                      <span
                        className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                        style={{ backgroundColor: `var(--color-${key})` }}
                      />
                      {seriesLabel ?? key}
                    </span>
                    <span className="font-medium text-foreground">
                      {pct.toFixed(1)}%
                      <span className="ml-2 font-normal text-muted-foreground">
                        {formatCurrency(String(value), currency)}
                      </span>
                    </span>
                  </div>
                );
              }}
            />
          }
        />
        {present.map((c) => (
          <Area
            key={c.key}
            dataKey={c.key}
            type="monotone"
            stackId="categories"
            fill={`var(--color-${c.key})`}
            stroke={`var(--color-${c.key})`}
            fillOpacity={0.6}
            isAnimationActive={false}
          />
        ))}
        {brush && (
          <Brush
            dataKey="month"
            height={28}
            travellerWidth={10}
            stroke="var(--border)"
            fill="var(--card)"
            startIndex={Math.min(win.start, data.length - 1)}
            endIndex={Math.min(win.end, data.length - 1)}
            onChange={(r) => {
              if (r.startIndex === undefined || r.endIndex === undefined) return;
              if (r.startIndex === win.start && r.endIndex === win.end) return;
              onWindowChange({ start: r.startIndex, end: r.endIndex });
            }}
          />
        )}
        <ChartLegend content={<ChartLegendContent />} />
      </AreaChart>
    </ChartContainer>
  );
}
