import { Circle, G, Line, Path, Svg, Text, View } from "@react-pdf/renderer";
import { formatChartMonth, formatCompactNumber } from "@/lib/format";
import {
  computeTwoLineGeometry,
  monthToDate,
  TREND_INVESTMENT_RETURN_COLOR,
  TREND_LIVING_EXPENSES_COLOR,
} from "@/lib/pdf/charts/lineChartMath";
import type { TrendPoint } from "@/lib/pdf/reportPdfData";

const WIDTH = 460;
const HEIGHT = 116;
const MARGIN = { left: 40, right: 4, top: 8, bottom: 14 };
const PLOT_WIDTH = WIDTH - MARGIN.left - MARGIN.right;
const PLOT_HEIGHT = HEIGHT - MARGIN.top - MARGIN.bottom;
const AXIS_COLOR = "#CBD5E1";

type Props = { trend: TrendPoint[] };

// The 12-month expense-vs-passive-income trend (ADR-0045). Two pre-aligned,
// contiguous series on one shared scale — see computeTwoLineGeometry for why
// this doesn't need LineChart's gap-filling. Axis/marker treatment mirrors
// LineChart.tsx; the series legend itself is drawn by the caller
// (ReportDocument), which already owns the swatch + translated label markup.
export function TrendChart({ trend }: Props) {
  const { pathA, pathB, pointsA, pointsB, minY, maxY } = computeTwoLineGeometry(
    trend.map((p) => p.livingExpenses),
    trend.map((p) => p.investmentReturn),
    { width: PLOT_WIDTH, height: PLOT_HEIGHT },
  );
  if (!pathA || !pathB) return null;

  const months = trend.map((p) => p.year_month.slice(0, 7));

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
          <Path d={pathA} stroke={TREND_LIVING_EXPENSES_COLOR} strokeWidth={2} fill="none" />
          <Path d={pathB} stroke={TREND_INVESTMENT_RETURN_COLOR} strokeWidth={2} fill="none" />
          {pointsA.map((p, i) => (
            <Circle key={`a-${i}`} cx={p.x} cy={p.y} r={1.75} fill={TREND_LIVING_EXPENSES_COLOR} />
          ))}
          {pointsB.map((p, i) => (
            <Circle
              key={`b-${i}`}
              cx={p.x}
              cy={p.y}
              r={1.75}
              fill={TREND_INVESTMENT_RETURN_COLOR}
            />
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
