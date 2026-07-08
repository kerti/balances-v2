import { Path, Svg, Text, View } from "@react-pdf/renderer";
import {
  computeDonutSlices,
  PIE_CHART_PALETTE,
  type PieChartSlice,
} from "@/lib/pdf/charts/pieChartMath";

const SIZE = 96;
const R_OUTER = 46;
const R_INNER = 26;

type Props = { data: PieChartSlice[] };

// The donut only — no legend, no text. Hand-rolled with react-pdf's own
// Svg/Path primitives (react-pdf can't render DOM/canvas charts), the same
// approach LineChart.tsx uses for the net-worth trend. Renders nothing when
// every value is zero/absent (an empty ring reads as broken, not empty).
export function PieChart({ data }: Props) {
  const visible = data.filter((d) => d.value > 0);
  const total = visible.reduce((sum, d) => sum + d.value, 0);
  if (total <= 0) return null;

  const slices = computeDonutSlices(visible, {
    cx: SIZE / 2,
    cy: SIZE / 2,
    rOuter: R_OUTER,
    rInner: R_INNER,
  });

  return (
    <Svg width={SIZE} height={SIZE} viewBox={`0 0 ${SIZE} ${SIZE}`}>
      {slices.map((s, i) => (
        <Path key={s.key} d={s.path} fill={PIE_CHART_PALETTE[i % PIE_CHART_PALETTE.length]} />
      ))}
    </Svg>
  );
}

// PieChartLegend is the swatch + label + percentage list beside the donut —
// split out so ReportDocument (which owns `t`) composes the label text,
// keeping this file translation-agnostic. `label` is already resolved by the
// caller; percentages are computed here so callers don't duplicate the total.
export function PieChartLegend({
  data,
  labelFor,
}: {
  data: PieChartSlice[];
  labelFor: (key: string, percent: number) => string;
}) {
  const visible = data.filter((d) => d.value > 0);
  const total = visible.reduce((sum, d) => sum + d.value, 0);
  if (total <= 0) return null;

  return (
    <View>
      {visible.map((d, i) => (
        <View key={d.key} style={{ flexDirection: "row", alignItems: "center", marginBottom: 2 }}>
          <View
            style={{
              width: 7,
              height: 7,
              marginRight: 4,
              backgroundColor: PIE_CHART_PALETTE[i % PIE_CHART_PALETTE.length],
            }}
          />
          <Text style={{ fontSize: 8, color: "#334155" }}>
            {labelFor(d.key, Math.round((d.value / total) * 100))}
          </Text>
        </View>
      ))}
    </View>
  );
}
