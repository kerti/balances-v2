import { Circle, G, Line, Path, Svg, Text, View } from "@react-pdf/renderer";
import { formatChartMonth, formatCompactNumber } from "@/lib/format";
import { computeLineChartGeometry, monthToDate } from "@/lib/pdf/charts/lineChartMath";

const WIDTH = 460;
const HEIGHT = 116;
const MARGIN = { left: 40, right: 4, top: 8, bottom: 14 };
const PLOT_WIDTH = WIDTH - MARGIN.left - MARGIN.right;
const PLOT_HEIGHT = HEIGHT - MARGIN.top - MARGIN.bottom;
const AXIS_COLOR = "#CBD5E1";
const LINE_COLOR = "#6366F1";

type Props = {
  series: { year_month: string; amount: string }[];
};

// Redraws the net-worth time series as a native vector path (ADR-0044) rather
// than rasterizing the on-screen recharts SVG — print-crisp, theme-independent.
// Unlike the on-screen SnapshotChartImpl (recharts, tick per month), this
// hand-rolled axis only labels the two value extremes and the two endpoint
// months — the print layout has no room for a dense tick grid.
export function LineChart({ series }: Props) {
  const { path, points, months, minY, maxY } = computeLineChartGeometry(series, {
    width: PLOT_WIDTH,
    height: PLOT_HEIGHT,
  });
  if (!path) return null;

  return (
    <View style={{ width: WIDTH, height: HEIGHT }}>
      <Svg width={WIDTH} height={HEIGHT} viewBox={`0 0 ${WIDTH} ${HEIGHT}`}>
        <G transform={`translate(${MARGIN.left}, ${MARGIN.top})`}>
          <Line x1={0} y1={0} x2={0} y2={PLOT_HEIGHT} stroke={AXIS_COLOR} strokeWidth={1} />
          <Line
            x1={0}
            y1={PLOT_HEIGHT}
            x2={PLOT_WIDTH}
            y2={PLOT_HEIGHT}
            stroke={AXIS_COLOR}
            strokeWidth={1}
          />
          <Path d={path} stroke={LINE_COLOR} strokeWidth={2} fill="none" />
          {points.map((p, i) => (
            <Circle key={i} cx={p.x} cy={p.y} r={2} fill={LINE_COLOR} />
          ))}
        </G>
      </Svg>
      <Text
        style={{
          position: "absolute",
          left: 0,
          top: MARGIN.top - 4,
          fontSize: 7,
          color: "#64748B",
        }}
      >
        {formatCompactNumber(maxY)}
      </Text>
      <Text
        style={{
          position: "absolute",
          left: 0,
          top: MARGIN.top + PLOT_HEIGHT - 4,
          fontSize: 7,
          color: "#64748B",
        }}
      >
        {formatCompactNumber(minY)}
      </Text>
      <Text
        style={{
          position: "absolute",
          left: MARGIN.left,
          top: HEIGHT - 9,
          fontSize: 7,
          color: "#64748B",
        }}
      >
        {formatChartMonth(monthToDate(months[0]))}
      </Text>
      <Text
        style={{
          position: "absolute",
          right: MARGIN.right,
          top: HEIGHT - 9,
          fontSize: 7,
          color: "#64748B",
        }}
      >
        {formatChartMonth(monthToDate(months[months.length - 1]))}
      </Text>
    </View>
  );
}
