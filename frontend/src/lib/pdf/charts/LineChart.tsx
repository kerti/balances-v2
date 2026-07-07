import { Path, Svg } from "@react-pdf/renderer";
import { computeLineChartGeometry } from "@/lib/pdf/charts/lineChartMath";

const WIDTH = 460;
const HEIGHT = 120;

type Props = {
  series: { year_month: string; amount: string }[];
};

// Redraws the net-worth time series as a native vector path (ADR-0044) rather
// than rasterizing the on-screen recharts SVG — print-crisp, theme-independent.
export function LineChart({ series }: Props) {
  const { path } = computeLineChartGeometry(series, { width: WIDTH, height: HEIGHT });
  if (!path) return null;
  return (
    <Svg width={WIDTH} height={HEIGHT} viewBox={`0 0 ${WIDTH} ${HEIGHT}`}>
      <Path d={path} stroke="#6366F1" strokeWidth={2} fill="none" />
    </Svg>
  );
}
