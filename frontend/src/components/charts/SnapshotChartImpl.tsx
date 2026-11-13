import { useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { MonthAxis } from "@/components/charts/MonthAxis";
import {
  Area,
  AreaChart,
  Brush,
  CartesianGrid,
  Line,
  ReferenceDot,
  ReferenceLine,
  YAxis,
  usePlotArea,
} from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { seriesOrder, seriesRank, sortBySeriesOrder } from "@/lib/chartSeries";
import { fitYDomain } from "@/lib/chartDomain";
import { sliceWindow, type IndexWindow } from "@/lib/chartRange";
import { compactTickFormatter, formatChartMonth, formatCurrency } from "@/lib/format";
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

// A secondary composition line drawn beneath the primary area (dashboard
// net-worth view, ADR-0001 Presentation). Each carries its own snapshot series
// over the same month domain as `snapshots`; `key` is the data/config key,
// `color` a CSS var. Detail screens pass none and are visually unchanged.
type ExtraSeries = {
  key: string;
  label: string;
  color: string;
  snapshots: SnapshotLike[];
};

// Range zoom (#852, #901): the visible window over the continuous month list
// and (desktop) a Brush strip. The selected-month marker and click-to-select
// are dashboard-only: they exist only when both selection fields are passed.
export type ChartInteraction = {
  window: IndexWindow;
  showBrush: boolean;
  onWindowChange: (w: IndexWindow) => void;
  // Selected month as bare "YYYY-MM".
  selectedMonth?: string;
  onSelectMonth?: (yearMonth: string) => void;
};

type Props = {
  interaction?: ChartInteraction;
  snapshots: SnapshotLike[];
  currency: string;
  costSeries?: CostPoint[];
  status?: string | null;
  extraSeries?: ExtraSeries[];
  // Legend label for the primary area. Defaults to the generic "Amount"
  // (detail screens); the dashboard passes "Net worth" so the legend reads
  // clearly beside the composition lines.
  primaryLabel?: string;
  // Colour of the primary area. Defaults to --chart-1 (detail screens); the
  // dashboard passes the brand brass, so the headline aggregate carries the
  // accent and its components read as context beneath it (ADR-0054).
  primaryColor?: string;
};

type ChartPoint = { ym: string; month: string; amount: number; cost?: number } & Record<
  string,
  unknown
>;

function toChartData(
  snapshots: SnapshotLike[],
  costSeries?: CostPoint[],
  extraSeries?: ExtraSeries[],
) {
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
  // Each extra series gets its own month→value lookup + carry-forward cursor,
  // keyed by the series key so points carry one field per line.
  const extras = (extraSeries ?? []).map((s) => {
    const byMonth = new Map<string, number>();
    for (const p of s.snapshots) byMonth.set(p.year_month.slice(0, 7), Number(p.amount));
    return { key: s.key, byMonth, last: 0 };
  });

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
    const point: ChartPoint = {
      ym,
      month: formatChartMonth(new Date(y, m - 1, 1)),
      amount: lastAmount,
    };
    if (hasCost && lastCost !== undefined) point.cost = lastCost;
    for (const e of extras) {
      if (e.byMonth.has(ym)) e.last = e.byMonth.get(ym)!;
      point[e.key] = e.last;
    }
    return point;
  });
}

export default function SnapshotChartImpl({
  interaction,
  snapshots,
  currency,
  costSeries,
  status,
  extraSeries,
  primaryLabel,
  primaryColor,
}: Props) {
  const { t } = useTranslation("dashboard");
  // Recharts resolves no active point on a tap, so click falls back to the
  // tap's x within the plot area.
  const plotArea = useRef<PlotArea | undefined>(undefined);
  const setPlotArea = useCallback((a: PlotArea | undefined) => {
    plotArea.current = a;
  }, []);
  const data = toChartData(snapshots, costSeries, extraSeries);

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
          label: status === "matured" ? t("chart.maturedMarker") : t("chart.soldMarker"),
        }
      : null;

  const hasCost = (costSeries ?? []).length > 0;
  const extras = extraSeries ?? [];
  const hasExtras = extras.length > 0;
  // ChartConfig is built per-render so the legend label picks up the active
  // locale. Cheap — single key, no per-row computation.
  const chartConfig = {
    amount: {
      label: primaryLabel ?? t("chart.amountLegend"),
      color: primaryColor ?? "var(--chart-1)",
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
    // Composition lines (dashboard net-worth view) carry their own label +
    // color, so `--color-<key>` resolves for both the line and its legend/
    // tooltip swatch.
    ...Object.fromEntries(extras.map((s) => [s.key, { label: s.label, color: s.color }])),
  } satisfies ChartConfig;

  // One order for the legend and the tooltip (INC-34); see lib/chartSeries.
  const order = seriesOrder(
    extras.map((s) => s.key),
    hasCost,
  );

  // The y-axis fits what is on screen: the zoom window when there is one, else
  // everything. Series values only — `ym` / `month` are labels.
  const win = interaction?.window;
  const visible = sliceWindow(data, win);
  const fit = fitYDomain(
    visible.flatMap((p) =>
      Object.entries(p)
        .filter(([k]) => k !== "ym" && k !== "month")
        .map(([, v]) => Number(v)),
    ),
  );
  const onSelectMonth = interaction?.onSelectMonth;
  const selectedLabel = interaction?.selectedMonth
    ? data.find((p) => p.ym === interaction.selectedMonth)?.month
    : undefined;
  const showBrush = !!interaction?.showBrush && data.length > 2;
  // With the Brush the chart keeps every month and the Brush narrows the view;
  // without it (phones) the chart itself draws only the window.
  const chartData = showBrush ? data : visible;

  return (
    <ChartContainer config={chartConfig} className={showBrush ? "h-80 w-full" : "h-64 w-full"}>
      <AreaChart
        data={chartData}
        className={onSelectMonth ? "cursor-pointer" : undefined}
        onClick={
          onSelectMonth
            ? (state, event) => {
                let label = state?.activeLabel;
                const area = plotArea.current;
                const svg = (event?.target as Element | null)?.closest("svg");
                if (label == null && area && svg && chartData.length > 0) {
                  const x = event.clientX - svg.getBoundingClientRect().left;
                  const f = Math.min(1, Math.max(0, (x - area.x) / area.width));
                  label = chartData[Math.round(f * (chartData.length - 1))].month;
                }
                const hit = chartData.find((p) => p.month === label);
                if (hit) onSelectMonth(hit.ym);
              }
            : undefined
        }
        // Extra top headroom when a maturity/sold marker is drawn — its
        // label sits above the dot, which often lands at the line's peak
        // (plot top), so the default 12px margin clips the text vertically.
        margin={{ left: 0, right: 24, top: marker ? 28 : 12, bottom: 0 }}
      >
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        {/* Ticks follow the visible window: with the Brush, chartData is every month. */}
        <MonthAxis months={visible.map((p) => p.month)} yAxisWidth={80} />
        <YAxis
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          fontSize={12}
          width={80}
          domain={fit.domain}
          ticks={fit.ticks}
          interval={0}
          tickFormatter={compactTickFormatter(fit.ticks)}
        />
        {fit.crossesZero && (
          <ReferenceLine y={0} stroke="var(--muted-foreground)" strokeOpacity={0.6} />
        )}
        {selectedLabel && (
          <ReferenceLine
            x={selectedLabel}
            stroke="var(--foreground)"
            strokeOpacity={0.55}
            strokeDasharray="4 3"
            data-testid="chart-selected-month-marker"
          />
        )}
        <ChartTooltip
          content={(props) => (
            <ChartTooltipContent
              active={props.active}
              label={props.label}
              payload={sortBySeriesOrder(props.payload ?? [], order, (p) => p.dataKey)}
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
                    <span className="font-medium text-foreground">
                      {formatCurrency(String(value), currency)}
                    </span>
                  </div>
                );
              }}
              labelFormatter={(label) => label}
            />
          )}
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
        {extras.map((s) => (
          // Composition lines sit beneath the net-worth area — thin, unfilled,
          // dotless — so the headline area reads first; they are context, not
          // co-equal series (ADR-0001 Presentation).
          <Line
            key={s.key}
            dataKey={s.key}
            type="monotone"
            stroke={`var(--color-${s.key})`}
            strokeWidth={1.5}
            dot={false}
            activeDot={{ r: 3 }}
            isAnimationActive={false}
          />
        ))}
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
        {onSelectMonth && <PlotAreaProbe onChange={setPlotArea} />}
        {showBrush && interaction && win && (
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
              interaction.onWindowChange({ start: r.startIndex, end: r.endIndex });
            }}
          />
        )}
        {(hasCost || hasExtras) && (
          <ChartLegend
            itemSorter={(item) => seriesRank(order, item.dataKey)}
            content={<ChartLegendContent />}
          />
        )}
      </AreaChart>
    </ChartContainer>
  );
}

type PlotArea = NonNullable<ReturnType<typeof usePlotArea>>;

function PlotAreaProbe({ onChange }: { onChange: (area: PlotArea | undefined) => void }) {
  const area = usePlotArea();
  useEffect(() => onChange(area), [area, onChange]);
  return null;
}
