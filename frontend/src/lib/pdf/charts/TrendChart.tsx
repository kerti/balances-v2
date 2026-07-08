import { Path, Svg } from "@react-pdf/renderer";
import {
  computeTwoLineGeometry,
  TREND_INVESTMENT_RETURN_COLOR,
  TREND_LIVING_EXPENSES_COLOR,
} from "@/lib/pdf/charts/lineChartMath";

const WIDTH = 460;
const HEIGHT = 100;

type Props = { livingExpenses: number[]; investmentReturn: number[] };

// The 12-month expense-vs-passive-income trend (ADR-0045). Two pre-aligned,
// contiguous series on one shared scale — see computeTwoLineGeometry for why
// this doesn't need LineChart's gap-filling.
export function TrendChart({ livingExpenses, investmentReturn }: Props) {
  const { pathA, pathB } = computeTwoLineGeometry(livingExpenses, investmentReturn, {
    width: WIDTH,
    height: HEIGHT,
  });
  if (!pathA || !pathB) return null;
  return (
    <Svg width={WIDTH} height={HEIGHT} viewBox={`0 0 ${WIDTH} ${HEIGHT}`}>
      <Path d={pathA} stroke={TREND_LIVING_EXPENSES_COLOR} strokeWidth={2} fill="none" />
      <Path d={pathB} stroke={TREND_INVESTMENT_RETURN_COLOR} strokeWidth={2} fill="none" />
    </Svg>
  );
}
