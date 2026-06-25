import { useTranslation } from "react-i18next";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { formatChartMonth, formatCurrency } from "@/lib/format";
import {
  INVESTMENT_CATEGORIES,
  type CategoryTimePoint,
  type InvestmentCategory,
} from "@/lib/homeAggregates";

// Per-category fill. The shadcn theme ships --chart-1..5 but all five
// are cyan-family, so a stacked / pie split would read as one blob.
// Use distinct hues from the Tailwind 500-level shades instead, sourced
// from existing app usage where possible (emerald = gain tone, amber =
// medium-risk tone, violet = neutral accent). Documented here so the
// choice is auditable.
const CATEGORY_FILLS: Record<InvestmentCategory, string> = {
  stock: "#06b6d4", // cyan-500
  mutualFund: "#8b5cf6", // violet-500
  bond: "#3b82f6", // blue-500
  timeDeposit: "#10b981", // emerald-500
  gold: "#eab308", // yellow-500 (literal gold connotation)
};

type Props = {
  series: CategoryTimePoint[];
  currency: string;
};

type Row = {
  month: string;
} & Record<InvestmentCategory, number>;

function toRows(series: CategoryTimePoint[]): Row[] {
  return [...series]
    .sort((a, b) => a.year_month.localeCompare(b.year_month))
    .map((p) => {
      const row: Row = {
        month: formatChartMonth(new Date(p.year_month)),
        stock: p.byCategory.stock,
        mutualFund: p.byCategory.mutualFund,
        bond: p.byCategory.bond,
        timeDeposit: p.byCategory.timeDeposit,
        gold: p.byCategory.gold,
      };
      return row;
    });
}

export default function CategoryStackChartImpl({ series, currency }: Props) {
  const { t } = useTranslation("investments");
  const data = toRows(series);

  // Drop categories that are zero across every month — keeps the legend
  // tight for households that only hold a few subtypes.
  const presentCategories = INVESTMENT_CATEGORIES.filter((c) =>
    data.some((row) => row[c] > 0),
  );

  const chartConfig: ChartConfig = Object.fromEntries(
    presentCategories.map((c) => [
      c,
      {
        label: t(`home.categoryLabel.${c}`),
        color: CATEGORY_FILLS[c],
      },
    ]),
  );

  return (
    <ChartContainer config={chartConfig} className="h-64 w-full">
      <AreaChart
        data={data}
        stackOffset="expand"
        margin={{ left: 0, right: 12, top: 12, bottom: 0 }}
      >
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        <XAxis
          dataKey="month"
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          fontSize={12}
        />
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
                const total = presentCategories.reduce(
                  (s, c) => s + (item.payload[c] ?? 0),
                  0,
                );
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
                    <span className="font-mono font-medium tabular-nums text-foreground">
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
        {presentCategories.map((c) => (
          <Area
            key={c}
            dataKey={c}
            type="monotone"
            stackId="categories"
            fill={`var(--color-${c})`}
            stroke={`var(--color-${c})`}
            fillOpacity={0.6}
            isAnimationActive={false}
          />
        ))}
        <ChartLegend content={<ChartLegendContent />} />
      </AreaChart>
    </ChartContainer>
  );
}
