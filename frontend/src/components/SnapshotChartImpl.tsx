import { useTranslation } from "react-i18next";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  ReferenceDot,
  XAxis,
  YAxis,
} from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  formatChartMonth,
  formatCompactNumber,
  formatCurrency,
} from "@/lib/format";
import { monthRange } from "@/lib/months";

// Generic snapshot shape — all four position groups (asset, liability,
// receivable, investment) have amount-shaped snapshots with year_month +
// amount, so the chart only needs these two fields.
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
  costSeries?: CostPoint[];
  status?: string | null;
};

function toChartData(snapshots: SnapshotLike[], costSeries?: CostPoint[]) {
  // Lookups by year_month prefix — caller passes either the bare "YYYY-MM"
  // or the API's "YYYY-MM-DDT..." shape, both reduce to the same key via
  // slice(0, 7).
  const amountByMonth = new Map<string, number>();
  for (const s of snapshots) {
    amountByMonth.set(s.year_month.slice(0, 7), Number(s.amount));
  }
  const costByMonth = new Map<string, number>();
  for (const c of costSeries ?? []) {
    costByMonth.set(c.year_month.slice(0, 7), c.cost);
  }

  const months = [...amountByMonth.keys()].sort();
  if (months.length === 0) return [];

  // Walk the continuous month range, not just months with a snapshot, so
  // the categorical X axis renders a proportional timeline (#24). Gap
  // months carry the last known value (and cost) forward — a balance you
  // didn't re-snapshot still held its value, it didn't drop to zero.
  const hasCost = (costSeries ?? []).length > 0;
  let lastAmount = 0;
  let lastCost: number | undefined;
  return monthRange(months[0], months[months.length - 1]).map((ym) => {
    if (amountByMonth.has(ym)) lastAmount = amountByMonth.get(ym)!;
    if (costByMonth.has(ym)) lastCost = costByMonth.get(ym);
    const [y, m] = ym.split("-").map(Number);
    const point: { month: string; amount: number; cost?: number } = {
      month: formatChartMonth(new Date(y, m - 1, 1)),
      amount: lastAmount,
    };
    if (hasCost && lastCost !== undefined) point.cost = lastCost;
    return point;
  });
}

export default function SnapshotChartImpl({
  snapshots,
  currency,
  costSeries,
  status,
}: Props) {
  const { t } = useTranslation("dashboard");
  const data = toChartData(snapshots, costSeries);

  // A terminated position carries a truthful 0-value close snapshot at its
  // termination month (#25). Drawn as-is the value line craters to 0, which
  // reads as "the position lost all its value" rather than "the position
  // closed and the cash moved to the bank." Drop that trailing 0 point so
  // the line ends at the last real value, and mark that point Sold/Matured.
  const isClosed = status === "sold" || status === "matured";
  if (isClosed && data.length > 0 && data[data.length - 1].amount === 0) {
    data.pop();
  }
  const marker =
    isClosed && data.length > 0
      ? {
          month: data[data.length - 1].month,
          amount: data[data.length - 1].amount,
          label:
            status === "matured"
              ? t("chart.maturedMarker")
              : t("chart.soldMarker"),
        }
      : null;

  const hasCost = (costSeries ?? []).length > 0;
  // ChartConfig is built per-render so the legend label picks up the active
  // locale. Cheap — single key, no per-row computation.
  const chartConfig = {
    amount: {
      label: t("chart.amountLegend"),
      color: "var(--chart-1)",
    },
    ...(hasCost && {
      cost: {
        label: t("chart.costLegend"),
        // Muted-slate baseline (issue #14 decision): cost is a reference
        // line; gain / loss reads from the gap between value and cost,
        // not from the cost line's own color cue.
        color: "var(--muted-foreground)",
      },
    }),
  } satisfies ChartConfig;

  return (
    <ChartContainer config={chartConfig} className="h-64 w-full">
      <AreaChart
        data={data}
        // Extra top headroom when a maturity/sold marker is drawn — its
        // label sits above the dot, which often lands at the line's peak
        // (plot top), so the default 12px margin clips the text vertically.
        margin={{ left: 0, right: 12, top: marker ? 28 : 12, bottom: 0 }}
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
          width={80}
          tickFormatter={(v: number) => formatCompactNumber(v)}
        />
        <ChartTooltip
          content={
            <ChartTooltipContent
              // Render a full row per series — colored indicator + series
              // label + formatted value — so amount vs cost is legible.
              // (ChartTooltipContent renders *only* the formatter's output
              // when one is set, dropping its own label/indicator, so the
              // formatter must supply them itself.)
              formatter={(value, name) => {
                const key = String(name);
                const seriesLabel = (chartConfig as ChartConfig)[key]?.label;
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
                      {formatCurrency(String(value), currency)}
                    </span>
                  </div>
                );
              }}
              labelFormatter={(label) => label}
            />
          }
        />
        <Area
          dataKey="amount"
          type="monotone"
          fill="var(--color-amount)"
          fillOpacity={0.2}
          stroke="var(--color-amount)"
          strokeWidth={2}
        />
        {hasCost && (
          <Line
            dataKey="cost"
            type="monotone"
            stroke="var(--color-cost)"
            strokeWidth={1.5}
            dot={false}
            activeDot={false}
            isAnimationActive={false}
          />
        )}
        {marker && (
          <ReferenceDot
            x={marker.month}
            y={marker.amount}
            r={4}
            fill="var(--color-amount)"
            stroke="var(--background)"
            strokeWidth={1.5}
            // The dot sits at the rightmost data point, so a default
            // (middle-anchored) top label is half-clipped by the chart's
            // right edge. `textAnchor: 'end'` anchors the text at the dot and
            // extends it leftward, back into the plot, so it stays readable.
            label={{
              value: marker.label,
              position: "top",
              textAnchor: "end",
              fontSize: 11,
              fontWeight: 500,
              fill: "var(--muted-foreground)",
            }}
          />
        )}
        {hasCost && <ChartLegend content={<ChartLegendContent />} />}
      </AreaChart>
    </ChartContainer>
  );
}
